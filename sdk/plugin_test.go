package sdk

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Skarlso/go-plugin-framework/types"
)

func TestNewPlugin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := types.Config{
		ID:   "test-plugin",
		Type: types.TCP,
	}

	ctx := context.Background()
	plugin := NewPlugin(ctx, logger, config, os.Stdout)

	require.Equal(t, "test-plugin", plugin.Config.ID)
	require.Equal(t, types.TCP, plugin.Config.Type)
	require.NotNil(t, plugin.interrupt)
	require.Equal(t, int64(0), plugin.workerCounter.Load())
}

func TestPluginWorkTracking(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	config := types.Config{
		ID:   "test-plugin",
		Type: types.TCP,
	}

	ctx := context.Background()
	plugin := NewPlugin(ctx, logger, config, os.Stdout)

	// Initially no work
	require.Equal(t, int64(0), plugin.workerCounter.Load())

	// Start work
	plugin.StartWork()
	require.Equal(t, int64(1), plugin.workerCounter.Load())

	// Start more work
	plugin.StartWork()
	require.Equal(t, int64(2), plugin.workerCounter.Load())

	// Stop work
	plugin.StopWork()
	require.Equal(t, int64(1), plugin.workerCounter.Load())

	plugin.StopWork()
	require.Equal(t, int64(0), plugin.workerCounter.Load())
}

func TestPluginIdleTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	shortTimeout := 100 * time.Millisecond
	config := types.Config{
		ID:          "test-plugin",
		Type:        types.TCP,
		IdleTimeout: &shortTimeout,
	}

	ctx := context.Background()
	plugin := NewPlugin(ctx, logger, config, os.Stdout)

	// This test would need more sophisticated setup to actually test the idle checker
	// but we can at least verify the configuration is set
	require.Equal(t, &shortTimeout, plugin.Config.IdleTimeout)
}
