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
	"github.com/weeb-vip/algolia-sync/internal/services/work_index"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// EventingWorkNats consumes work records into the works search queue.
//
// NATS only; there is no Kafka twin. The Kafka handlers here exist because this
// service predates the move and production ran on Kafka while staging changed
// over. Nothing has ever carried works over Kafka, so a second entry point
// would be dead code for a transport already retired.
//
// It is deliberately the same shape as EventingAlgoliaNats rather than sharing
// code with it. What differs is only the record type, and the two are threaded
// through as generic parameters at every call, so a shared version would take a
// type parameter on every line and be harder to read than the copy.
//
// Which index it feeds is configuration, not code: the subject, the Redis key
// and the Algolia index name all come from the environment, so this runs as a
// second deployment of the same image pointed at the works index.
func EventingWorkNats() error {
	cfg := config.LoadConfigOrPanic()
	ctx := context.Background()
	log := logger.Get()
	ctx = logger.WithCtx(ctx, log)

	natsConfig := &epNats.Config{
		URL:               cfg.NatsConfig.URL,
		ConsumerGroupName: cfg.NatsConfig.ConsumerGroupName,
		// Empty StreamName: this subject is produced by anime-sync rather than
		// Debezium, so it is outside anime-db.> and the driver creates a stream
		// from the subject.
		StreamName:              cfg.NatsConfig.StreamName,
		ConsumerAutoOffsetReset: &cfg.NatsConfig.Offset,
	}

	driver := epNats.NewNatsDriver(natsConfig)
	defer func(d drivers.Driver[*epNats.Message]) {
		if err := d.Close(); err != nil {
			log.Error("Error closing NATS driver", zap.String("error", err.Error()))
		}
	}(driver)

	redisService := redis.NewRedisService[work_index.QueuedItem](ctx, cfg.RedisConfig)
	workProcessor := work_index.NewProcessor[*epNats.Message](redisService)

	retrySubject := cfg.NatsConfig.Subject + "-retry"
	dlqSubject := cfg.NatsConfig.Subject + "-dlq"

	// The retry consumer runs in this process rather than a second deployment.
	// It needs its own driver because the durable consumer name is driver-level
	// configuration, not per-subject: two Consume calls on one driver would
	// call CreateOrUpdateConsumer with the same durable name and different
	// filter subjects, and the second would reconfigure the first.
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

	processorInstance := processor.NewProcessor[*epNats.Message, work_index.Payload](driver, cfg.NatsConfig.Subject, workProcessor.Process).
		AddMiddleware(backoffretry.NewBackoffRetry[work_index.Payload](driver, backoffretry.Config{
			MaxRetries: maxRetries,
			HeaderKey:  retryHeaderKey,
			RetryQueue: retrySubject,
		}).Process)

	// Exhausted retries go to a dead-letter subject rather than back onto the
	// retry subject. ep acks and drops a message once the counter reaches
	// MaxRetries, so cycling it here would make a permanently failing record
	// disappear with no trace of it.
	retryProcessorInstance := processor.NewProcessor[*epNats.Message, work_index.Payload](retryDriver, retrySubject, workProcessor.Process).
		AddMiddleware(backoffretry.NewBackoffRetry[work_index.Payload](retryDriver, backoffretry.Config{
			MaxRetries: maxRetries,
			HeaderKey:  retryHeaderKey,
			RetryQueue: dlqSubject,
		}).Process)

	log.Info("Starting NATS work processors",
		zap.String("subject", cfg.NatsConfig.Subject),
		zap.String("retry_subject", retrySubject),
		zap.String("dlq_subject", dlqSubject))

	// One consumer returning must stop the other: Consume blocks until its
	// iterator is stopped, so without cancelling here a dead main consumer
	// would leave the process alive and apparently healthy.
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return processorInstance.Run(groupCtx) })
	group.Go(func() error { return retryProcessorInstance.Run(groupCtx) })

	if err := group.Wait(); err != nil && ctx.Err() == nil {
		log.Error("Error consuming messages", zap.String("error", err.Error()))
		return err
	}

	return nil
}
