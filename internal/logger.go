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
	database  Database
	location  *time.Location
	debugMode bool
	writer    chan *LogEvent
}

type LogEvent struct {
	Importance Importance
	Message    *FeatureLogMessage
}

func NewLogger(location *time.Location) *Logger {
	logger := &Logger{
		debugMode: false,
		location:  location,
		writer:    make(chan *LogEvent, 100),
	}
	go logger.startWriter()
	return logger
}

func (l *Logger) startWriter() {
	for {
		event := <-l.writer

		message := event.Message
		messageText := fmt.Sprintf("[%s] %s: %s", message.ChargePointId, message.Feature, message.Text)
		l.logLine(event.Importance, messageText)

		if l.database != nil {
			if err := l.database.WriteLogMessage(message); err != nil {
				l.logLine(Error, fmt.Sprintln("write log to database failed:", err))
			}
		}
	}
}

func (l *Logger) SetDebugMode(debugMode bool) {
	l.debugMode = debugMode
}

func (l *Logger) SetDatabase(database Database) {
	l.database = database
}

func logTime(t time.Time) string {
	timeString := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return timeString
}

func (l *Logger) FeatureEvent(feature, id, text string) {
	l.logEvent(Info, l.newFeatureLogMessage(feature, id, text))
}

func (l *Logger) logEvent(importance Importance, message *FeatureLogMessage) {
	if message.ChargePointId == "" {
		message.ChargePointId = "*"
	}
	message.Importance = string(importance)
	event := &LogEvent{
		Importance: importance,
		Message:    message,
	}
	l.writer <- event
}

func (l *Logger) Debug(text string) {
	l.logEvent(Info, l.newFeatureLogMessage("info", "", text))
}

func (l *Logger) Warn(text string) {
	l.logEvent(Warning, l.newFeatureLogMessage("warning", "", text))
}

func (l *Logger) Error(text string, err error) {
	l.logEvent(Error, l.newFeatureLogMessage("error", "", fmt.Sprintf("%s: %s", text, err)))
}

func (l *Logger) RawDataEvent(direction, data string) {
	if l.debugMode {
		l.logEvent(Raw, l.newFeatureLogMessage("raw", "", fmt.Sprintf("%s: %s", direction, data)))
	}
}

func (l *Logger) logLine(importance Importance, text string) {
	if importance == Info && l.database != nil {
		return
	}
	log.Printf("%s %s", importance, text)
}

func (l *Logger) newFeatureLogMessage(feature, id, text string) *FeatureLogMessage {
	return &FeatureLogMessage{
		Time:          logTime(time.Now().In(l.location)),
		TimeStamp:     time.Now().UTC(),
		Text:          text,
		Feature:       feature,
		ChargePointId: id,
	}
}
