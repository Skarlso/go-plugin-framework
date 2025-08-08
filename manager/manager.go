package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Skarlso/go-plugin-framework/contracts"
	"github.com/Skarlso/go-plugin-framework/registry"
	"github.com/Skarlso/go-plugin-framework/registry/plugins"
	"github.com/Skarlso/go-plugin-framework/types"
)

// ErrNoPluginsFound is returned when a register plugin call finds no plugins.
var ErrNoPluginsFound = errors.New("no plugins found")

// PluginManager manages all connected plugins.
type PluginManager struct {
	// Registry holds all registered plugins by their type.
	Registry *registry.Registry

	mu sync.Mutex

	// baseCtx is the context that is used for all plugins.
	// This is a different context than the one used for fetching plugins because
	// that context is done once fetching is done. The plugin context, however, must not
	// be cancelled.
	baseCtx context.Context
}

// NewPluginManager initializes the PluginManager
// the passed ctx is used for all plugins.
func NewPluginManager(ctx context.Context) *PluginManager {
	return &PluginManager{
		Registry: registry.NewRegistry(ctx),
		baseCtx:  ctx,
	}
}

// RegistrationOptions holds configuration for plugin registration.
type RegistrationOptions struct {
	IdleTimeout  time.Duration
	ConfigData   []types.ConfigData
	PluginFilter func(string) bool // Filter function to decide which plugins to register
}

// RegistrationOptionFn is a function that configures RegistrationOptions.
type RegistrationOptionFn func(*RegistrationOptions)

// WithIdleTimeout configures the maximum amount of time for a plugin to quit if it's idle.
func WithIdleTimeout(d time.Duration) RegistrationOptionFn {
	return func(o *RegistrationOptions) {
		o.IdleTimeout = d
	}
}

// WithConfigData adds configuration data to be passed to plugins.
func WithConfigData(configData []types.ConfigData) RegistrationOptionFn {
	return func(o *RegistrationOptions) {
		o.ConfigData = configData
	}
}

// WithPluginFilter sets a filter function to decide which plugins to register.
func WithPluginFilter(filter func(string) bool) RegistrationOptionFn {
	return func(o *RegistrationOptions) {
		o.PluginFilter = filter
	}
}

// RegisterPlugins walks through files in a folder and registers them
// as plugins if connection points can be established. This function doesn't support
// concurrent access.
func (pm *PluginManager) RegisterPlugins(ctx context.Context, dir string, opts ...RegistrationOptionFn) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	defaultOpts := &RegistrationOptions{
		IdleTimeout:  time.Hour,
		PluginFilter: func(string) bool { return true }, // Accept all plugins by default
	}

	for _, opt := range opts {
		opt(defaultOpts)
	}

	conf := &types.Config{
		IdleTimeout: &defaultOpts.IdleTimeout,
		ConfigTypes: defaultOpts.ConfigData,
	}

	t, err := determineConnectionType()
	if err != nil {
		return fmt.Errorf("could not determine connection type: %w", err)
	}
	conf.Type = t

	plugins, err := pm.fetchPlugins(ctx, conf, dir, defaultOpts.PluginFilter)
	if err != nil {
		return fmt.Errorf("could not fetch plugins: %w", err)
	}

	if len(plugins) == 0 {
		return ErrNoPluginsFound
	}

	for _, plugin := range plugins {
		conf.ID = plugin.ID
		plugin.Config = *conf

		output := bytes.NewBuffer(nil)
		cmd := exec.CommandContext(ctx, cleanPath(plugin.Path), "capabilities") //nolint:gosec // G204 does not apply
		cmd.Stdout = output
		cmd.Stderr = os.Stderr

		// Use Wait so we get the capabilities and make sure that the command exists and returns the values we need.
		if err := cmd.Run(); err != nil {
			slog.WarnContext(ctx, "failed to get capabilities from plugin, skipping", "plugin", plugin.ID, "error", err)
			continue
		}

		if err := pm.addPlugin(pm.baseCtx, *plugin, output); err != nil {
			slog.WarnContext(ctx, "failed to add plugin, skipping", "plugin", plugin.ID, "error", err)
			continue
		}
	}

	return nil
}

func cleanPath(path string) string {
	return strings.Trim(path, `,;:'"|&*!@#$`)
}

// Shutdown is called to terminate all plugins.
func (pm *PluginManager) Shutdown(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.Registry.Shutdown(ctx)
}

// GetPlugin returns a plugin that implements the specified contract.
func (pm *PluginManager) GetPlugin(ctx context.Context, pluginType string) (contracts.PluginBase, error) {
	return pm.Registry.GetPlugin(ctx, pluginType)
}

// ExternalPluginWrapper is re-exported from registry for use by client applications.
type ExternalPluginWrapper = registry.ExternalPluginWrapper

// RegisterInternalPlugin registers an internal plugin implementation.
func (pm *PluginManager) RegisterInternalPlugin(pluginType string, plugin contracts.PluginBase) error {
	return pm.Registry.RegisterInternal(pluginType, plugin)
}

func (pm *PluginManager) fetchPlugins(ctx context.Context, conf *types.Config, dir string, filter func(string) bool) ([]*types.Plugin, error) {
	var plugins []*types.Plugin
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return ErrNoPluginsFound
		}

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip files with extensions (we want executables)
		ext := filepath.Ext(info.Name())
		if ext != "" {
			return nil
		}

		id := filepath.Base(path)

		// Apply filter if provided
		if !filter(id) {
			slog.DebugContext(ctx, "skipping plugin due to filter", "id", id, "path", path)
			return nil
		}

		p := &types.Plugin{
			ID:     id,
			Path:   path,
			Config: *conf,
		}

		slog.DebugContext(ctx, "discovered plugin", "id", id, "path", path)

		plugins = append(plugins, p)

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to discover plugins: %w", err)
	}

	return plugins, nil
}

func (pm *PluginManager) addPlugin(ctx context.Context, plugin types.Plugin, capabilitiesCommandOutput *bytes.Buffer) error {
	// Determine Configuration requirements.
	capabilities := &types.PluginCapabilities{}
	if err := json.Unmarshal(capabilitiesCommandOutput.Bytes(), capabilities); err != nil {
		return fmt.Errorf("failed to unmarshal capabilities: %w", err)
	}

	serialized, err := json.Marshal(plugin.Config)
	if err != nil {
		return err
	}

	// Create a command that can then be managed.
	pluginCmd := exec.CommandContext(ctx, cleanPath(plugin.Path), "--config", string(serialized)) //nolint:gosec // G204 does not apply
	pluginCmd.Cancel = func() error {
		slog.Info("killing plugin process because the parent context is cancelled", "id", plugin.ID)
		return pluginCmd.Process.Kill()
	}

	// Set up communication pipes.
	plugin.Cmd = pluginCmd
	sdtErr, err := pluginCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	sdtOut, err := pluginCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	plugin.Types = capabilities.Types

	// Register the plugin with the registry
	return pm.Registry.AddExternalPlugin(plugin)
}

func determineConnectionType() (types.ConnectionType, error) {
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmp)
	}()

	socketPath := filepath.Join(tmp, "plugin.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return types.TCP, nil
	}

	if err := listener.Close(); err != nil {
		return "", fmt.Errorf("failed to close socket: %w", err)
	}

	return types.Socket, nil
}
