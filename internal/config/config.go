package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"sync"
)

type Config struct {
	IsDebug *bool `yaml:"is_debug"`
	Listen  struct {
		Type     string `yaml:"type" env-default:"port"`
		BindIP   string `yaml:"bind_ip" env-default:"0.0.0.0"`
		Port     string `yaml:"port" env-default:"5000"`
		TLS      bool   `yaml:"tls_enabled" env-default:"false"`
		CertFile string `yaml:"cert_file" env-default:""`
		KeyFile  string `yaml:"key_file" env-default:""`
	}
	Pusher struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		AppID   string `yaml:"app_id" env-default:""`
		Key     string `yaml:"key" env-default:""`
		Secret  string `yaml:"secret" env-default:""`
		Cluster string `yaml:"cluster" env-default:"eu"`
	}
}

var instance *Config
var once sync.Once

func GetConfig() (*Config, error) {
	var err error
	once.Do(func() {
		log.Println("reading config")
		instance = &Config{}
		if err = cleanenv.ReadConfig("config.yml", instance); err != nil {
			desc, _ := cleanenv.GetDescription(instance, nil)
			log.Println(desc)
			log.Println(err)
			instance = nil
		}
	})
	return instance, err
}
