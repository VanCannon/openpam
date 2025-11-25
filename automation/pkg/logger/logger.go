package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

type Level string

const (
	DEBUG Level = "DEBUG"
	INFO  Level = "INFO"
	WARN  Level = "WARN"
	ERROR Level = "ERROR"
)

type Logger struct {
	level  Level
	format string
}

func New(level, format string) *Logger {
	return &Logger{
		level:  Level(level),
		format: format,
	}
}

func (l *Logger) log(level Level, message string, fields map[string]interface{}) {
	if !l.shouldLog(level) {
		return
	}

	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level,
		"message":   message,
		"service":   "license",
	}

	for k, v := range fields {
		entry[k] = v
	}

	if l.format == "json" {
		data, _ := json.Marshal(entry)
		log.Println(string(data))
	} else {
		fieldStr := ""
		for k, v := range fields {
			fieldStr += fmt.Sprintf(" %s=%v", k, v)
		}
		log.Printf("[%s] %s: %s%s\n", entry["timestamp"], level, message, fieldStr)
	}
}

func (l *Logger) shouldLog(level Level) bool {
	levels := map[Level]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
	}
	return levels[level] >= levels[l.level]
}

func (l *Logger) Debug(message string, fields map[string]interface{}) {
	l.log(DEBUG, message, fields)
}

func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.log(INFO, message, fields)
}

func (l *Logger) Warn(message string, fields map[string]interface{}) {
	l.log(WARN, message, fields)
}

func (l *Logger) Error(message string, fields map[string]interface{}) {
	l.log(ERROR, message, fields)
}

func (l *Logger) Fatal(message string, fields map[string]interface{}) {
	l.log(ERROR, message, fields)
	os.Exit(1)
}
