package config

import (
	"github.com/jinzhu/configor"
)

type Config struct {
	AppConfig     AppConfig
	PulsarConfig  PulsarConfig
	AlgoliaConfig AlgoliaConfig
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

func LoadConfigOrPanic() Config {
	var config = Config{}
	configor.Load(&config, "config/config.dev.json")

	return config
}
