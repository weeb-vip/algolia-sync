package work_index

import (
	"context"
	"fmt"
	"time"

	"github.com/ThatCatDev/ep/v2/event"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
)

// Processor queues a work event for indexing.
//
// Generic over the driver message for the same reason the anime one is: nothing
// here reads DriverMessage, RawData or Headers, only Payload, which ep fills in
// from the message body.
//
// Redis sits between this and Algolia so writes are batched by the sync job
// rather than sent one per event. That matters more here than for anime: a
// manga backfill produces tens of thousands of events in a burst, and one
// Algolia write each would be both slow and expensive.
type Processor[DM any] interface {
	Process(ctx context.Context, data event.Event[DM, Payload]) (event.Event[DM, Payload], error)
}

type ProcessorImpl[DM any] struct {
	redisService redis.RedisService[QueuedItem]
}

func NewProcessor[DM any](redisService redis.RedisService[QueuedItem]) Processor[DM] {
	return &ProcessorImpl[DM]{redisService: redisService}
}

func (p *ProcessorImpl[DM]) Process(ctx context.Context, data event.Event[DM, Payload]) (event.Event[DM, Payload], error) {
	log := logger.FromCtx(ctx)

	payload := data.Payload

	// Set for every action, not just creates: a delete carries an id too, and
	// the log line below reads it unconditionally.
	if payload.Data.Id == "" {
		return data, fmt.Errorf("cannot queue a work with no id")
	}
	objectID := payload.Data.Id
	payload.Data.ObjectId = &objectID

	queued := QueuedItem{
		Action:    payload.Action,
		Data:      payload.Data,
		Timestamp: time.Now().Unix(),
	}

	if err := p.redisService.StoreData(ctx, queued); err != nil {
		log.Error("failed to queue work for indexing",
			zap.String("id", payload.Data.Id),
			zap.String("action", string(payload.Action)),
			zap.Error(err))

		return data, err
	}

	log.Info("queued work for indexing",
		zap.String("id", payload.Data.Id),
		zap.String("action", string(payload.Action)))

	return data, nil
}
