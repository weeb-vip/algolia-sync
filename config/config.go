package config

import (
	"github.com/jinzhu/configor"
)

type Config struct {
	AppConfig     AppConfig
	PulsarConfig  PulsarConfig
	AlgoliaConfig AlgoliaConfig
	KafkaConfig   KafkaConfig
	RedisConfig   RedisConfig
}

type AppConfig struct {
	APPName string `default:"anime-api"`
	Port    int    `env:"PORT" default:"3000"`
	Version string `default:"x.x.x"`
}

type PulsarConfig struct {
	URL              string `default:"pulsar://localhost:6650" env:"PULSARURL"`
	Topic            string `default:"public/default/myanimelist.public.anime" env:"PULSARTOPIC"`
	SubscribtionName string `default:"my-sub" env:"PULSARSUBSCRIPTIONNAME"`
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

func LoadConfigOrPanic() Config {
	var config = Config{}
	configor.Load(&config, "config/config.dev.json")

	return config
}
