package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Skarlso/go-plugin-framework/contracts"
	"github.com/Skarlso/go-plugin-framework/manager"
	"github.com/Skarlso/go-plugin-framework/registry/plugins"
)

// DataProcessorClient wraps external plugin calls to implement the DataProcessor interface.
type DataProcessorClient struct {
	wrapper    *manager.ExternalPluginWrapper // This would need to be accessible from manager package
	pluginType string
}

func (dpc *DataProcessorClient) Ping(ctx context.Context) error {
	return dpc.wrapper.Ping(ctx)
}

func (dpc *DataProcessorClient) ProcessData(ctx context.Context, input []byte) ([]byte, error) {
	request := contracts.DataProcessorRequest{
		Data:   input,
		Format: "text/plain",
	}

	var response contracts.DataProcessorResponse
	err := dpc.wrapper.CallPlugin(ctx, "/process", "POST",
		plugins.WithPayload(request),
		plugins.WithResult(&response),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call plugin: %w", err)
	}

	return response.Data, nil
}

func (dpc *DataProcessorClient) GetSupportedFormats(ctx context.Context) ([]string, error) {
	var response map[string][]string
	err := dpc.wrapper.CallPlugin(ctx, "/formats", "GET",
		plugins.WithResult(&response),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call plugin: %w", err)
	}

	return response["formats"], nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Create plugin manager
	pm := manager.NewPluginManager(ctx)

	// Get the directory where plugins are located
	pluginDir := os.Getenv("PLUGIN_DIR")
	if pluginDir == "" {
		// Default to a plugins directory relative to this example
		wd, err := os.Getwd()
		if err != nil {
			logger.Error("failed to get working directory", "error", err)
			os.Exit(1)
		}
		pluginDir = filepath.Join(wd, "plugins")
	}

	logger.Info("Looking for plugins", "directory", pluginDir)

	// Register plugins from directory
	err := pm.RegisterPlugins(ctx, pluginDir,
		manager.WithIdleTimeout(5*time.Minute),
		manager.WithPluginFilter(func(name string) bool {
			// Only load plugins that start with "simple-"
			return len(name) > 7 && name[:7] == "simple-"
		}),
	)

	if err != nil {
		logger.Error("failed to register plugins", "error", err)
		os.Exit(1)
	}

	// Example: Get a data processor plugin
	plugin, err := pm.GetPlugin(ctx, "dataProcessor")
	if err != nil {
		logger.Error("failed to get data processor plugin", "error", err)
		os.Exit(1)
	}

	// Test the plugin
	logger.Info("Testing plugin...")

	// First, ping the plugin
	if err := plugin.Ping(ctx); err != nil {
		logger.Error("failed to ping plugin", "error", err)
		os.Exit(1)
	}
	logger.Info("Plugin is responsive")

	// If it's an external plugin, we need to use the wrapper to call specific methods
	if wrapper, ok := plugin.(*manager.ExternalPluginWrapper); ok {
		client := &DataProcessorClient{wrapper: wrapper, pluginType: "dataProcessor"}

		// Test data processing
		testData := []byte("hello world!")
		result, err := client.ProcessData(ctx, testData)
		if err != nil {
			logger.Error("failed to process data", "error", err)
			os.Exit(1)
		}

		logger.Info("Processing result", "input", string(testData), "output", string(result))

		// Test getting supported formats
		formats, err := client.GetSupportedFormats(ctx)
		if err != nil {
			logger.Error("failed to get supported formats", "error", err)
			os.Exit(1)
		}

		logger.Info("Supported formats", "formats", formats)
	}

	// Cleanup
	if err := pm.Shutdown(ctx); err != nil {
		logger.Error("failed to shutdown plugin manager", "error", err)
	}

	logger.Info("Example completed successfully")
}
