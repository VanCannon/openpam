package logger

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// Level represents the log level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the log level
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new logger instance
func New(level Level, out io.Writer) *Logger {
	if out == nil {
		out = os.Stdout
	}

	return &Logger{
		level:  level,
		logger: log.New(out, "", 0),
	}
}

// Default creates a default logger with INFO level
func Default() *Logger {
	return New(LevelInfo, os.Stdout)
}

// log writes a log message with the given level
func (l *Logger) log(level Level, msg string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format(time.RFC3339)
	logMsg := fmt.Sprintf("[%s] %s: %s", timestamp, level.String(), msg)

	if len(fields) > 0 {
		logMsg += " |"
		for k, v := range fields {
			logMsg += fmt.Sprintf(" %s=%v", k, v)
		}
	}

	l.logger.Println(logMsg)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	l.log(LevelDebug, msg, mergeFields(fields...))
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	l.log(LevelInfo, msg, mergeFields(fields...))
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	l.log(LevelWarn, msg, mergeFields(fields...))
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	l.log(LevelError, msg, mergeFields(fields...))
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *ContextLogger {
	return &ContextLogger{
		logger: l,
		fields: fields,
	}
}

// ContextLogger is a logger with pre-set fields
type ContextLogger struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with context fields
func (c *ContextLogger) Debug(msg string, fields ...map[string]interface{}) {
	c.logger.log(LevelDebug, msg, mergeFields(c.fields, mergeFields(fields...)))
}

// Info logs an info message with context fields
func (c *ContextLogger) Info(msg string, fields ...map[string]interface{}) {
	c.logger.log(LevelInfo, msg, mergeFields(c.fields, mergeFields(fields...)))
}

// Warn logs a warning message with context fields
func (c *ContextLogger) Warn(msg string, fields ...map[string]interface{}) {
	c.logger.log(LevelWarn, msg, mergeFields(c.fields, mergeFields(fields...)))
}

// Error logs an error message with context fields
func (c *ContextLogger) Error(msg string, fields ...map[string]interface{}) {
	c.logger.log(LevelError, msg, mergeFields(c.fields, mergeFields(fields...)))
}

// mergeFields merges multiple field maps into one
func mergeFields(fields ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, f := range fields {
		if f != nil {
			for k, v := range f {
				result[k] = v
			}
		}
	}
	return result
}

// contextKey is a custom type for context keys
type contextKey string

const loggerKey contextKey = "logger"

// WithContext adds a logger to the context
func WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves the logger from the context
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	return Default()
}
