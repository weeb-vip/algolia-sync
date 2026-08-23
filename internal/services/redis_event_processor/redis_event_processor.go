package redis_event_processor

import (
	"context"
	"fmt"
	"github.com/ThatCatDev/ep/v2/event"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
	"time"
)

// The driver message type is a parameter because the processor never looks at
// it. Nothing here reads DriverMessage, RawData or Headers -- only Payload,
// which ep populates from the message body. This package was called
// redis_processor_kafka purely because *kafka.Message was baked into the
// signature; none of the logic was ever Kafka-specific.
//
// Renamed to redis_event_processor rather than redis_processor: that name is
// taken by the document-mapping package, which reconcile, sync-redis-to-algolia
// and the index admin commands all use and which has nothing to do with events.
type RedisProcessor[DM any] interface {
	Process(ctx context.Context, data event.Event[DM, Payload]) (event.Event[DM, Payload], error)
}

type RedisProcessorImpl[DM any] struct {
	redisService redis.RedisService[QueuedItem]
}

func NewRedisProcessor[DM any](redisService redis.RedisService[QueuedItem]) RedisProcessor[DM] {
	return &RedisProcessorImpl[DM]{
		redisService: redisService,
	}
}

func (p *RedisProcessorImpl[DM]) Process(ctx context.Context, data event.Event[DM, Payload]) (event.Event[DM, Payload], error) {
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
