package pusher

import (
	"evsys/internal"
	"evsys/logger"
	"github.com/pusher/pusher-http-go/v5"
)

type MessagePusher struct {
	client pusher.Client
}

func NewPusher() *MessagePusher {
	client := pusher.Client{
		AppID:   "1551169",
		Key:     "a1f101fb40a32c47c791",
		Secret:  "d2a4f3029920cd9265aa",
		Cluster: "eu",
		Secure:  true,
	}
	messagePusher := MessagePusher{
		client: client,
	}
	return &messagePusher
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
