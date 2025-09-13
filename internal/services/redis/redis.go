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

func (r *RedisServiceImpl[T]) GetAllData(ctx context.Context) ([]T, error) {
	log := logger.FromCtx(ctx)
	
	// Get all items from the list
	items, err := r.client.LRange(ctx, r.key, 0, -1).Result()
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

func (r *RedisServiceImpl[T]) ClearData(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	
	err := r.client.Del(ctx, r.key).Err()
	if err != nil {
		log.Error("Failed to clear data from Redis", zap.Error(err))
		return err
	}
	
	log.Info("Cleared all data from Redis")
	return nil
}