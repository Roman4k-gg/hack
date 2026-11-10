package logging

import (
	"log/slog"
	"os"
	"strings"
)

func New(level, service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       ParseLevel(level),
		ReplaceAttr: replaceAttr,
	})
	return slog.New(handler).With("service", service)
}

func ParseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 {
		switch a.Key {
		case slog.TimeKey:
			a.Key = "ts"
		case slog.SourceKey:
			return slog.Attr{}
		}
	}
	return a
}
