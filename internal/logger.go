package internal

import (
	"fmt"
	"log"
	"time"
)

type Importance string

const (
	Info    Importance = " "
	Warning Importance = "?"
	Error   Importance = "!"
	Raw     Importance = "-"
)

type Logger struct {
	messageService MessageService
	database       Database
	debugMode      bool
	writer         chan *Event
}

type Event struct {
	Importance Importance
	Message    *FeatureLogMessage
}

func NewLogger() *Logger {
	logger := &Logger{
		debugMode: false,
		writer:    make(chan *Event, 100),
	}
	logger.StartWriter()
	return logger
}

func (l *Logger) StartWriter() {
	go func() {
		for {
			event := <-l.writer

			message := event.Message
			messageText := fmt.Sprintf("[%s] %s: %s", message.ChargePointId, message.Feature, message.Text)
			l.logLine(event.Importance, messageText)

			if l.messageService != nil {
				if err := l.messageService.Send(message); err != nil {
					l.logLine(Error, fmt.Sprintln("error sending message:", err))
				}
			}

			if l.database != nil {
				if err := l.database.WriteLogMessage(message); err != nil {
					l.logLine(Error, fmt.Sprintln("write log to database failed:", err))
				}
			}
		}
	}()
}

func (l *Logger) SetDebugMode(debugMode bool) {
	l.debugMode = debugMode
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
	l.logEvent(Info, NewFeatureLogMessage(feature, id, text))
}

func (l *Logger) logEvent(importance Importance, message *FeatureLogMessage) {
	if message.ChargePointId == "" {
		message.ChargePointId = "*"
	}
	message.Importance = string(importance)
	event := &Event{
		Importance: importance,
		Message:    message,
	}
	l.writer <- event
}

func (l *Logger) Debug(text string) {
	l.logEvent(Info, NewFeatureLogMessage("info", "", text))
}

func (l *Logger) Warn(text string) {
	l.logEvent(Warning, NewFeatureLogMessage("warning", "", text))
}

func (l *Logger) Error(text string, err error) {
	l.logEvent(Error, NewFeatureLogMessage("error", "", fmt.Sprintf("%s: %s", text, err)))
}

func (l *Logger) RawDataEvent(direction, data string) {
	if l.debugMode {
		l.logEvent(Raw, NewFeatureLogMessage("raw", "", fmt.Sprintf("%s: %s", direction, data)))
	}
}

func (l *Logger) logLine(importance Importance, text string) {
	if importance == Raw {
		return
	}
	if importance == Info && !l.debugMode && l.database != nil {
		return
	}
	log.Printf("%s %s", importance, text)
}
