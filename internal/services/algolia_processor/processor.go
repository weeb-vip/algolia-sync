package algolia_processor

import (
	"fmt"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"golang.org/x/net/context"
	"strings"
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
		if data.Data.TitleEn != nil {
			// convert title to lowercase and replace spaces with underscores
			title := strings.ToLower(*data.Data.TitleEn)
			title = strings.ReplaceAll(title, " ", "_")
			data.Data.ObjectId = &title
		} else if data.Data.TitleJp != nil {
			// convert title to lowercase and replace spaces with underscores
			title := strings.ToLower(*data.Data.TitleJp)
			title = strings.ReplaceAll(title, " ", "_")
			data.Data.ObjectId = &title
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
