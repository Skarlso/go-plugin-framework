package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/Skarlso/go-plugin-framework/contracts"
	"github.com/Skarlso/go-plugin-framework/registry/plugins"
	"github.com/Skarlso/go-plugin-framework/types"
)

// Registry manages both internal and external plugins.
type Registry struct {
	ctx context.Context
	mu  sync.RWMutex

	// internalPlugins holds plugins that are compiled into the application
	internalPlugins map[string]contracts.PluginBase

	// externalPlugins holds external plugin processes
	externalPlugins map[string]*ExternalPlugin
}

// ExternalPlugin represents a running external plugin.
type ExternalPlugin struct {
	Plugin types.Plugin
	Client contracts.PluginBase
}

// NewRegistry creates a new plugin registry.
func NewRegistry(ctx context.Context) *Registry {
	return &Registry{
		ctx:             ctx,
		internalPlugins: make(map[string]contracts.PluginBase),
		externalPlugins: make(map[string]*ExternalPlugin),
	}
}

// RegisterInternal registers an internal plugin implementation.
func (r *Registry) RegisterInternal(pluginType string, plugin contracts.PluginBase) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.internalPlugins[pluginType]; exists {
		return fmt.Errorf("internal plugin for type %q already registered", pluginType)
	}

	r.internalPlugins[pluginType] = plugin
	return nil
}

// AddExternalPlugin starts and registers an external plugin.
func (r *Registry) AddExternalPlugin(plugin types.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if we already have a plugin for any of the types this plugin supports
	for pluginType := range plugin.Types {
		if _, exists := r.internalPlugins[pluginType]; exists {
			return fmt.Errorf("internal plugin for type %q already registered", pluginType)
		}
		if _, exists := r.externalPlugins[pluginType]; exists {
			return fmt.Errorf("external plugin for type %q already registered", pluginType)
		}
	}

	// Start the plugin
	if err := plugin.Cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin %s: %w", plugin.ID, err)
	}

	// Wait for the plugin to be ready
	client, location, err := plugins.WaitForPlugin(r.ctx, &plugin)
	if err != nil {
		return fmt.Errorf("failed to wait for plugin %s to start: %w", plugin.ID, err)
	}

	// Create a wrapper that implements the PluginBase interface
	pluginWrapper := &ExternalPluginWrapper{
		client:         client,
		location:       location,
		connectionType: plugin.Config.Type,
		plugin:         &plugin,
	}

	externalPlugin := &ExternalPlugin{
		Plugin: plugin,
		Client: pluginWrapper,
	}

	// Register for all types this plugin supports
	for pluginType := range plugin.Types {
		r.externalPlugins[pluginType] = externalPlugin
	}

	return nil
}

// GetPlugin returns a plugin for the specified type.
func (r *Registry) GetPlugin(ctx context.Context, pluginType string) (contracts.PluginBase, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check internal plugins first
	if plugin, exists := r.internalPlugins[pluginType]; exists {
		return plugin, nil
	}

	// Check external plugins
	if externalPlugin, exists := r.externalPlugins[pluginType]; exists {
		return externalPlugin.Client, nil
	}

	return nil, fmt.Errorf("no plugin found for type %q", pluginType)
}

// Shutdown stops all external plugins.
func (r *Registry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error

	// Shutdown all external plugins
	processedPlugins := make(map[string]bool)
	for _, externalPlugin := range r.externalPlugins {
		if processedPlugins[externalPlugin.Plugin.ID] {
			continue // Already processed this plugin
		}
		processedPlugins[externalPlugin.Plugin.ID] = true

		// Send interrupt signal to the plugin
		if err := externalPlugin.Plugin.Cmd.Process.Signal(os.Interrupt); err != nil {
			errs = append(errs, fmt.Errorf("failed to send interrupt to plugin %s: %w", externalPlugin.Plugin.ID, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ExternalPluginWrapper wraps an external plugin to implement the PluginBase interface.
type ExternalPluginWrapper struct {
	client         *http.Client
	location       string
	connectionType types.ConnectionType
	plugin         *types.Plugin
}

// Ping implements the PluginBase interface.
func (w *ExternalPluginWrapper) Ping(ctx context.Context) error {
	return plugins.Call(ctx, w.client, w.connectionType, w.location, "/healthz", "GET")
}

// GetHTTPClient returns the HTTP client for making calls to the plugin.
func (w *ExternalPluginWrapper) GetHTTPClient() *http.Client {
	return w.client
}

// GetLocation returns the plugin's connection location.
func (w *ExternalPluginWrapper) GetLocation() string {
	return w.location
}

// GetConnectionType returns the plugin's connection type.
func (w *ExternalPluginWrapper) GetConnectionType() types.ConnectionType {
	return w.connectionType
}

// CallPlugin makes an HTTP call to the plugin.
func (w *ExternalPluginWrapper) CallPlugin(ctx context.Context, endpoint, method string, opts ...plugins.CallOptionFn) error {
	return plugins.Call(ctx, w.client, w.connectionType, w.location, endpoint, method, opts...)
}
