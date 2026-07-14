package log_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/andreas-lindfalk/latentframe/pkg/log"
)

func TestContextRoundTrip(t *testing.T) {
	base, err := log.New(true)
	require.NoError(t, err)

	ctx := log.NewContext(context.Background(), base)
	require.Same(t, base, log.FromContext(ctx))
}

func TestFromContextDefaultsToNop(t *testing.T) {
	// No logger on the context → a usable no-op logger, never nil.
	got := log.FromContext(context.Background())
	require.NotNil(t, got)
	require.NotPanics(t, func() { got.Info("safe", zap.String("k", "v")) })
}
