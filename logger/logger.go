package logger

import (
	"evsys/internal"
	"fmt"
	"log"
	"time"
)

type Logger struct {
	messageService internal.MessageService
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) SetMessageService(messageService internal.MessageService) {
	l.messageService = messageService
}

func logTime(t time.Time) string {
	timeString := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeString
}

func (l *Logger) FeatureEvent(feature, id, text string) {
	messageText := fmt.Sprintf("[%s] %s: %s", id, feature, text)
	log.Print(messageText)

	if l.messageService != nil {
		logMessage := &FeatureLogMessage{
			Time:          logTime(time.Now()),
			Feature:       feature,
			ChargePointId: id,
			Text:          text,
		}
		if err := l.messageService.Send(logMessage); err != nil {
			log.Printf("error sending message; %s", err)
		}
	}
}
