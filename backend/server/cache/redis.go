package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	"skat/game"

	"github.com/redis/go-redis/v9"
)

const syncQueueKey = "skat:cache-sync:games"

type RedisBackend struct {
	client *redis.Client
}

func NewRedisBackend(addr, password string, db int) *RedisBackend {
	return &RedisBackend{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

func (r *RedisBackend) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisBackend) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.client.Get(ctx, "skat:cache:"+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return data, err
}

func (r *RedisBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, "skat:cache:"+key, value, ttl).Err()
}

func (r *RedisBackend) WriteRevision(ctx context.Context, gs game.GameState, ttl time.Duration) (int64, error) {
	revisionKey := "skat:cache-revision:game:" + gs.ID
	gameKey := "skat:cache:game:" + gs.ID
	sessionKey := "skat:cache:session:" + gs.SessionID + ":latest"
	codeKey := "skat:cache:code:" + string(gs.Code) + ":latest"

	for {
		var nextRevision int64
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			currentRevision, err := redisInt64OrZero(tx.Get(ctx, revisionKey).Result())
			if err != nil {
				return err
			}
			if currentRevision > gs.CacheRevision {
				return ErrStaleGameState
			}

			nextRevision = currentRevision + 1
			gs.CacheRevision = nextRevision
			data, err := encodeGameState(gs)
			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, revisionKey, nextRevision, ttl)
				pipe.Set(ctx, gameKey, data, ttl)
				pipe.Set(ctx, sessionKey, []byte(gs.ID), ttl)
				pipe.Set(ctx, codeKey, []byte(gs.ID), ttl)
				return nil
			})
			return err
		}, revisionKey)
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}
		if err != nil {
			return 0, err
		}
		return nextRevision, nil
	}
}

func (r *RedisBackend) EnqueueGameSave(ctx context.Context, gs game.GameState) error {
	data, err := encodeGameState(gs)
	if err != nil {
		return err
	}
	return r.client.RPush(ctx, syncQueueKey, data).Err()
}

func (r *RedisBackend) DequeueGameSave(ctx context.Context) (*game.GameState, error) {
	items, err := r.client.BLPop(ctx, 5*time.Second, syncQueueKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrQueueEmpty
	}
	if err != nil {
		return nil, err
	}
	if len(items) < 2 {
		return nil, ErrQueueEmpty
	}
	return decodeGameState([]byte(items[1]))
}

func (r *RedisBackend) MarkOnline(ctx context.Context, profileID, nodeID string, ttl time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Set(ctx, "skat:presence:"+profileID, nodeID, ttl)
	pipe.SAdd(ctx, "skat:presence:online", profileID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisBackend) MarkOffline(ctx context.Context, profileID, nodeID string) error {
	current, err := r.client.Get(ctx, "skat:presence:"+profileID).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if current != "" && current != nodeID {
		return nil
	}
	pipe := r.client.Pipeline()
	pipe.Del(ctx, "skat:presence:"+profileID)
	pipe.SRem(ctx, "skat:presence:online", profileID)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisBackend) OnlineIDs(ctx context.Context) ([]string, error) {
	ids, err := r.client.SMembers(ctx, "skat:presence:online").Result()
	if err != nil {
		return nil, err
	}
	online := make([]string, 0, len(ids))
	for _, id := range ids {
		exists, err := r.client.Exists(ctx, "skat:presence:"+id).Result()
		if err != nil {
			return nil, err
		}
		if exists == 1 {
			online = append(online, id)
			continue
		}
		_ = r.client.SRem(ctx, "skat:presence:online", id).Err()
	}
	return online, nil
}

func (r *RedisBackend) IsOnline(ctx context.Context, profileID string) (bool, error) {
	exists, err := r.client.Exists(ctx, "skat:presence:"+profileID).Result()
	return exists == 1, err
}

func (r *RedisBackend) PublishClientMessage(ctx context.Context, payload []byte) error {
	return r.client.Publish(ctx, "skat:clients:messages", payload).Err()
}

func (r *RedisBackend) SubscribeClientMessages(ctx context.Context) (<-chan []byte, error) {
	pubsub := r.client.Subscribe(ctx, "skat:clients:messages")
	if _, err := pubsub.Receive(ctx); err != nil {
		return nil, err
	}
	out := make(chan []byte)
	go func() {
		defer close(out)
		defer pubsub.Close()
		for msg := range pubsub.Channel() {
			payload := []byte(msg.Payload)
			select {
			case out <- payload:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (r *RedisBackend) Close() error {
	return r.client.Close()
}

// Private utilities

func redisInt64OrZero(value string, err error) (int64, error) {
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, 64)
}
