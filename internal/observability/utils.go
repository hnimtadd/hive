package observability

import (
	"log/slog"
)

// ParseLogLevel - Convert string to slog.Level.
func ParseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// DebugArgs creates slog attr that only appears in debug mode
// If the current global level is higher than DEBUG, return an empty Attr.
// slog will discard empty attributes completely.
func DebugArgs(key string, value any) slog.Attr {
	if loggerLevel.Level() > slog.LevelDebug {
		return slog.Attr{}
	}
	return slog.Any(key, value)
}
