package algolia_processor

import (
	"fmt"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"net/url"
	"time"
)

type ImageProcessor interface {
	Process(ctx context.Context, data Payload) error
}

type ImageProcessorImpl struct {
	algolia.AlgoliaService[Schema]
}

func NewImageProcessor(algoliaService algolia.AlgoliaService[Schema]) ImageProcessor {
	return &ImageProcessorImpl{
		AlgoliaService: algoliaService,
	}
}

func (p *ImageProcessorImpl) Process(ctx context.Context, data Payload) error {
	log := logger.FromCtx(ctx)
	if data.Action == CreateAction {
		log.Info("Processing create action")
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
				log.Warn("Failed to parse start date, continuing without date_rank",
					zap.String("startDate", *data.Data.StartDate),
					zap.Error(err))
			} else {
				dateRank := startDate.Unix()
				dateRank = dateRank / 1000
				data.Data.DateRank = &dateRank
			}
		}

		_, err := p.AlgoliaService.AddToIndex(ctx, data.Data)
		if err != nil {
			return err
		}
	}

	return nil
}
