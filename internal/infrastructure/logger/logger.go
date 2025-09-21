package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger implements application logger port using slog.
type Logger struct {
	base *slog.Logger
}

// New constructs slog-based logger with level derived from APP_ENV.
func New(env string) *Logger {
	level := slog.LevelInfo
	if strings.EqualFold(env, "development") {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return &Logger{base: slog.New(handler)}
}

// Info logs informational message.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.base.InfoContext(ctx, msg, args...)
}

// Error logs error message.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.base.ErrorContext(ctx, msg, args...)
}
