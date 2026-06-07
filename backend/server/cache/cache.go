package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"skat/game"
	"skat/server/db"
)

type GameCache interface {
	GetGameByID(gameID string) (*game.GameState, error)
	GetGameBySessionID(sessionID string) (*game.GameState, error)
	GetGameBySessionCode(sessionCode string) (*game.GameState, error)
	SaveGame(gs game.GameState) error
}

type SyncQueue interface {
	EnqueueGameSave(ctx context.Context, gs game.GameState) error
	DequeueGameSave(ctx context.Context) (*game.GameState, error)
	Close() error
}

type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Close() error
}

type RevisionStore interface {
	NextRevision(ctx context.Context, gameID string, ttl time.Duration) (int64, error)
}

type ClientPresenceStore interface {
	MarkOnline(ctx context.Context, profileID, nodeID string, ttl time.Duration) error
	MarkOffline(ctx context.Context, profileID, nodeID string) error
	IsOnline(ctx context.Context, profileID string) (bool, error)
	OnlineIDs(ctx context.Context) ([]string, error)
}

type ClientMessageBus interface {
	PublishClientMessage(ctx context.Context, payload []byte) error
	SubscribeClientMessages(ctx context.Context) (<-chan []byte, error)
}

var ErrQueueEmpty = errors.New("cache sync queue empty")

var ErrMiss = errors.New("distributed cache miss")

type DistributedCache struct {
	store Store
	queue SyncQueue
	db    db.Database
	ttl   time.Duration
}

func NewDistributedCache(database db.Database, store Store, queue SyncQueue, ttl time.Duration) *DistributedCache {
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	return &DistributedCache{
		store: store,
		queue: queue,
		db:    database,
		ttl:   ttl,
	}
}

func (c *DistributedCache) GetGameByID(gameID string) (*game.GameState, error) {
	if gs, err := c.getGame("game:" + gameID); err == nil {
		if !hasInvalidMissingDeclarer(gs) {
			return gs, nil
		}
	}

	gs, err := c.db.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	_ = c.writeGameToCache(*gs)
	return gs, nil
}

func hasInvalidMissingDeclarer(gs *game.GameState) bool {
	if gs == nil || gs.Mode == game.ModeRamsch || gs.Declarer != nil {
		return false
	}
	return gs.Phase == game.PhaseSkatExchange ||
		gs.Phase == game.PhaseDeclarerChoice ||
		gs.Phase == game.PhasePlaying
}

func (c *DistributedCache) GetGameBySessionID(sessionID string) (*game.GameState, error) {
	gameID, err := c.getString("session:" + sessionID + ":latest")
	if err == nil && gameID != "" {
		return c.GetGameByID(gameID)
	}

	session, err := c.db.GetGameSession(sessionID)
	if err != nil {
		return nil, err
	}
	return c.GetGameByID(session.GameID)
}

func (c *DistributedCache) GetGameBySessionCode(sessionCode string) (*game.GameState, error) {
	gameID, err := c.getString("code:" + sessionCode + ":latest")
	if err == nil && gameID != "" {
		return c.GetGameByID(gameID)
	}

	gs, err := c.db.GetGameBySessionCode(sessionCode)
	if err != nil {
		return nil, err
	}
	_ = c.writeGameToCache(*gs)
	return gs, nil
}

func (c *DistributedCache) SaveGame(gs game.GameState) error {
	snapshot := *gs.Clone()
	revision, err := c.nextRevision(snapshot.ID)
	if err != nil {
		return c.db.SaveGame(snapshot)
	}
	snapshot.CacheRevision = revision

	if err := c.writeGameToCache(snapshot); err != nil {
		return c.db.SaveGame(snapshot)
	}
	if c.queue == nil {
		return c.db.SaveGame(snapshot)
	}
	if err := c.queue.EnqueueGameSave(context.Background(), snapshot); err != nil {
		return c.db.SaveGame(snapshot)
	}
	return nil
}

func (c *DistributedCache) nextRevision(gameID string) (int64, error) {
	revisionStore, ok := c.store.(RevisionStore)
	if !ok {
		return time.Now().UnixNano(), nil
	}
	return revisionStore.NextRevision(context.Background(), gameID, c.ttl)
}

func (c *DistributedCache) writeGameToCache(gs game.GameState) error {
	data, err := encodeGameState(gs)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := c.store.Set(ctx, "game:"+gs.ID, data, c.ttl); err != nil {
		return err
	}
	if err := c.store.Set(ctx, "session:"+gs.SessionID+":latest", []byte(gs.ID), c.ttl); err != nil {
		return err
	}
	if err := c.store.Set(ctx, "code:"+string(gs.Code)+":latest", []byte(gs.ID), c.ttl); err != nil {
		return err
	}
	return nil
}

func (c *DistributedCache) getGame(key string) (*game.GameState, error) {
	data, err := c.store.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	return decodeGameState(data)
}

func (c *DistributedCache) getString(key string) (string, error) {
	data, err := c.store.Get(context.Background(), key)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func StartSyncWorker(ctx context.Context, database db.Database, queue SyncQueue, store ...Store) {
	if queue == nil {
		return
	}
	go func() {
		backoff := 500 * time.Millisecond
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			gs, err := queue.DequeueGameSave(ctx)
			if err != nil {
				if errors.Is(err, ErrQueueEmpty) {
					continue
				}
				time.Sleep(backoff)
				if backoff < 10*time.Second {
					backoff *= 2
				}
				continue
			}

			if len(store) > 0 && store[0] != nil {
				latest, err := latestCachedGame(ctx, store[0], gs.ID)
				if err == nil && latest.CacheRevision > 0 && latest.CacheRevision > gs.CacheRevision {
					continue
				}
			}

			if err := database.SaveGame(*gs); err != nil {
				_ = queue.EnqueueGameSave(context.Background(), *gs)
				time.Sleep(backoff)
				continue
			}
			backoff = 500 * time.Millisecond
		}
	}()
}

func latestCachedGame(ctx context.Context, store Store, gameID string) (*game.GameState, error) {
	data, err := store.Get(ctx, "game:"+gameID)
	if err != nil {
		return nil, err
	}
	return decodeGameState(data)
}

func encodeGameState(gs game.GameState) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(gs); err != nil {
		return nil, fmt.Errorf("encode game state: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeGameState(data []byte) (*game.GameState, error) {
	buf := bytes.NewBuffer(data)
	var gs game.GameState
	if err := gob.NewDecoder(buf).Decode(&gs); err != nil {
		return nil, fmt.Errorf("decode game state: %w", err)
	}
	return &gs, nil
}
