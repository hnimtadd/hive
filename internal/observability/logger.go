package observability

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/hnimtadd/hive/pkg/config"
)

var defaultLogger *slog.Logger

var defaultConfig = &config.TraceConfig{
	Enabled:   true,
	LogLevel:  "info",
	LogFormat: "json",
}

func Initialize(cfg *config.TraceConfig) {
	if cfg == nil {
		cfg = defaultConfig
	}
	// Initialize tracing
	if !cfg.Enabled {
		return
	}
	logOutput := os.Stdout
	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Fatalf("failed to open log file: %v", err)
		}
		defer f.Close()
		logOutput = f
	}

	opts := &slog.HandlerOptions{
		Level:     ParseLogLevel(cfg.LogLevel),
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(logOutput, opts)
	} else {
		handler = slog.NewTextHandler(logOutput, opts)
	}
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func Logger(ctx context.Context) *slog.Logger {
	if defaultLogger == nil {
		Initialize(nil)
	}

	logger := defaultLogger
	tc, ok := TraceContextFromContext(ctx)
	if ok {
		logger = logger.With(slog.String("trace_id", tc.TraceID))
	}
	return logger
}
