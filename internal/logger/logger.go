// Package logger provides structured logging for HookTM.
// It supports multiple output formats (JSON, text) and log levels.
package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents a log level.
type Level int

const (
	// DebugLevel is for detailed debugging information.
	DebugLevel Level = iota
	// InfoLevel is for general operational information.
	InfoLevel
	// WarnLevel is for warning messages.
	WarnLevel
	// ErrorLevel is for error messages.
	ErrorLevel
)

// String returns the string representation of a log level.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a log level string.
func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DebugLevel, nil
	case "INFO":
		return InfoLevel, nil
	case "WARN", "WARNING":
		return WarnLevel, nil
	case "ERROR":
		return ErrorLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", s)
	}
}

// Fields represents structured log fields.
type Fields map[string]interface{}

// Logger is the interface for structured logging.
type Logger interface {
	// WithField adds a field to the logger and returns a new logger.
	WithField(key string, value interface{}) Logger
	
	// WithFields adds multiple fields to the logger and returns a new logger.
	WithFields(fields Fields) Logger
	
	// WithContext extracts correlation ID from context and returns a new logger.
	WithContext(ctx context.Context) Logger
	
	// Debug logs a debug message.
	Debug(msg string)
	
	// Debugf logs a formatted debug message.
	Debugf(format string, args ...interface{})
	
	// Info logs an info message.
	Info(msg string)
	
	// Infof logs a formatted info message.
	Infof(format string, args ...interface{})
	
	// Warn logs a warning message.
	Warn(msg string)
	
	// Warnf logs a formatted warning message.
	Warnf(format string, args ...interface{})
	
	// Error logs an error message.
	Error(msg string)
	
	// Errorf logs a formatted error message.
	Errorf(format string, args ...interface{})
}

// Config holds logger configuration.
type Config struct {
	// Level is the minimum log level to output.
	Level Level
	
	// Format is the output format: "json" or "text".
	Format string
	
	// Output is the writer to log to. Defaults to os.Stderr.
	Output io.Writer
}

// New creates a new Logger with the given configuration.
func New(cfg Config) Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}
	
	if cfg.Format == "" {
		cfg.Format = "text"
	}
	
	base := &baseLogger{
		level:  cfg.Level,
		output: cfg.Output,
		fields: make(Fields),
	}
	
	switch strings.ToLower(cfg.Format) {
	case "json":
		return &jsonLogger{baseLogger: base}
	default:
		return &textLogger{baseLogger: base}
	}
}

// NewDefault creates a new Logger with sensible defaults.
func NewDefault() Logger {
	return New(Config{
		Level:  InfoLevel,
		Format: "text",
		Output: os.Stderr,
	})
}

// baseLogger holds common logger state.
type baseLogger struct {
	level  Level
	output io.Writer
	fields Fields
	mu     sync.RWMutex
}

