package redis

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"go.uber.org/zap"
)

type RedisService[T any] interface {
	StoreData(ctx context.Context, data T) error
	GetAllData(ctx context.Context) ([]T, error)
	ClearData(ctx context.Context) error
}

type RedisServiceImpl[T any] struct {
	client *redis.Client
	key    string
}

func NewRedisService[T any](ctx context.Context, redisCfg config.RedisConfig) RedisService[T] {
	log := logger.FromCtx(ctx)

	opts, err := redis.ParseURL(redisCfg.URL)
	if err != nil {
		log.Fatal("Failed to parse Redis URL", zap.Error(err))
	}

	if redisCfg.Password != "" {
		opts.Password = redisCfg.Password
	}
	opts.DB = redisCfg.DB

	client := redis.NewClient(opts)

	// Test connection
	_, err = client.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Successfully connected to Redis")

	return &RedisServiceImpl[T]{
		client: client,
		key:    redisCfg.Key,
	}
}

func (r *RedisServiceImpl[T]) StoreData(ctx context.Context, data T) error {
	log := logger.FromCtx(ctx)

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Error("Failed to marshal data to JSON", zap.Error(err))
		return err
	}

	err = r.client.LPush(ctx, r.key, jsonData).Err()
	if err != nil {
		log.Error("Failed to store data in Redis", zap.Error(err))
		return err
	}

	log.Debug("Data stored in Redis")
	return nil
}

// GetAllData claims the queue by renaming it, then reads the claimed copy.
//
// It used to LRANGE the live key and let the caller DEL it afterwards, which
// loses everything pushed in between: the consumer writes continuously, so any
// item arriving between the read and the delete was destroyed without ever
// being sent to Algolia. Silent, and proportional to load -- a catalogue replay
// of ~30,000 records dropped 295 of them this way.
//
// RENAME is atomic. New pushes land on a fresh key while this batch is worked
// from the claimed one, so nothing can slip through the gap.
func (r *RedisServiceImpl[T]) GetAllData(ctx context.Context) ([]T, error) {
	log := logger.FromCtx(ctx)

	// A leftover claimed batch means the previous run died before finishing it.
	// Work that first: RENAME overwrites its destination, so claiming again
	// would destroy the orphaned batch -- the same silent loss this is fixing.
	orphaned, err := r.client.Exists(ctx, r.claimedKey()).Result()
	if err != nil {
		log.Error("Failed to check for an orphaned batch", zap.Error(err))
		return nil, err
	}
	if orphaned > 0 {
		log.Warn("recovering a batch left behind by a previous run",
			zap.String("key", r.claimedKey()))
	} else if err := r.client.Rename(ctx, r.key, r.claimedKey()).Err(); err != nil {
		// Nothing to claim: an empty queue has no key to rename.
		if err == redis.Nil || err.Error() == "ERR no such key" {
			return nil, nil
		}
		log.Error("Failed to claim the Redis queue", zap.Error(err))
		return nil, err
	}

	items, err := r.client.LRange(ctx, r.claimedKey(), 0, -1).Result()
	if err != nil {
		log.Error("Failed to get data from Redis", zap.Error(err))
		return nil, err
	}

	var results []T
	for _, item := range items {
		var data T
		err := json.Unmarshal([]byte(item), &data)
		if err != nil {
			log.Warn("Failed to unmarshal item, skipping", zap.Error(err))
			continue
		}
		results = append(results, data)
	}

	log.Info("Retrieved data from Redis", zap.Int("count", len(results)))
	return results, nil
}

// claimedKey holds the batch currently being worked. Anything that arrives
// mid-sync accumulates on the live key and is picked up by the next run.
func (r *RedisServiceImpl[T]) claimedKey() string {
	return r.key + ":claimed"
}

// ClearData discards the claimed batch, not the live queue.
func (r *RedisServiceImpl[T]) ClearData(ctx context.Context) error {
	log := logger.FromCtx(ctx)

	err := r.client.Del(ctx, r.claimedKey()).Err()
	if err != nil {
		log.Error("Failed to clear data from Redis", zap.Error(err))
		return err
	}

	log.Info("Cleared all data from Redis")
	return nil
}
