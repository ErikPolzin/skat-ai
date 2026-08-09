package cache

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"runtime"
	"time"

	"skat/game"
	"skat/logger"
	"skat/server/db"
)

type GameCache interface {
	GetGameByID(gameID string) (*game.GameState, error)
	GetGameBySessionID(sessionID string) (*game.GameState, error)
	GetGameBySessionCode(sessionCode string) (*game.GameState, error)
	SaveGame(gs *game.GameState) error
}

// ProfileCache stores authentication profiles by username. Cached entries include
// the password hash so authentication can be completed without a database read.
type ProfileCache interface {
	GetProfileByName(name string) (*db.ProfileEntry, error)
	CacheProfile(profile db.ProfileEntry) error
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
	WriteRevision(ctx context.Context, gs game.GameState, ttl time.Duration) (int64, error)
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

var ErrStaleGameState = errors.New("stale game state")

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
	var invalidCached *game.GameState
	if gs, err := c.getGame("game:" + gameID); err == nil {
		if !hasInvalidMissingDeclarer(gs) {
			return gs, nil
		}
		invalidCached = gs
		logger.Warning("Ignoring cached game with missing declarer: %s", gs)
	}

	gs, err := c.db.GetGameByID(gameID)
	if err != nil {
		return nil, err
	}
	if hasInvalidMissingDeclarer(gs) {
		logger.Error("Loaded DB game with missing declarer: db=%s cache=%s", gs, invalidCached)
		return gs, nil
	}
	if invalidCached != nil && gs.Phase != invalidCached.Phase {
		logger.Warning("DB fallback phase differs from invalid cache: db=%s cache=%s", gs, invalidCached)
	}
	_, _ = c.writeGameToCache(*gs)
	return gs, nil
}

func (c *DistributedCache) GetProfileByName(name string) (*db.ProfileEntry, error) {
	data, err := c.store.Get(context.Background(), "profile:name:"+name)
	if err != nil {
		return nil, err
	}
	var profile db.ProfileEntry
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (c *DistributedCache) CacheProfile(profile db.ProfileEntry) error {
	var data bytes.Buffer
	if err := gob.NewEncoder(&data).Encode(profile); err != nil {
		return err
	}
	return c.store.Set(context.Background(), "profile:name:"+profile.Name, data.Bytes(), c.ttl)
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
	_, _ = c.writeGameToCache(*gs)
	return gs, nil
}

func (c *DistributedCache) SaveGame(gs *game.GameState) error {
	if gs == nil {
		return fmt.Errorf("cannot save nil game")
	}
	snapshot := *gs.Clone()
	if hasInvalidMissingDeclarer(&snapshot) {
		logger.Warning("Saving game with missing declarer: %s caller=%s", &snapshot, callerLocation())
	}
	revision, err := c.writeGameToCache(snapshot)
	if err != nil {
		if errors.Is(err, ErrStaleGameState) {
			logger.Warning("Rejected stale game save: %s expected_revision=%d", &snapshot, snapshot.CacheRevision)
			return err
		} else {
			logger.Warning("Failed to allocate cache revision for %s: %v", snapshot.ID, err)
			return c.db.SaveGame(snapshot)
		}
	}
	gs.CacheRevision = revision
	snapshot.CacheRevision = revision
	if c.queue == nil {
		return c.db.SaveGame(snapshot)
	}
	if err := c.queue.EnqueueGameSave(context.Background(), snapshot); err != nil {
		return c.db.SaveGame(snapshot)
	}
	return nil
}

func (c *DistributedCache) writeGameToCache(gs game.GameState) (int64, error) {
	revisionStore, ok := c.store.(RevisionStore)
	if !ok {
		revision := gs.CacheRevision + 1
		gs.CacheRevision = revision
		data, err := encodeGameState(gs)
		if err != nil {
			return revision, err
		}
		ctx := context.Background()
		if err := c.store.Set(ctx, "game:"+gs.ID, data, c.ttl); err != nil {
			return revision, err
		}
		if err := c.store.Set(ctx, "session:"+gs.SessionID+":latest", []byte(gs.ID), c.ttl); err != nil {
			return revision, err
		}
		if err := c.store.Set(ctx, "code:"+string(gs.Code)+":latest", []byte(gs.ID), c.ttl); err != nil {
			return revision, err
		}
		return revision, nil
	}
	return revisionStore.WriteRevision(context.Background(), gs, c.ttl)
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
					if hasInvalidMissingDeclarer(gs) {
						logger.Warning("Skipping stale queued game with missing declarer: queued=%s latest=%s", gs, latest)
					}
					continue
				}
			}

			if hasInvalidMissingDeclarer(gs) {
				logger.Warning("Sync worker saving game with missing declarer: %s", gs)
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
	if err := gob.NewEncoder(&buf).Encode(toEncodedGameState(gs)); err != nil {
		return nil, fmt.Errorf("encode game state: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeGameState(data []byte) (*game.GameState, error) {
	buf := bytes.NewBuffer(data)
	var encoded encodedGameState
	if err := gob.NewDecoder(buf).Decode(&encoded); err == nil {
		return encoded.toGameState(), nil
	}

	buf = bytes.NewBuffer(data)
	var gs game.GameState
	if err := gob.NewDecoder(buf).Decode(&gs); err != nil {
		return nil, fmt.Errorf("decode game state: %w", err)
	}
	return &gs, nil
}

type encodedGameState struct {
	State              game.GameState
	PlayerSet          [3]bool
	Players            [3]game.PlayerState
	DeclarerSet        bool
	Declarer           game.GamePosition
	TrickWinnerSet     bool
	TrickWinner        game.GamePosition
	ForfeitedPlayerSet bool
	ForfeitedPlayer    game.GamePosition
}

func toEncodedGameState(gs game.GameState) encodedGameState {
	state := gs
	state.Players = [3]*game.PlayerState{{}, {}, {}}
	state.Declarer = nil
	state.TrickWinner = nil
	state.ForfeitedPlayer = nil

	encoded := encodedGameState{State: state}
	for i, player := range gs.Players {
		if player != nil {
			encoded.PlayerSet[i] = true
			encoded.Players[i] = *player
		}
	}
	if gs.Declarer != nil {
		encoded.DeclarerSet = true
		encoded.Declarer = *gs.Declarer
	}
	if gs.TrickWinner != nil {
		encoded.TrickWinnerSet = true
		encoded.TrickWinner = *gs.TrickWinner
	}
	if gs.ForfeitedPlayer != nil {
		encoded.ForfeitedPlayerSet = true
		encoded.ForfeitedPlayer = *gs.ForfeitedPlayer
	}
	return encoded
}

func (e encodedGameState) toGameState() *game.GameState {
	gs := e.State
	gs.Players = [3]*game.PlayerState{}
	for i, playerSet := range e.PlayerSet {
		if playerSet {
			player := e.Players[i]
			gs.Players[i] = &player
		}
	}
	gs.Declarer = nil
	if e.DeclarerSet {
		declarer := e.Declarer
		gs.Declarer = &declarer
	}
	gs.TrickWinner = nil
	if e.TrickWinnerSet {
		trickWinner := e.TrickWinner
		gs.TrickWinner = &trickWinner
	}
	gs.ForfeitedPlayer = nil
	if e.ForfeitedPlayerSet {
		forfeitedPlayer := e.ForfeitedPlayer
		gs.ForfeitedPlayer = &forfeitedPlayer
	}
	return &gs
}

func callerLocation() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s:%d", file, line)
}
