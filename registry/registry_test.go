package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockPlugin implements PluginBase for testing
type MockPlugin struct {
	name string
}

func (m *MockPlugin) Ping(ctx context.Context) error {
	return nil
}

func TestRegistryInternalPlugins(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry(ctx)

	// Register an internal plugin
	mockPlugin := &MockPlugin{name: "test-plugin"}
	err := registry.RegisterInternal("test-type", mockPlugin)
	require.NoError(t, err)

	// Retrieve the plugin
	plugin, err := registry.GetPlugin(ctx, "test-type")
	require.NoError(t, err)
	require.Equal(t, mockPlugin, plugin)

	// Try to register the same type again
	err = registry.RegisterInternal("test-type", &MockPlugin{name: "another-plugin"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already registered")
}

func TestRegistryPluginNotFound(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry(ctx)

	// Try to get a non-existent plugin
	_, err := registry.GetPlugin(ctx, "non-existent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no plugin found")
}

func TestRegistryShutdown(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry(ctx)

	// Register an internal plugin
	mockPlugin := &MockPlugin{name: "test-plugin"}
	err := registry.RegisterInternal("test-type", mockPlugin)
	require.NoError(t, err)

	// Shutdown should not error even with internal plugins
	err = registry.Shutdown(ctx)
	require.NoError(t, err)
}
