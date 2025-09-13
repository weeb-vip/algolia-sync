package redis_processor

import (
	"fmt"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"net/url"
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
	
	// Process the data similar to the original processor
	if data.Action == CreateAction {
		log.Info("Processing create action for Redis storage")
		data.Data.ObjectId = &data.Data.Id
		if data.Data.ObjectId == nil {
			return fmt.Errorf("object id is nil")
		}
		// convert to url encoded string
		encoded := url.QueryEscape(*data.Data.ObjectId)
		data.Data.ObjectId = &encoded
		// convert startDate to unix timestamp
		if data.Data.StartDate != nil {
			// format of startDate 2007-04-02 04:00:00
			startDate, err := time.Parse("2006-01-02 15:04:05", *data.Data.StartDate)
			if err != nil {
				return err
			}
			dateRank := startDate.Unix()
			dateRank = dateRank / 1000
			data.Data.DateRank = &dateRank
		}
	}

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
		zap.String("objectId", *data.Data.ObjectId))

	return nil
}