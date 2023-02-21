package internal

import (
	"fmt"
	"log"
	"time"
)

type Logger struct {
	messageService MessageService
	database       Database
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) SetMessageService(messageService MessageService) {
	l.messageService = messageService
}

func (l *Logger) SetDatabase(database Database) {
	l.database = database
}

func logTime(t time.Time) string {
	timeString := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeString
}

func (l *Logger) FeatureEvent(feature, id, text string) {
	messageText := fmt.Sprintf("[%s] %s: %s", id, feature, text)
	l.Debug(messageText)

	logMessage := &FeatureLogMessage{
		Time:          logTime(time.Now()),
		Feature:       feature,
		ChargePointId: id,
		Text:          text,
	}

	if l.messageService != nil {
		if err := l.messageService.Send(logMessage); err != nil {
			l.Error("error sending message;", err)
		}
	}

	if l.database != nil {
		if err := l.database.WriteLogMessage(logMessage); err != nil {
			l.Error("write log to database failed;", err)
		}
	}
}

func (l *Logger) Debug(text string) {
	logLine("", text)
}

func (l *Logger) Error(text string, err error) {
	logLine("!", fmt.Sprintln(text, ":", err))
}

func logLine(flag, text string) {
	if flag == "" {
		flag = " "
	}
	log.Printf("%s %s", flag, text)
}
