package log

import (
	"context"
	"log/slog"
	"os"

	"github.com/mazrean/isucon-go-tools/v2/internal/config"
)

func init() {
	var logger *slog.Logger
	if config.Enable {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	} else {
		logger = slog.New(DiscardHandler{})
	}

	slog.SetDefault(logger)
}

type DiscardHandler struct{}

func (h DiscardHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

func (h DiscardHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

func (h DiscardHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h DiscardHandler) WithGroup(_ string) slog.Handler {
	return h
}
