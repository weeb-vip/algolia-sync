package redis_processor_kafka

import (
	"context"
	"fmt"
	"github.com/ThatCatDev/ep/v2/event"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
	"time"
)

type RedisProcessor interface {
	Process(ctx context.Context, data event.Event[*kafka.Message, Payload]) (event.Event[*kafka.Message, Payload], error)
}

type RedisProcessorImpl struct {
	redisService redis.RedisService[QueuedItem]
}

func NewRedisProcessor(redisService redis.RedisService[QueuedItem]) RedisProcessor {
	return &RedisProcessorImpl{
		redisService: redisService,
	}
}

func (p *RedisProcessorImpl) Process(ctx context.Context, data event.Event[*kafka.Message, Payload]) (event.Event[*kafka.Message, Payload], error) {
	log := logger.FromCtx(ctx)

	payload := data.Payload

	// Set for every action, not just creates: the log line below dereferences
	// ObjectId unconditionally, so the first delete to arrive would panic.
	if payload.Data.Id == "" {
		return data, fmt.Errorf("cannot queue a record with no id")
	}
	objectID := payload.Data.Id
	payload.Data.ObjectId = &objectID

	// date_rank is computed at index time instead. The parser here accepted one
	// layout that Debezium's ISO 8601 timestamps do not match, then divided
	// unix seconds by 1000, producing a value that was not a timestamp.

	// Create a queued item with action and processed data
	queuedItem := QueuedItem{
		Action:    payload.Action,
		Data:      payload.Data,
		Timestamp: time.Now().Unix(),
	}

	// Store in Redis
	err := p.redisService.StoreData(ctx, queuedItem)
	if err != nil {
		log.Error("Failed to store data in Redis")
		return data, err
	}

	log.Info("Successfully stored data in Redis queue", 
		zap.String("action", string(payload.Action)),
		zap.String("objectId", objectID))

	return data, nil
}