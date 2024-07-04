package eventing

import (
	"context"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/weeb-vip/algolia-sync/config"
	"github.com/weeb-vip/algolia-sync/internal/logger"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia"
	"github.com/weeb-vip/algolia-sync/internal/services/algolia_processor"
	"github.com/weeb-vip/algolia-sync/internal/services/processor"
	"go.uber.org/zap"
	"time"
)

func EventingAlgolia() error {
	cfg := config.LoadConfigOrPanic()
	ctx := context.Background()
	log := logger.Get()
	ctx = logger.WithCtx(ctx, log)

	algoService := algolia.NewAlgoliaService[algolia_processor.Schema](ctx, cfg.AlgoliaConfig)

	imageProcessor := algolia_processor.NewImageProcessor(algoService)

	messageProcessor := processor.NewProcessor[algolia_processor.Payload]()

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
