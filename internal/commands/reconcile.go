package commands

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/catalogue"
	"github.com/weeb-vip/algolia-sync/internal/services/redis_processor"
	"go.uber.org/zap"
)

var (
	reconcileApply     bool
	reconcileMaxDelete int
)

// reconcileCmd compares the index against the catalogue and removes records
// whose anime no longer exists.
//
// The event stream alone cannot keep the index correct. It is a sequence of
// deltas, so anything missed is missed permanently -- a delete dropped while a
// consumer was down leaves a record that nothing will ever revisit, because
// there is no second event for a row that no longer exists. That is how the
// index came to hold ~2,860 anime that had been merged away as duplicates,
// every one of them a search result leading to a 404.
//
// This is the periodic correction: state comparison rather than event replay.
var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Remove index records whose anime no longer exists",
	Long: `Compares every objectID in the Algolia index against the catalogue and
deletes the ones that no longer correspond to an anime.

Reports without changing anything unless --apply is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := logger.WithCtx(context.Background(), logger.Get())
		log := logger.FromCtx(ctx)

		source := catalogue.New(cfg.SourceConfig.GraphQLHost)
		log.Info("reading catalogue", zap.String("endpoint", cfg.SourceConfig.GraphQLHost))
		entries, err := source.All(ctx)
		if err != nil {
			return err
		}

		live := make(map[string]struct{}, len(entries))
		missingSlug := 0
		for _, e := range entries {
			live[e.ID] = struct{}{}
			if e.Slug == nil || *e.Slug == "" {
				missingSlug++
			}
		}

		algoliaService := algolia.NewAlgoliaServiceWithoutTimer[redis_processor.AnimeDocument](ctx, cfg.AlgoliaConfig)
		indexed, err := algoliaService.AllObjectIDs(ctx)
		if err != nil {
			return err
		}

		orphans := make([]string, 0)
		for id := range indexed {
			if _, ok := live[id]; !ok {
				orphans = append(orphans, id)
			}
		}

		absent := 0
		for id := range live {
			if _, ok := indexed[id]; !ok {
				absent++
			}
		}

		log.Info("reconcile summary",
			zap.Int("catalogue", len(entries)),
			zap.Int("indexed", len(indexed)),
			zap.Int("orphaned_in_index", len(orphans)),
			zap.Int("missing_from_index", absent),
			zap.Int("catalogue_without_slug", missingSlug))

		if len(orphans) == 0 {
			log.Info("nothing to remove")
			return nil
		}

		// A run that wants to delete an implausible share of the index is more
		// likely to be reading a degraded catalogue than to have found that much
		// genuine drift. Stop and make a human look.
		if reconcileMaxDelete > 0 && len(orphans) > reconcileMaxDelete {
			log.Error("refusing to delete: more orphans than the safety limit allows",
				zap.Int("orphans", len(orphans)),
				zap.Int("limit", reconcileMaxDelete),
				zap.String("hint", "re-run with a higher --max-delete once the count is understood"))
			return nil
		}

		if !reconcileApply {
			sample := orphans
			if len(sample) > 10 {
				sample = sample[:10]
			}
			log.Info("dry run; pass --apply to delete",
				zap.Strings("sample", sample))
			return nil
		}

		for _, id := range orphans {
			if err := algoliaService.DeleteFromIndex(ctx, id); err != nil {
				log.Error("failed to queue delete", zap.String("objectId", id), zap.Error(err))
				return err
			}
		}
		if _, err := algoliaService.Flush(ctx); err != nil {
			return err
		}

		log.Info("removed orphaned records", zap.Int("count", len(orphans)))
		return nil
	},
}

func init() {
	reconcileCmd.Flags().BoolVar(&reconcileApply, "apply", false,
		"actually delete the orphaned records (default is a dry run)")
	reconcileCmd.Flags().IntVar(&reconcileMaxDelete, "max-delete", 5000,
		"refuse to delete more than this many records in one run; 0 disables the check")
	rootCmd.AddCommand(reconcileCmd)
}
