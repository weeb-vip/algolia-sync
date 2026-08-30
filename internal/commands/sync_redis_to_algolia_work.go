package commands

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"github.com/weeb-vip/algolia-sync/internal/services/work_index"
	"go.uber.org/zap"
)

// syncRedisToAlgoliaWorkCmd ships queued work records to the works index.
//
// The works counterpart of sync-redis-to-algolia. Which queue and which index
// are configuration -- REDIS_KEY and ALGOLIA_INDEX -- so this runs as a second
// cron job on the same image, pointed at the works queue.
//
// Sending in batches from Redis rather than one write per event matters more
// here than for anime: a manga backfill produces tens of thousands of records
// in a burst, and a write each would be slow and costly.
var syncRedisToAlgoliaWorkCmd = &cobra.Command{
	Use:   "sync-redis-to-algolia-work",
	Short: "Sync queued works from Redis to the Algolia works index (for cron job usage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := context.Background()
		log := logger.Get()
		ctx = logger.WithCtx(ctx, log)

		log.Info("Starting Redis to Algolia works sync job")

		redisService := redis.NewRedisService[work_index.QueuedItem](ctx, cfg.RedisConfig)
		algoliaService := algolia.NewAlgoliaServiceWithoutTimer[work_index.WorkDocument](ctx, cfg.AlgoliaConfig)

		queuedItems, err := redisService.GetAllData(ctx)
		if err != nil {
			log.Error("Failed to get works from Redis", zap.Error(err))
			return err
		}

		if len(queuedItems) == 0 {
			log.Info("No works to sync from Redis to Algolia")
			return nil
		}

		log.Info("Processing queued works", zap.Int("count", len(queuedItems)))

		successCount := 0
		failCount := 0

		for _, item := range queuedItems {
			switch item.Action {
			case work_index.CreateAction, work_index.UpdateAction:
				doc := item.Data.ToDocument()
				if _, err := algoliaService.AddToIndex(ctx, doc); err != nil {
					log.Error("Failed to add work to Algolia",
						zap.Error(err),
						zap.String("action", string(item.Action)),
						zap.String("objectId", doc.ObjectID))
					failCount++
					continue
				}
				successCount++
			case work_index.DeleteAction:
				// Deletes are shipped, not skipped. These events are the only
				// thing feeding the index and a deleted row emits no further
				// ones, so anything left behind stays searchable forever with
				// every hit landing on a 404 -- which is how ~2,860 merged-away
				// anime lingered in the anime index.
				if err := algoliaService.DeleteFromIndex(ctx, item.Data.Id); err != nil {
					log.Error("Failed to delete work from Algolia",
						zap.Error(err), zap.String("objectId", item.Data.Id))
					failCount++
					continue
				}
				successCount++
			default:
				log.Warn("Unknown action type",
					zap.String("action", string(item.Action)),
					zap.String("objectId", item.Data.Id))
				failCount++
			}
		}

		if _, err := algoliaService.Flush(ctx); err != nil {
			log.Error("Failed to flush works to Algolia", zap.Error(err))
			return err
		}

		log.Info("Works sync processing completed",
			zap.Int("successful", successCount),
			zap.Int("failed", failCount),
			zap.Int("total", len(queuedItems)))

		// Only clear on a clean run. GetAllData claims the batch by renaming the
		// key, so anything arriving meanwhile is already on a fresh key and is
		// not lost by this.
		if failCount == 0 {
			if err := redisService.ClearData(ctx); err != nil {
				log.Error("Failed to clear Redis works queue", zap.Error(err))
				return err
			}
			log.Info("Successfully cleared Redis works queue")
		} else {
			log.Warn("Not clearing Redis works queue due to failed syncs", zap.Int("failCount", failCount))
		}

		log.Info("Redis to Algolia works sync job completed successfully")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncRedisToAlgoliaWorkCmd)
}
