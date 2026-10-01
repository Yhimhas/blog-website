package provider

import (
	"context"
	"log/slog"
)

type syncLoggerKey struct{}

// WithSyncLogger carries safe request/source identifiers into adapter diagnostics.
func WithSyncLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, syncLoggerKey{}, logger)
}

func SyncLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(syncLoggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
