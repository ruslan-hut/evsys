package utility

import (
	"evsys/pusher"
	"fmt"
	"log"
	"time"
)

var messageService = pusher.NewPusher()

type Logger struct {
}

func NewLogger() *Logger {
	return &Logger{}
}

func logTime(t time.Time) string {
	timeString := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeString
}

func (l *Logger) FeatureEvent(feature, id, text string) {
	messageText := fmt.Sprintf("[%s] %s: %s", id, feature, text)
	log.Print(messageText)

	logMessage := pusher.FeatureLogMessage{
		Time:          logTime(time.Now()),
		Feature:       feature,
		ChargePointId: id,
		Text:          text,
	}

	msg := pusher.Message{
		Channel: pusher.SystemLog,
		Event:   pusher.Call,
		Data:    logMessage,
	}
	if err := messageService.Send(msg); err != nil {
		log.Printf("error pushing message; %s", err)
	}
}
