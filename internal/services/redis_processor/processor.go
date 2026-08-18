package redis_processor

import (
	"fmt"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"time"
)

type ImageProcessor interface {
	Process(ctx context.Context, data Payload) error
}

type ImageProcessorImpl struct {
	redisService redis.RedisService[QueuedItem]
}

func NewImageProcessor(redisService redis.RedisService[QueuedItem]) ImageProcessor {
	return &ImageProcessorImpl{
		redisService: redisService,
	}
}

func (p *ImageProcessorImpl) Process(ctx context.Context, data Payload) error {
	log := logger.FromCtx(ctx)

	// ObjectID is set for every action, not just creates. It used to be
	// assigned inside the create branch while the log line below dereferenced
	// it unconditionally, so the first delete or update to arrive would have
	// panicked -- latent only because anime-sync labels everything "create".
	if data.Data.Id == "" {
		return fmt.Errorf("cannot queue a record with no id")
	}
	objectID := data.Data.Id
	data.Data.ObjectId = &objectID

	// date_rank is no longer computed here. It was parsed with a single layout
	// that Debezium's ISO 8601 timestamps do not match, and then divided by
	// 1000, which turned unix seconds into a number that was not a timestamp at
	// all. Schema.ToDocument does it once, correctly, at index time.

	// Create a queued item with action and processed data
	queuedItem := QueuedItem{
		Action:    data.Action,
		Data:      data.Data,
		Timestamp: time.Now().Unix(),
	}

	// Store in Redis
	err := p.redisService.StoreData(ctx, queuedItem)
	if err != nil {
		log.Error("Failed to store data in Redis")
		return err
	}

	log.Info("Successfully stored data in Redis queue",
		zap.String("action", string(data.Action)),
		zap.String("objectId", objectID))

	return nil
}
