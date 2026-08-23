package eventing

import (
	"context"
	"github.com/ThatCatDev/ep/v2/drivers"
	epNats "github.com/ThatCatDev/ep/v2/drivers/nats"
	"github.com/ThatCatDev/ep/v2/middlewares/nats/backoffretry"
	"github.com/ThatCatDev/ep/v2/processor"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"github.com/weeb-vip/algolia-sync/internal/services/redis_event_processor"
	"go.uber.org/zap"
)

// EventingAlgoliaNats is the NATS counterpart of EventingAlgoliaKafka.
//
// A separate entry point rather than a flag, so production keeps running the
// Kafka command untouched while staging moves over.
func EventingAlgoliaNats() error {
	cfg := config.LoadConfigOrPanic()
	ctx := context.Background()
	log := logger.Get()
	ctx = logger.WithCtx(ctx, log)

	debug := &cfg.KafkaConfig.Debug
	if *debug == "" {
		debug = nil
	}
	natsConfig := &epNats.Config{
		URL:               cfg.NatsConfig.URL,
		ConsumerGroupName: cfg.NatsConfig.ConsumerGroupName,
		// Empty StreamName: algolia-sync is produced by anime-sync, not
		// Debezium, so the driver creates a stream from the subject.
		StreamName:              cfg.NatsConfig.StreamName,
		ConsumerAutoOffsetReset: &cfg.NatsConfig.Offset,
	}

	log.Info("Creating NATS driver", zap.String("bootstrapServers", cfg.KafkaConfig.BootstrapServers))
	driver := epNats.NewNatsDriver(natsConfig)
	defer func(driver drivers.Driver[*epNats.Message]) {
		err := driver.Close()
		if err != nil {
			log.Error("Error closing NATS driver", zap.String("error", err.Error()))
		} else {
			log.Info("NATS driver closed successfully")
		}
	}(driver)

	log.Info("Creating processor for Kafka messages", zap.String("subject", cfg.NatsConfig.Subject))

	redisService := redis.NewRedisService[redis_event_processor.QueuedItem](ctx, cfg.RedisConfig)

	redisProcessor := redis_event_processor.NewRedisProcessor[*epNats.Message](redisService)

	processorInstance := processor.NewProcessor[*epNats.Message, redis_event_processor.Payload](driver, cfg.NatsConfig.Subject, redisProcessor.Process)

	log.Info("initializing backoff retry middleware", zap.String("subject", cfg.NatsConfig.Subject))
	backoffRetryInstance := backoffretry.NewBackoffRetry[redis_event_processor.Payload](driver, backoffretry.Config{
		MaxRetries: 3,
		HeaderKey:  "retry",
		RetryQueue: cfg.NatsConfig.Subject + "-retry",
	})

	log.Info("Starting NATS processor", zap.String("subject", cfg.NatsConfig.Subject))
	err := processorInstance.
		AddMiddleware(backoffRetryInstance.Process).
		Run(ctx)

	if err != nil && ctx.Err() == nil { // Ignore error if caused by context cancellation
		log.Error("Error consuming messages", zap.String("error", err.Error()))
		return err
	}

	return nil
}
