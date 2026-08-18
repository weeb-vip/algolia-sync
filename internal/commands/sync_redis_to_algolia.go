package commands

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"github.com/weeb-vip/algolia-sync/internal/services/redis_processor"
	"go.uber.org/zap"
)

// syncRedisToAlgoliaCmd represents the sync command that reads from Redis and sends to Algolia
var syncRedisToAlgoliaCmd = &cobra.Command{
	Use:   "sync-redis-to-algolia",
	Short: "Sync data from Redis to Algolia (for cron job usage)",
	Long: `This command reads all queued data from Redis, sends it to Algolia, 
and then clears the Redis queue. It's designed to be run as a cron job.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.LoadConfigOrPanic()
		ctx := context.Background()
		log := logger.Get()
		ctx = logger.WithCtx(ctx, log)

		log.Info("Starting Redis to Algolia sync job")

		// Initialize Redis service
		redisService := redis.NewRedisService[redis_processor.QueuedItem](ctx, cfg.RedisConfig)

		// Initialize Algolia service - using AlgoliaSchema for proper array fields
		algoliaService := algolia.NewAlgoliaServiceWithoutTimer[redis_processor.AnimeDocument](ctx, cfg.AlgoliaConfig)

		// Get all data from Redis
		queuedItems, err := redisService.GetAllData(ctx)
		if err != nil {
			log.Error("Failed to get data from Redis", zap.Error(err))
			return err
		}

		if len(queuedItems) == 0 {
			log.Info("No data to sync from Redis to Algolia")
			return nil
		}

		log.Info("Processing queued items", zap.Int("count", len(queuedItems)))

		// Process each item
		successCount := 0
		failCount := 0

		for _, item := range queuedItems {
			switch item.Action {
			case redis_processor.CreateAction, redis_processor.UpdateAction:
				doc := item.Data.ToDocument()
				_, err := algoliaService.AddToIndex(ctx, doc)
				if err != nil {
					log.Error("Failed to add item to Algolia",
						zap.Error(err),
						zap.String("action", string(item.Action)),
						zap.String("objectId", doc.ObjectID))
					failCount++
					continue
				}
				successCount++
			case redis_processor.DeleteAction:
				// Previously a TODO that logged a warning and moved on, which is
				// why the index accumulated ~2,860 records whose anime no longer
				// exists -- every one of them a search result leading to a 404.
				if err := algoliaService.DeleteFromIndex(ctx, item.Data.Id); err != nil {
					log.Error("Failed to delete item from Algolia",
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

		// Flush any remaining data to Algolia
		_, err = algoliaService.Flush(ctx)
		if err != nil {
			log.Error("Failed to flush data to Algolia", zap.Error(err))
			return err
		}

		log.Info("Sync processing completed", 
			zap.Int("successful", successCount),
			zap.Int("failed", failCount),
			zap.Int("total", len(queuedItems)))

		// Clear Redis data only if sync was successful
		if failCount == 0 {
			err = redisService.ClearData(ctx)
			if err != nil {
				log.Error("Failed to clear Redis data", zap.Error(err))
				return err
			}
			log.Info("Successfully cleared Redis queue")
		} else {
			log.Warn("Not clearing Redis queue due to failed syncs", zap.Int("failCount", failCount))
		}

		log.Info("Redis to Algolia sync job completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(syncRedisToAlgoliaCmd)
}