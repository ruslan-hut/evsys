package pusher

import (
	"evsys/internal"
	"evsys/internal/config"
	"evsys/logger"
	"evsys/utility"
	"github.com/pusher/pusher-http-go/v5"
)

type MessagePusher struct {
	client pusher.Client
}

func NewPusher(conf *config.Config) (*MessagePusher, error) {
	if !conf.Pusher.Enabled {
		return nil, nil
	}
	if conf.Pusher.AppID == "" {
		return nil, utility.Err("missed AppID parameter in Pusher configuration")
	}
	if conf.Pusher.Key == "" {
		return nil, utility.Err("missed Key parameter in Pusher configuration")
	}
	if conf.Pusher.Secret == "" {
		return nil, utility.Err("missed Secret parameter in Pusher configuration")
	}
	client := pusher.Client{
		AppID:   conf.Pusher.AppID,
		Key:     conf.Pusher.Key,
		Secret:  conf.Pusher.Secret,
		Cluster: conf.Pusher.Cluster,
		Secure:  true,
	}
	messagePusher := MessagePusher{
		client: client,
	}
	return &messagePusher, nil
}

func (p *MessagePusher) Send(msg internal.Message) error {
	messageType := msg.MessageType()
	switch messageType {
	case logger.FeatureLogMessageType:
		//payload := msg.(*utility.FeatureLogMessage)
		return p.client.Trigger(string(SystemLog), string(Call), msg)
	}
	return nil
}
