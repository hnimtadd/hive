package trace

import (
	"context"
	"io"
	"log/slog"
	"os"
)

var (
	defaultLogger *slog.Logger
)

type LogConfig struct {
	Level     slog.Level
	Format    string
	Output    io.Writer
	AddSource bool
}

func InitLogger(cfg *LogConfig) {
	if cfg == nil {
		cfg = &LogConfig{
			Level:  slog.LevelInfo,
			Format: "json",
			Output: os.Stdout,
		}
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	} else {
		handler = slog.NewTextHandler(cfg.Output, opts)
	}
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func Logger(ctx context.Context) *slog.Logger {
	if defaultLogger == nil {
		InitLogger(nil)
	}

	logger := defaultLogger
	tc, ok := TraceFromContext(ctx)
	if ok {
		logger = logger.With(slog.String("trace_id", string(tc.TraceID)))
	}
	return logger
}
