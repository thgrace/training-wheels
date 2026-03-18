package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	once sync.Once
	mu   sync.RWMutex
	l    *slog.Logger
)

// Default returns the global logger instance.
func Default() *slog.Logger {
	once.Do(func() {
		if l == nil {
			l = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}))
		}
	})
	mu.RLock()
	defer mu.RUnlock()
	return l
}

// SetDefault replaces the global logger.
func SetDefault(newLogger *slog.Logger) {
	mu.Lock()
	defer mu.Unlock()
	l = newLogger
}

// Format identifies the output format for the logger.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Configure global logger with the given level, format, and output destination.
func Configure(w io.Writer, level slog.Level, format Format) {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	switch format {
	case FormatJSON:
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}

	SetDefault(slog.New(handler))
}

// SetOutput replaces the global logger with a default text handler at Info level
// writing to the given destination. This is useful for tests or temporary redirection.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	l = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Global convenience wrappers.

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	Default().Log(ctx, level, msg, args...)
}
