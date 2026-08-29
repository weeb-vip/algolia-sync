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
	"golang.org/x/sync/errgroup"
)

const (
	maxRetries     = 3
	retryHeaderKey = "retry"
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

	retrySubject := cfg.NatsConfig.Subject + "-retry"
	dlqSubject := cfg.NatsConfig.Subject + "-dlq"

	// The retry consumer runs in this same process rather than a second
	// deployment. It needs its own driver because the durable consumer name is
	// driver-level configuration, not per-subject: two Consume calls on one
	// driver would call CreateOrUpdateConsumer with the same durable name and
	// different filter subjects, and the second would reconfigure the first.
	retryDriver := epNats.NewNatsDriver(&epNats.Config{
		URL:                     cfg.NatsConfig.URL,
		ConsumerGroupName:       cfg.NatsConfig.ConsumerGroupName + "-retry",
		StreamName:              cfg.NatsConfig.StreamName,
		ConsumerAutoOffsetReset: &cfg.NatsConfig.Offset,
	})
	defer func(d drivers.Driver[*epNats.Message]) {
		if err := d.Close(); err != nil {
			log.Error("Error closing NATS retry driver", zap.String("error", err.Error()))
		}
	}(retryDriver)

	processorInstance := processor.NewProcessor[*epNats.Message, redis_event_processor.Payload](driver, cfg.NatsConfig.Subject, redisProcessor.Process).
		AddMiddleware(backoffretry.NewBackoffRetry[redis_event_processor.Payload](driver, backoffretry.Config{
			MaxRetries: maxRetries,
			HeaderKey:  retryHeaderKey,
			RetryQueue: retrySubject,
		}).Process)

	// Exhausted retries go to a dead-letter subject rather than back onto the
	// retry subject. ep acks and drops a message once the counter reaches
	// MaxRetries, so cycling it here would make a permanently failing message
	// disappear with no record.
	retryProcessorInstance := processor.NewProcessor[*epNats.Message, redis_event_processor.Payload](retryDriver, retrySubject, redisProcessor.Process).
		AddMiddleware(backoffretry.NewBackoffRetry[redis_event_processor.Payload](retryDriver, backoffretry.Config{
			MaxRetries: maxRetries,
			HeaderKey:  retryHeaderKey,
			RetryQueue: dlqSubject,
		}).Process)

	log.Info("Starting NATS processors",
		zap.String("subject", cfg.NatsConfig.Subject),
		zap.String("retry_subject", retrySubject),
		zap.String("dlq_subject", dlqSubject))

	// One consumer returning must stop the other: Consume blocks until its
	// iterator is stopped, so without cancelling here a dead main consumer
	// would leave the process alive and apparently healthy.
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return processorInstance.Run(groupCtx) })
	group.Go(func() error { return retryProcessorInstance.Run(groupCtx) })

	if err := group.Wait(); err != nil && ctx.Err() == nil { // Ignore error if caused by context cancellation
		log.Error("Error consuming messages", zap.String("error", err.Error()))
		return err
	}

	return nil
}
