package eventing

import (
	"context"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/processor"
	"github.com/weeb-vip/algolia-sync/internal/services/redis"
	"github.com/weeb-vip/algolia-sync/internal/services/redis_processor"
	"go.uber.org/zap"
	"time"
)

func EventingAlgolia() error {
	cfg := config.LoadConfigOrPanic()
	ctx := context.Background()
	log := logger.Get()
	ctx = logger.WithCtx(ctx, log)

	redisService := redis.NewRedisService[redis_processor.QueuedItem](ctx, cfg.RedisConfig)

	imageProcessor := redis_processor.NewImageProcessor(redisService)

	messageProcessor := processor.NewProcessor[redis_processor.Payload]()

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: cfg.PulsarConfig.URL,
	})

	if err != nil {
		log.Fatal("Error creating pulsar client: ", zap.String("error", err.Error()))
		return err
	}

	defer client.Close()

	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            cfg.PulsarConfig.Topic,
		SubscriptionName: cfg.PulsarConfig.SubscribtionName,
		Type:             pulsar.Shared,
	})

	defer consumer.Close()

	// create channel to receive messages

	for {
		msg, err := consumer.Receive(ctx)
		if err != nil {
			log.Fatal("Error receiving message: ", zap.String("error", err.Error()))
		}

		log.Info("Received message", zap.String("msgId", msg.ID().String()))

		err = messageProcessor.Process(ctx, string(msg.Payload()), imageProcessor.Process)
		if err != nil {
			log.Warn("error processing message: ", zap.String("error", err.Error()))
			continue
		}
		consumer.Ack(msg)
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}
