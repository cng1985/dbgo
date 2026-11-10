package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/cng1985/dbgo/internal/platform/buildinfo"
	"github.com/cng1985/dbgo/internal/platform/config"
)

type Logger struct {
	*slog.Logger
}

func New(cfg *config.Config) *Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.EqualFold(cfg.Logging.Format, "text") {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	logger := slog.New(handler).With(
		"service", cfg.App.Name,
		"version", buildinfo.Version,
	)
	slog.SetDefault(logger)
	return &Logger{Logger: logger}
}

func (l *Logger) Writer() io.Writer { return os.Stdout }
