// Package log is Latent Frame's structured logger (zap), carried on context —
// mirroring the shape of the cloud repo's pkg/log (FromContext / NewContext), but
// deliberately minimal: the heavier concerns (fx wiring, Prometheus metrics, sampling,
// dynamic log levels) are deferred until a long-running service actually needs them.
//
// CLI one-shot tools can keep using the stdlib log for human-readable output; use this
// for services, workers, and pipeline code where structured (JSON) logs matter.
package log

import (
	"context"

	"go.uber.org/zap"
)

// New returns a logger: JSON production logger by default, or a human-readable
// development console logger when dev is true.
func New(dev bool) (*zap.Logger, error) {
	if dev {
		return zap.NewDevelopment()
	}
	return zap.NewProduction()
}

// Must is New but panics on error — convenient for main() setup.
func Must(dev bool) *zap.Logger {
	l, err := New(dev)
	if err != nil {
		panic(err)
	}
	return l
}

type ctxKey struct{}

// NewContext returns ctx carrying logger, so it propagates through a request/job.
func NewContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the logger on ctx, or a no-op logger if none is set — so call
// sites never need a nil check.
func FromContext(ctx context.Context) *zap.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*zap.Logger); ok && l != nil {
		return l
	}
	return zap.NewNop()
}
