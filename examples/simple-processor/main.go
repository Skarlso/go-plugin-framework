package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Skarlso/go-plugin-framework/contracts"
	"github.com/Skarlso/go-plugin-framework/sdk"
	"github.com/Skarlso/go-plugin-framework/types"
)

// SimpleProcessor is an example data processor plugin.
type SimpleProcessor struct {
	contracts.EmptyBasePlugin
}

// ProcessData implements a simple string transformation.
func (sp *SimpleProcessor) ProcessData(ctx context.Context, input []byte) ([]byte, error) {
	// Simple example: convert to uppercase
	result := strings.ToUpper(string(input))
	return []byte(result), nil
}

// GetSupportedFormats returns the formats this processor supports.
func (sp *SimpleProcessor) GetSupportedFormats(ctx context.Context) ([]string, error) {
	return []string{"text/plain", "text"}, nil
}

var logger *slog.Logger

// HTTP handlers for the plugin endpoints
func (sp *SimpleProcessor) handleProcessData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req contracts.DataProcessorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	result, err := sp.ProcessData(r.Context(), req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := contracts.DataProcessorResponse{
		Data:   result,
		Format: req.Format,
		Metadata: map[string]interface{}{
			"processed_by": "simple-processor",
			"operation":    "uppercase",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

func (sp *SimpleProcessor) handleGetFormats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	formats, err := sp.GetSupportedFormats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string][]string{"formats": formats}); err != nil {
		logger.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

func main() {
	args := os.Args[1:]
	// log messages are shared over stderr by convention established by the plugin manager.
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	processor := &SimpleProcessor{}

	// Handle capabilities request
	if len(args) > 0 && args[0] == "capabilities" {
		capabilities := types.PluginCapabilities{
			Types: map[string][]types.TypeInfo{
				"dataProcessor": {
					{
						Type:       "simple-text-processor",
						JSONSchema: []byte(`{"type": "object", "properties": {"format": {"type": "string"}}}`),
					},
				},
			},
			ConfigTypes: []string{}, // This plugin doesn't require specific config
		}

		content, err := json.Marshal(capabilities)
		if err != nil {
			logger.Error("failed to marshal capabilities", "error", err)
			os.Exit(1)
		}

		if _, err := fmt.Fprintln(os.Stdout, string(content)); err != nil {
			logger.Error("failed print capabilities", "error", err)
			os.Exit(1)
		}

		logger.Info("capabilities sent")
		os.Exit(0)
	}

	// Parse command-line arguments
	configData := flag.String("config", "", "Plugin config.")
	flag.Parse()
	if configData == nil || *configData == "" {
		logger.Error("missing required flag --config")
		os.Exit(1)
	}

	conf := types.Config{}
	if err := json.Unmarshal([]byte(*configData), &conf); err != nil {
		logger.Error("failed to unmarshal config", "error", err)
		os.Exit(1)
	}
	logger.Debug("config data", "config", conf)

	if conf.ID == "" {
		logger.Error("plugin config has no ID")
		os.Exit(1)
	}

	// Create the plugin
	ctx := context.Background()
	plugin := sdk.NewPlugin(ctx, logger, conf, os.Stdout)

	// Register HTTP handlers
	handlers := []sdk.Handler{
		{
			Location: "/process",
			Handler:  processor.handleProcessData,
		},
		{
			Location: "/formats",
			Handler:  processor.handleGetFormats,
		},
	}

	if err := plugin.RegisterHandlers(handlers...); err != nil {
		logger.Error("failed to register handlers", "error", err)
		os.Exit(1)
	}

	logger.Info("starting up plugin", "plugin", conf.ID)

	if err := plugin.Start(ctx); err != nil {
		logger.Error("failed to start plugin", "error", err)
		os.Exit(1)
	}
}
