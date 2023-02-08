package utility

import (
	"evsys/pusher"
	"fmt"
	"log"
)

var messageService = pusher.NewPusher()

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) FeatureEvent(feature, id, text string) {
	messageText := fmt.Sprintf("[%s] %s: %s", id, feature, text)
	log.Print(messageText)

	msg := pusher.Message{
		Channel: pusher.SystemLog,
		Event:   pusher.Call,
		Text:    messageText,
	}
	if err := messageService.Send(msg); err != nil {
		log.Printf("error pushing message; %s", err)
	}
}
