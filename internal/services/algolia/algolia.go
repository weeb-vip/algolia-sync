package algolia

import (
	"context"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/opt"
	"github.com/algolia/algoliasearch-client-go/v3/algolia/search"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"go.uber.org/zap"
	"io"
	"time"
)

type AlgoliaService[T any] interface {
	AddToIndex(ctx context.Context, object T) (res search.GroupBatchRes, err error)
	DeleteFromIndex(ctx context.Context, objectID string) error
	Flush(ctx context.Context) (res search.GroupBatchRes, err error)
	// AllObjectIDs walks the whole index. Used by reconcile to find records
	// whose source row is gone.
	AllObjectIDs(ctx context.Context) (map[string]struct{}, error)
	ApplySettings(ctx context.Context) error
	ReplaceLiveIndex(ctx context.Context, sourceIndex string) error
}

type AlgoliaServiceImpl[T any] struct {
	AlgoliaSearch *search.Client
	Index         *search.Index
	// The v3 client's Index has no accessor for its own name, and both the
	// settings and swap calls need it.
	IndexName   string
	AddBatch    []T
	DeleteBatch []string
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
		IndexName:     algoliaCfg.Index,
		AddBatch:      make([]T, 0),
		DeleteBatch:   make([]string, 0),
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
		IndexName:     algoliaCfg.Index,
		AddBatch:      make([]T, 0),
		DeleteBatch:   make([]string, 0),
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

// DeleteFromIndex removes a record. Batched like adds, because a reconcile
// run can produce thousands of deletions at once and one HTTP call each would
// be both slow and a good way to hit the rate limit.
func (a *AlgoliaServiceImpl[T]) DeleteFromIndex(ctx context.Context, objectID string) error {
	log := logger.FromCtx(ctx)
	a.DeleteBatch = append(a.DeleteBatch, objectID)
	if len(a.DeleteBatch) >= 1000 {
		log.Info("deleting batch from algolia", zap.Int("batchSize", len(a.DeleteBatch)))
		if _, err := a.Index.DeleteObjects(a.DeleteBatch); err != nil {
			return err
		}
		a.DeleteBatch = make([]string, 0)
	}
	return nil
}

// AllObjectIDs pages through the entire index. Only objectID is requested, so
// walking ~30,000 records stays cheap.
func (a *AlgoliaServiceImpl[T]) AllObjectIDs(ctx context.Context) (map[string]struct{}, error) {
	ids := make(map[string]struct{})
	it, err := a.Index.BrowseObjects(opt.AttributesToRetrieve("objectID"))
	if err != nil {
		return nil, err
	}
	for {
		var rec struct {
			ObjectID string `json:"objectID"`
		}
		_, err := it.Next(&rec)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if rec.ObjectID != "" {
			ids[rec.ObjectID] = struct{}{}
		}
	}
	return ids, nil
}

// ApplySettings makes the index's behaviour explicit rather than inherited
// from whatever was clicked in the dashboard.
//
// The important line is searchableAttributes: description is deliberately NOT
// in it. A synopsis is a thousand characters of prose, and indexing it means a
// plot summary mentioning "Tokyo" competes with an anime actually called Tokyo
// something. It is still stored and returned for display, just not matched on.
//
// Ordered attributes matter: Algolia treats earlier entries as more important,
// so a title hit outranks a studio hit.
func (a *AlgoliaServiceImpl[T]) ApplySettings(ctx context.Context) error {
	log := logger.FromCtx(ctx)
	log.Info("applying index settings", zap.String("index", a.IndexName))

	_, err := a.Index.SetSettings(search.Settings{
		// title_romaji is deliberately absent: it is populated in 0 of 29,634
		// rows in postgres, so it can never match, and Algolia warns that a
		// record carrying an attribute ranks above one that does not -- which
		// would quietly bias results the moment the scraper started filling it
		// for some anime and not others. Add it back when the data exists.
		// title_kanji is empty for the same reason and likewise not listed.
		SearchableAttributes: opt.SearchableAttributes(
			"title_en,title_jp,title_synonyms",
			"studios",
			"tags",
		),
		AttributesForFaceting: opt.AttributesForFaceting(
			"searchable(tags)",
			"searchable(studios)",
			"type",
			"status",
			"year",
			// filterOnly: never shown as a facet, but usable in filters, which
			// is what the reconcile and any id-based lookup need.
			"filterOnly(id)",
			"filterOnly(slug)",
		),
		// Ties on text relevance fall back to how well known the anime is.
		// rank_sort is MyAnimeList's position where lower is better, with
		// unranked entries carrying a sentinel so they sort last. Sorting on
		// `ranking` directly does the opposite: an omitted attribute scores
		// better than any real value, so every unranked anime won.
		CustomRanking: opt.CustomRanking("asc(rank_sort)"),
		// Returned but never matched on.
		AttributesToRetrieve: opt.AttributesToRetrieve("*"),
	})
	return err
}

// ReplaceLiveIndex atomically moves sourceIndex over this one.
//
// Algolia's MoveIndex is a swap, not a copy-then-delete, so searches never
// observe a half-populated index. That is what makes a breaking field rename
// safe: the new index is built and verified in full first, and only then does
// anything start reading it.
func (a *AlgoliaServiceImpl[T]) ReplaceLiveIndex(ctx context.Context, sourceIndex string) error {
	log := logger.FromCtx(ctx)
	log.Info("swapping index into place",
		zap.String("from", sourceIndex), zap.String("to", a.IndexName))

	res, err := a.AlgoliaSearch.MoveIndex(sourceIndex, a.IndexName)
	if err != nil {
		return err
	}
	return res.Wait()
}

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
	if len(a.DeleteBatch) > 0 {
		log.With(zap.Int("batchSize", len(a.DeleteBatch))).Info("Flushing algolia deletes...")
		if _, err := a.Index.DeleteObjects(a.DeleteBatch); err != nil {
			return res, err
		}
		a.DeleteBatch = make([]string, 0)
	}
	return res, err
}
