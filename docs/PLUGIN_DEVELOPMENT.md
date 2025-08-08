# Plugin Development Guide

This guide covers how to create plugins using the Go Plugin Framework.

## Plugin Types

### Internal Plugins

Internal plugins are compiled directly into your host application.

#### Creating an Internal Plugin

1. **Implement the Contract Interface**

```go
package myplugin

import (
    "context"
    "github.com/Skarlso/go-plugin-framework/contracts"
)

type MyDataProcessor struct {
    contracts.EmptyBasePlugin // Provides Ping() implementation
}

func (m *MyDataProcessor) ProcessData(ctx context.Context, input []byte) ([]byte, error) {
    // Your processing logic
    return processedData, nil
}

func (m *MyDataProcessor) GetSupportedFormats(ctx context.Context) ([]string, error) {
    return []string{"json", "xml"}, nil
}
```

2. **Register with Plugin Manager**

```go
// In your host application
pm := manager.NewPluginManager(ctx)
processor := &myplugin.MyDataProcessor{}
err := pm.RegisterInternalPlugin("dataProcessor", processor)
```

### External Plugins

External plugins run as separate processes and communicate via HTTP.

#### Plugin Structure

```
my-plugin/
├── main.go          # Plugin entry point
├── handlers.go      # HTTP handlers
├── processor.go     # Business logic
└── go.mod
```

#### Implementation Steps

1. **Define Plugin Logic**

```go
package main

import (
    "context"
    "strings"
)

type SimpleProcessor struct{}

func (sp *SimpleProcessor) ProcessData(ctx context.Context, input []byte) ([]byte, error) {
    // Example: convert to uppercase
    result := strings.ToUpper(string(input))
    return []byte(result), nil
}

func (sp *SimpleProcessor) GetSupportedFormats(ctx context.Context) ([]string, error) {
    return []string{"text/plain"}, nil
}
```

2. **Create HTTP Handlers**

```go
func (sp *SimpleProcessor) handleProcessData(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req contracts.DataProcessorRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
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
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

3. **Plugin Main Function**

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))

    processor := &SimpleProcessor{}

    // Handle capabilities request
    if len(os.Args) > 1 && os.Args[1] == "capabilities" {
        capabilities := types.PluginCapabilities{
            Types: map[string][]types.TypeInfo{
                "dataProcessor": {
                    {
                        Type:       "simple-text-processor",
                        JSONSchema: []byte(`{"type": "object"}`),
                    },
                },
            },
        }

        json.NewEncoder(os.Stdout).Encode(capabilities)
        return
    }

    // Parse configuration
    configData := flag.String("config", "", "Plugin config")
    flag.Parse()

    var config types.Config
    json.Unmarshal([]byte(*configData), &config)

    // Create and start plugin
    plugin := sdk.NewPlugin(context.Background(), logger, config, os.Stdout)

    handlers := []sdk.Handler{
        {Location: "/process", Handler: processor.handleProcessData},
        {Location: "/formats", Handler: processor.handleGetFormats},
    }

    plugin.RegisterHandlers(handlers...)
    plugin.Start(context.Background())
}
```

## Plugin Contracts

### Defining Custom Contracts

```go
package contracts

import "context"

// Custom contract for image processing
type ImageProcessor interface {
    PluginBase
    
    ProcessImage(ctx context.Context, request *ImageProcessRequest) (*ImageProcessResponse, error)
    GetSupportedFormats(ctx context.Context) ([]string, error)
    GetFilters(ctx context.Context) ([]FilterInfo, error)
}

type ImageProcessRequest struct {
    ImageData []byte            `json:"imageData"`
    Format    string            `json:"format"`
    Filters   []FilterConfig    `json:"filters"`
}

type FilterConfig struct {
    Name       string                 `json:"name"`
    Parameters map[string]interface{} `json:"parameters"`
}
```

### Contract Best Practices

1. **Always embed PluginBase**: Ensures basic functionality
2. **Use context.Context**: For cancellation and deadlines
3. **Return structured errors**: With appropriate error codes
4. **Version your contracts**: Use semantic versioning
5. **Document schemas**: Provide JSON schemas for complex types

## Configuration

### Plugin Configuration Types

Plugins can declare configuration requirements:

```go
capabilities := types.PluginCapabilities{
    ConfigTypes: []string{
        "database-config",
        "logging-config",
        "cache-config",
    },
}
```

### Using Configuration in Plugins

```go
// Configuration is passed via the Config struct
func (p *MyPlugin) handleRequest(w http.ResponseWriter, r *http.Request) {
    // Access configuration
    for _, configData := range p.config.ConfigTypes {
        if configData.Type == "database-config" {
            var dbConfig DatabaseConfig
            json.Unmarshal(configData.Data, &dbConfig)
            // Use database configuration
        }
    }
}
```

