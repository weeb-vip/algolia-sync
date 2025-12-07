package redis_processor_kafka

import (
	"context"
	"fmt"
	"github.com/ThatCatDev/ep/v2/event"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"go.uber.org/zap"
	"net/url"
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

	// Process the data similar to the original processor
	if payload.Action == CreateAction {
		log.Info("Processing create action for Redis storage")
		payload.Data.ObjectId = &payload.Data.Id
		if payload.Data.ObjectId == nil {
			return data, fmt.Errorf("object id is nil")
		}
		// convert to url encoded string
		encoded := url.QueryEscape(*payload.Data.ObjectId)
		payload.Data.ObjectId = &encoded
		// convert startDate to unix timestamp
		if payload.Data.StartDate != nil {
			// format of startDate 2007-04-02 04:00:00
			startDate, err := time.Parse("2006-01-02 15:04:05", *payload.Data.StartDate)
			if err != nil {
				log.Warn("Failed to parse start date, continuing without date_rank",
					zap.String("startDate", *payload.Data.StartDate),
					zap.Error(err))
			} else {
				dateRank := startDate.Unix()
				dateRank = dateRank / 1000
				payload.Data.DateRank = &dateRank
			}
		}
	}

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
		zap.String("objectId", *payload.Data.ObjectId))

	return data, nil
}