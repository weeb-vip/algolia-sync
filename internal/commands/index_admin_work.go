package commands

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/work_index"
	"go.uber.org/zap"
)

// applyWorkSettingsCmd writes the works index configuration from code.
//
// Separate from apply-index-settings because the two indices hold different
// records and the settings say so. Running the anime command against the works
// index would declare studios and tags searchable, which no work has, and
// would leave authors -- the field people actually search a manga by -- out.
//
// ALGOLIA_INDEX selects which index this writes to, so the command has to be
// run with the works index configured.
var applyWorkSettingsCmd = &cobra.Command{
	Use:   "apply-work-index-settings",
	Short: "Write searchable attributes, facets and ranking to the configured works index",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := logger.WithCtx(context.Background(), logger.Get())

		svc := algolia.NewAlgoliaServiceWithoutTimer[work_index.WorkDocument](ctx, cfg.AlgoliaConfig)
		if err := svc.ApplyWorkSettings(ctx); err != nil {
			return err
		}
		logger.FromCtx(ctx).Info("work settings applied", zap.String("index", cfg.AlgoliaConfig.Index))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(applyWorkSettingsCmd)
}