## Error Handling

### HTTP Error Responses

```go
import "github.com/Skarlso/go-plugin-framework/registry/plugins"

func (p *MyPlugin) handleError(w http.ResponseWriter, err error, code int) {
    pluginError := plugins.NewError(err, code)
    pluginError.Write(w)
}
```

### Structured Error Types

```go
type ProcessingError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

func (pe *ProcessingError) Error() string {
    return pe.Message
}
```

## Testing Plugins

### Unit Testing Business Logic

```go
func TestSimpleProcessor_ProcessData(t *testing.T) {
    processor := &SimpleProcessor{}
    
    input := []byte("hello world")
    result, err := processor.ProcessData(context.Background(), input)
    
    require.NoError(t, err)
    require.Equal(t, []byte("HELLO WORLD"), result)
}
```

### Integration Testing with SDK

```go
func TestPluginIntegration(t *testing.T) {
    // Create test plugin
    config := types.Config{
        ID:   "test-plugin",
        Type: types.TCP,
    }
    
    logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
    plugin := sdk.NewPlugin(context.Background(), logger, config, os.Stdout)
    
    // Register test handlers
    processor := &SimpleProcessor{}
    handlers := []sdk.Handler{
        {Location: "/process", Handler: processor.handleProcessData},
    }
    
    plugin.RegisterHandlers(handlers...)
    
    // Test would involve starting plugin and making HTTP requests
}
```

## Performance Optimization

### Connection Reuse

```go
// Reuse HTTP clients in the SDK
type PluginClient struct {
    httpClient *http.Client
}

func (pc *PluginClient) makeRequest(ctx context.Context, endpoint string, payload interface{}) error {
    // Reuse connection
    return plugins.Call(ctx, pc.httpClient, connectionType, location, endpoint, "POST",
        plugins.WithPayload(payload))
}
```

### Batch Processing

```go
// Support batch operations for better performance
type BatchProcessRequest struct {
    Items []ProcessItem `json:"items"`
}

func (p *MyPlugin) handleBatchProcess(w http.ResponseWriter, r *http.Request) {
    var req BatchProcessRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    results := make([]ProcessResult, len(req.Items))
    for i, item := range req.Items {
        results[i] = p.processItem(item)
    }
    
    json.NewEncoder(w).Encode(BatchProcessResponse{Results: results})
}
```

### Memory Management

```go
// Use pools for frequently allocated objects
var requestPool = sync.Pool{
    New: func() interface{} {
        return &ProcessRequest{}
    },
}

func (p *MyPlugin) handleRequest(w http.ResponseWriter, r *http.Request) {
    req := requestPool.Get().(*ProcessRequest)
    defer func() {
        // Reset and return to pool
        *req = ProcessRequest{}
        requestPool.Put(req)
    }()
    
    // Use req...
}
```

## Deployment

### Building Plugins

```bash
# Build plugin binary
go build -o my-plugin main.go

# Make executable
chmod +x my-plugin

# Test capabilities
./my-plugin capabilities
```

### Plugin Distribution

1. **Binary Distribution**: Distribute compiled binaries
2. **Container Distribution**: Package in Docker containers
3. **Package Management**: Use Go modules or custom package managers

### Plugin Discovery

Plugins are discovered by scanning directories for executable files:

```
plugins/
├── data-processor          # Executable plugin
├── image-transformer       # Another plugin  
└── config.json            # Optional configuration
```

## Security Considerations

### Input Validation

```go
func (p *MyPlugin) validateInput(req *ProcessRequest) error {
    if len(req.Data) == 0 {
        return errors.New("data is required")
    }
    
    if len(req.Data) > maxDataSize {
        return errors.New("data too large")
    }
    
    return nil
}
```

### Resource Limits

```go
func (p *MyPlugin) processWithTimeout(ctx context.Context, data []byte) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    return p.processData(ctx, data)
}
```

### Secure Communication

- Use Unix sockets for local communication
- Validate all input data
- Implement proper error handling
- Log security-relevant events

## Troubleshooting

### Common Issues

1. **Plugin Not Found**: Check executable permissions and PATH
2. **Capabilities Error**: Ensure `capabilities` command returns valid JSON
3. **Connection Timeout**: Check firewall and network settings
4. **Memory Leaks**: Use profiling tools to identify issues

### Debugging

```go
// Enable debug logging
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

// Add request ID for tracking
func (p *MyPlugin) handleRequest(w http.ResponseWriter, r *http.Request) {
    requestID := uuid.New().String()
    ctx := context.WithValue(r.Context(), "requestID", requestID)
    
    p.logger.InfoContext(ctx, "handling request", "endpoint", r.URL.Path)
    
    // Process request...
}
```