package logging

import (
	"context"
	"log/slog"
	"os"
)

// Logger provides structured logging interface
type Logger struct {
	handler slog.Handler
	logger  *slog.Logger
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value any
}

// NewJSONLogger creates a JSON-formatted logger with the specified level
func NewJSONLogger(level string) *Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	return &Logger{
		handler: handler,
		logger:  slog.New(handler),
	}
}

// NewTextLogger creates a text-formatted logger with the specified level
func NewTextLogger(level string) *Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})

	return &Logger{
		handler: handler,
		logger:  slog.New(handler),
	}
}

// NewStderrTextLogger creates a text-formatted logger that writes to
// stderr. REQUIRED for any process whose stdout is a protocol channel —
// the MCP stdio server (cmd/df-mcp) speaks JSON-RPC on stdout, so a
// single stdout log line corrupts the stream.
func NewStderrTextLogger(level string) *Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})

	return &Logger{
		handler: handler,
		logger:  slog.New(handler),
	}
}

// Debug logs a debug-level message with structured fields
func (l *Logger) Debug(msg string, fields ...Field) {
	attrs := fieldsToAttrs(fields)
	l.logger.LogAttrs(context.Background(), slog.LevelDebug, msg, attrs...)
}

// Info logs an info-level message with structured fields
func (l *Logger) Info(msg string, fields ...Field) {
	attrs := fieldsToAttrs(fields)
	l.logger.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// Warn logs a warning-level message with structured fields
func (l *Logger) Warn(msg string, fields ...Field) {
	attrs := fieldsToAttrs(fields)
	l.logger.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
}

// Error logs an error-level message with structured fields
func (l *Logger) Error(msg string, err error, fields ...Field) {
	attrs := fieldsToAttrs(fields)
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	l.logger.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

// With returns a new logger with additional fields
func (l *Logger) With(fields ...Field) *Logger {
	attrs := fieldsToAttrs(fields)
	// Convert []slog.Attr to []any for logger.With()
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return &Logger{
		handler: l.handler,
		logger:  l.logger.With(args...),
	}
}

// fieldsToAttrs converts Field slice to slog.Attr slice
func fieldsToAttrs(fields []Field) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(fields))
	for _, f := range fields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}
	return attrs
}