func (l *baseLogger) clone() *baseLogger {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	newFields := make(Fields, len(l.fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	
	return &baseLogger{
		level:  l.level,
		output: l.output,
		fields: newFields,
	}
}

func (l *baseLogger) shouldLog(level Level) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return level >= l.level
}

// contextKey is the type for context keys.
type contextKey string

const correlationIDKey contextKey = "correlation_id"

// WithCorrelationID adds a correlation ID to the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationID extracts the correlation ID from the context.
func CorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// textLogger implements text format logging.
type textLogger struct {
	*baseLogger
}

func (l *textLogger) WithField(key string, value interface{}) Logger {
	newLogger := l.clone()
	newLogger.fields[key] = value
	return &textLogger{baseLogger: newLogger}
}

func (l *textLogger) WithFields(fields Fields) Logger {
	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &textLogger{baseLogger: newLogger}
}

func (l *textLogger) WithContext(ctx context.Context) Logger {
	if id := CorrelationID(ctx); id != "" {
		return l.WithField("correlation_id", id)
	}
	return l
}

func (l *textLogger) log(level Level, msg string) {
	if !l.shouldLog(level) {
		return
	}
	
	l.mu.RLock()
	fields := make(Fields, len(l.fields))
	for k, v := range l.fields {
		fields[k] = v
	}
	output := l.output
	l.mu.RUnlock()
	
	// Build log line
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s [%s] %s", timestamp, level.String(), msg))
	
	// Add fields
	for k, v := range fields {
		sb.WriteString(fmt.Sprintf(" %s=%v", k, v))
	}
	
	// Use standard log for thread-safe output
	log.New(output, "", 0).Println(sb.String())
}

func (l *textLogger) Debug(msg string)  { l.log(DebugLevel, msg) }
func (l *textLogger) Info(msg string)   { l.log(InfoLevel, msg) }
func (l *textLogger) Warn(msg string)   { l.log(WarnLevel, msg) }
func (l *textLogger) Error(msg string)  { l.log(ErrorLevel, msg) }

func (l *textLogger) Debugf(format string, args ...interface{}) { l.Debug(fmt.Sprintf(format, args...)) }
func (l *textLogger) Infof(format string, args ...interface{})  { l.Info(fmt.Sprintf(format, args...)) }
func (l *textLogger) Warnf(format string, args ...interface{})  { l.Warn(fmt.Sprintf(format, args...)) }
func (l *textLogger) Errorf(format string, args ...interface{}) { l.Error(fmt.Sprintf(format, args...)) }

// jsonLogger implements JSON format logging.
type jsonLogger struct {
	*baseLogger
}

type jsonLogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func (l *jsonLogger) WithField(key string, value interface{}) Logger {
	newLogger := l.clone()
	newLogger.fields[key] = value
	return &jsonLogger{baseLogger: newLogger}
}

func (l *jsonLogger) WithFields(fields Fields) Logger {
	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &jsonLogger{baseLogger: newLogger}
}

func (l *jsonLogger) WithContext(ctx context.Context) Logger {
	if id := CorrelationID(ctx); id != "" {
		return l.WithField("correlation_id", id)
	}
	return l
}

func (l *jsonLogger) log(level Level, msg string) {
	if !l.shouldLog(level) {
		return
	}
	
	l.mu.RLock()
	fields := make(Fields, len(l.fields))
	for k, v := range l.fields {
		fields[k] = v
	}
	output := l.output
	l.mu.RUnlock()
	
	entry := jsonLogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Fields:    fields,
	}
	
	if len(fields) == 0 {
		entry.Fields = nil
	}
	
	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple error message
		fmt.Fprintf(output, `{"timestamp":"%s","level":"ERROR","message":"failed to marshal log entry: %s"}`+"\n",
			time.Now().Format(time.RFC3339Nano), err)
		return
	}
	
	output.Write(data)
	output.Write([]byte("\n"))
}

func (l *jsonLogger) Debug(msg string)  { l.log(DebugLevel, msg) }
func (l *jsonLogger) Info(msg string)   { l.log(InfoLevel, msg) }
func (l *jsonLogger) Warn(msg string)   { l.log(WarnLevel, msg) }
func (l *jsonLogger) Error(msg string)  { l.log(ErrorLevel, msg) }

func (l *jsonLogger) Debugf(format string, args ...interface{}) { l.Debug(fmt.Sprintf(format, args...)) }
func (l *jsonLogger) Infof(format string, args ...interface{})  { l.Info(fmt.Sprintf(format, args...)) }
func (l *jsonLogger) Warnf(format string, args ...interface{})  { l.Warn(fmt.Sprintf(format, args...)) }
func (l *jsonLogger) Errorf(format string, args ...interface{}) { l.Error(fmt.Sprintf(format, args...)) }

// NopLogger is a logger that discards all output.
// Useful for testing.
type NopLogger struct{}

func (NopLogger) WithField(key string, value interface{}) Logger   { return NopLogger{} }
func (NopLogger) WithFields(fields Fields) Logger                  { return NopLogger{} }
func (NopLogger) WithContext(ctx context.Context) Logger           { return NopLogger{} }
func (NopLogger) Debug(msg string)                                 {}
func (NopLogger) Debugf(format string, args ...interface{})        {}
func (NopLogger) Info(msg string)                                  {}
func (NopLogger) Infof(format string, args ...interface{})         {}
func (NopLogger) Warn(msg string)                                  {}
func (NopLogger) Warnf(format string, args ...interface{})         {}
func (NopLogger) Error(msg string)                                 {}
func (NopLogger) Errorf(format string, args ...interface{})        {}
