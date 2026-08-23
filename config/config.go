package config

import (
	"github.com/jinzhu/configor"
)

type Config struct {
	AppConfig     AppConfig
	SourceConfig  SourceConfig
	AlgoliaConfig AlgoliaConfig
	KafkaConfig   KafkaConfig
	NatsConfig    NatsConfig
	RedisConfig   RedisConfig
}

// SourceConfig points at the system of record. Reconcile needs to know which
// anime exist; algolia-sync has no database of its own, and the gateway already
// exposes the whole catalogue.
type SourceConfig struct {
	GraphQLHost string `default:"http://anime-api-internal/graphql" env:"GRAPHQL_HOST"`
}

type AppConfig struct {
	APPName string `default:"algolia-sync" env:"APP_NAME"`
	Port    int    `env:"PORT" default:"3000"`
	Version string `default:"x.x.x"`
}

type AlgoliaConfig struct {
	AppID        string `default:"" env:"ALGOLIA_APP_ID"`
	APIKey       string `default:"" env:"ALGOLIA_API_KEY"`
	Index        string `default:"" env:"ALGOLIA_INDEX"`
	FlushTimeout int    `default:"10" env:"ALGOLIA_FLUSH_TIMEOUT"`
}

type KafkaConfig struct {
	ConsumerGroupName string `default:"image-sync-group" env:"KAFKA_CONSUMER_GROUP_NAME"`
	BootstrapServers  string `default:"localhost:9092" env:"KAFKA_BOOTSTRAP_SERVERS"`
	Topic             string `default:"algolia-sync" env:"KAFKA_TOPIC"`
	Offset            string `default:"earliest" env:"KAFKA_OFFSET"`
	Debug             string `default:"" env:"KAFKA_DEBUG"`
}

type RedisConfig struct {
	URL      string `default:"redis://localhost:6379" env:"REDIS_URL"`
	Password string `default:"" env:"REDIS_PASSWORD"`
	DB       int    `default:"0" env:"REDIS_DB"`
	Key      string `default:"algolia-sync:data" env:"REDIS_KEY"`
}

// NatsConfig mirrors KafkaConfig, so moving between the two is one substitution
// per setting.
type NatsConfig struct {
	URL string `default:"nats://localhost:4222" env:"NATSURL"`

	// The durable consumer name -- the closest equivalent to a Kafka consumer
	// group. Left empty the consumer is ephemeral and loses its position on
	// restart.
	ConsumerGroupName string `default:"algolia-sync-nats" env:"NATSCONSUMERGROUPNAME"`

	// Empty on purpose, unlike the CDC consumers: algolia-sync is produced by
	// anime-sync rather than Debezium, so no other stream declares it and the
	// driver should create one from the subject.
	StreamName string `env:"NATSSTREAMNAME"`

	Offset string `default:"earliest" env:"NATSOFFSET"`

	Subject string `default:"algolia-sync" env:"NATSSUBJECT"`
}

func LoadConfigOrPanic() Config {
	var config = Config{}
	configor.Load(&config, "config/config.dev.json")

	return config
}
