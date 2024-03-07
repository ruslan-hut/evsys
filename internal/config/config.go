package config

import (
	"fmt"
	"github.com/ilyakaznacheev/cleanenv"
	"sync"
)

type Config struct {
	IsDebug          bool   `yaml:"is_debug" env-default:"false"`
	TimeZone         string `yaml:"time_zone" env-default:"Europe/Madrid"`
	AcceptUnknownTag bool   `yaml:"accept_unknown_tag" env-default:"false"`
	AcceptUnknownChp bool   `yaml:"accept_unknown_chp" env-default:"false"`
	Listen           struct {
		Type     string `yaml:"type" env-default:"port"`
		BindIP   string `yaml:"bind_ip" env-default:"0.0.0.0"`
		Port     string `yaml:"port" env-default:"5000"`
		TLS      bool   `yaml:"tls_enabled" env-default:"false"`
		CertFile string `yaml:"cert_file" env-default:""`
		KeyFile  string `yaml:"key_file" env-default:""`
	}
	Api struct {
		BindIP   string `yaml:"bind_ip" env-default:"0.0.0.0"`
		Port     string `yaml:"port" env-default:"5001"`
		TLS      bool   `yaml:"tls_enabled" env-default:"false"`
		CertFile string `yaml:"cert_file" env-default:""`
		KeyFile  string `yaml:"key_file" env-default:""`
	}
	Metrics struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		BindIP  string `yaml:"bind_ip" env-default:"127.0.0.1"`
		Port    string `yaml:"port" env-default:"5003"`
	}
	Mongo struct {
		Enabled  bool   `yaml:"enabled" env-default:"false"`
		Host     string `yaml:"host" env-default:"127.0.0.1"`
		Port     string `yaml:"port" env-default:"27017"`
		User     string `yaml:"user" env-default:"admin"`
		Password string `yaml:"password" env-default:"pass"`
		Database string `yaml:"database" env-default:"evsys"`
	}
	Payment struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		ApiUrl  string `yaml:"api_url" env-default:""`
		ApiKey  string `yaml:"api_key" env-default:""`
	}
	Telegram struct {
		Enabled bool   `yaml:"enabled" env-default:"false"`
		ApiKey  string `yaml:"telegram_api_key" env-default:""`
	}
}

var instance *Config
var once sync.Once

func GetConfig(path *string) (*Config, error) {
	var err error
	once.Do(func() {
		instance = &Config{}
		if err = cleanenv.ReadConfig(*path, instance); err != nil {
			desc, _ := cleanenv.GetDescription(instance, nil)
			err = fmt.Errorf("%s; %s", err, desc)
			instance = nil
		}
	})
	return instance, err
}
