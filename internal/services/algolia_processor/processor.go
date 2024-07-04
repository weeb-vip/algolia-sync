package algolia_processor

import (
	"fmt"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"golang.org/x/net/context"
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
		if data.Data.AnidbID != nil {
			data.Data.ObjectId = data.Data.AnidbID
		} else {
			if data.Data.TitleEn != nil && data.Data.Type != nil && data.Data.StartDate != nil {
				id := fmt.Sprintf("%s-%s-%s", *data.Data.TitleEn, *data.Data.Type, *data.Data.StartDate)

				data.Data.ObjectId = &id
			}
		}
		if data.Data.ObjectId == nil {
			return fmt.Errorf("object id is nil")
		}
		_, err := p.AlgoliaService.AddToIndex(ctx, data.Data)
		if err != nil {
			return err
		}
	}

	return nil
}
