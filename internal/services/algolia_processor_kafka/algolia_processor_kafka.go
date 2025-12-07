package algolia_processor_kafka

import (
	"context"
	"fmt"
	"github.com/ThatCatDev/ep/v2/event"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"go.uber.org/zap"
	"net/url"
	"time"
)

type AlgoliaProcessor interface {
	Process(ctx context.Context, data event.Event[*kafka.Message, Payload]) (event.Event[*kafka.Message, Payload], error)
}

type AlgoliaProcessorImpl struct {
	algolia.AlgoliaService[Schema]
}

func NewAlgoliaProcessor(algoliaService algolia.AlgoliaService[Schema]) AlgoliaProcessor {
	return &AlgoliaProcessorImpl{
		AlgoliaService: algoliaService,
	}
}

func (p *AlgoliaProcessorImpl) Process(ctx context.Context, data event.Event[*kafka.Message, Payload]) (event.Event[*kafka.Message, Payload], error) {
	log := logger.FromCtx(ctx)

	payload := data.Payload

	if payload.Action == CreateAction {
		log.Info("Processing create action")
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

		_, err := p.AlgoliaService.AddToIndex(ctx, payload.Data)
		if err != nil {
			return data, err
		}
	}

	return data, nil
}
