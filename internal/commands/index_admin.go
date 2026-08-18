package commands

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/redis_processor"
	"go.uber.org/zap"
)

// applySettingsCmd writes the index configuration from code.
//
// Settings used to live only in the Algolia dashboard, which means the
// behaviour of search was not reviewable, not versioned, and not reproducible
// on a new index. Keeping them here makes rebuilding an index a repeatable
// operation rather than an afternoon of clicking.
var applySettingsCmd = &cobra.Command{
	Use:   "apply-index-settings",
	Short: "Write searchable attributes, facets and ranking to the configured index",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := logger.WithCtx(context.Background(), logger.Get())

		svc := algolia.NewAlgoliaServiceWithoutTimer[redis_processor.AnimeDocument](ctx, cfg.AlgoliaConfig)
		if err := svc.ApplySettings(ctx); err != nil {
			return err
		}
		logger.FromCtx(ctx).Info("settings applied", zap.String("index", cfg.AlgoliaConfig.Index))
		return nil
	},
}

var swapFrom string

// swapIndexCmd promotes a freshly built index over the live one.
//
// Renaming fields (genres -> tags, episodes -> episode_count) is breaking: a
// half-migrated index serves some records the frontend can read and some it
// cannot. Algolia's move is atomic, so the switch happens between one search
// and the next, and the previous index remains until it is explicitly removed
// -- which is the rollback.
var swapIndexCmd = &cobra.Command{
	Use:   "swap-index",
	Short: "Atomically move --from over the configured index",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := logger.WithCtx(context.Background(), logger.Get())
		log := logger.FromCtx(ctx)

		if swapFrom == "" {
			return cmd.Usage()
		}
		if swapFrom == cfg.AlgoliaConfig.Index {
			log.Error("source and destination are the same index",
				zap.String("index", swapFrom))
			return nil
		}

		svc := algolia.NewAlgoliaServiceWithoutTimer[redis_processor.AnimeDocument](ctx, cfg.AlgoliaConfig)
		if err := svc.ReplaceLiveIndex(ctx, swapFrom); err != nil {
			return err
		}
		log.Info("index swapped",
			zap.String("from", swapFrom), zap.String("to", cfg.AlgoliaConfig.Index))
		return nil
	},
}

func init() {
	swapIndexCmd.Flags().StringVar(&swapFrom, "from", "",
		"index to promote over the configured one (required)")
	rootCmd.AddCommand(applySettingsCmd)
	rootCmd.AddCommand(swapIndexCmd)
}
