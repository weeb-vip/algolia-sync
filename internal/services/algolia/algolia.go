package algolia

import (
	"context"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"go.uber.org/zap"
	"time"
)

type AlgoliaService[T any] interface {
	AddToIndex(ctx context.Context, object T) (res search.GroupBatchRes, err error)
	Flush(ctx context.Context) (res search.GroupBatchRes, err error)
}

type AlgoliaServiceImpl[T any] struct {
	AlgoliaSearch *search.Client
	Index         *search.Index
	AddBatch      []T
	DeleteBatch   []T
}

func AutoFlush[T any](ctx context.Context, service AlgoliaService[T]) {
	log := logger.FromCtx(ctx)
	_, err := service.Flush(ctx)
	if err != nil {
		log.Error(err.Error())
	}
}

func NewAlgoliaService[T any](ctx context.Context, algoliaCfg config.AlgoliaConfig) AlgoliaService[T] {
	client := search.NewClient(algoliaCfg.AppID, algoliaCfg.APIKey)
	service := &AlgoliaServiceImpl[T]{
		AlgoliaSearch: client,
		Index:         client.InitIndex(algoliaCfg.Index),
		AddBatch:      make([]T, 0),
		DeleteBatch:   make([]T, 0),
	}
	timeout := time.Duration(algoliaCfg.FlushTimeout) * time.Second
	// start autoflush which runs ever 5 minutes
	go func() {
		for {
			AutoFlush[T](ctx, service)
			<-time.After(timeout)
		}
	}()

	return service
}

func NewAlgoliaServiceWithoutTimer[T any](ctx context.Context, algoliaCfg config.AlgoliaConfig) AlgoliaService[T] {
	client := search.NewClient(algoliaCfg.AppID, algoliaCfg.APIKey)
	service := &AlgoliaServiceImpl[T]{
		AlgoliaSearch: client,
		Index:         client.InitIndex(algoliaCfg.Index),
		AddBatch:      make([]T, 0),
		DeleteBatch:   make([]T, 0),
	}
	// No timer-based auto flush for cron job usage
	return service
}

func (a *AlgoliaServiceImpl[T]) AddToIndex(ctx context.Context, object T) (res search.GroupBatchRes, err error) {
	log := logger.FromCtx(ctx)
	log.Info("adding to batch...")
	a.AddBatch = append(a.AddBatch, object)
	if len(a.AddBatch) >= 1000 {
		log.Info("Adding to algolia...")
		res, err = a.Index.SaveObjects(a.AddBatch)
		if err != nil {
			return res, err
		}
		a.AddBatch = make([]T, 0)
	}

	return res, err
}

//func (a *AlgoliaServiceImpl[T]) DeleteFromIndex(object interface{}) (res search.BatchRes, err error) {
//	a.DeleteBatch = append(a.DeleteBatch, object)
//	if len(a.DeleteBatch) >= 1000 {
//		res, err = a.Index.DeleteObjects(a.DeleteBatch)
//		if err != nil {
//			return res, err
//		}
//		a.DeleteBatch = make([]interface{}, 0)
//	}
//	return res, err
//}

func (a *AlgoliaServiceImpl[T]) Flush(ctx context.Context) (res search.GroupBatchRes, err error) {
	log := logger.FromCtx(ctx)
	if len(a.AddBatch) > 0 {
		log.With(zap.Int("batchSize", len(a.AddBatch))).Info("Flushing algolia...")
		res, err = a.Index.SaveObjects(a.AddBatch)
		if err != nil {
			return res, err
		}
		a.AddBatch = make([]T, 0)
	}
	log.Info("Nothing to flush")
	//if len(a.DeleteBatch) > 0 {
	//	res, err = a.Index().DeleteObjects(a.DeleteBatch)
	//	if err != nil {
	//		return res, err
	//	}
	//	a.DeleteBatch = make([]interface{}, 0)
	//}
	return res, err
}
