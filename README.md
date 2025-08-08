# Go Plugin Framework

A flexible, production-ready plugin system for Go applications that supports both internal and external plugins with HTTP-based communication.

## Features

- **Dual Plugin Support**: Internal (compiled-in) and external (separate binaries) plugins
- **HTTP Communication**: TCP and Unix socket communication protocols
- **Lifecycle Management**: Automatic idle timeout, graceful shutdown, process monitoring
- **Type Safety**: JSON schema validation and strongly typed contracts
- **Security**: Process isolation, lock file management, path sanitization
- **Extensible**: Generic plugin contracts with custom implementations

## Quick Start

### 1. Define a Plugin Contract

```go
package contracts

import "context"

type DataProcessor interface {
    PluginBase
    ProcessData(ctx context.Context, input []byte) ([]byte, error)
    GetSupportedFormats(ctx context.Context) ([]string, error)
}
```

### 2. Create an External Plugin

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    "os"
    
    "github.com/Skarlso/go-plugin-framework/sdk"
    "github.com/Skarlso/go-plugin-framework/types"
)

type MyProcessor struct{}

func (mp *MyProcessor) ProcessData(ctx context.Context, input []byte) ([]byte, error) {
    // Your processing logic here
    return input, nil
}

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
    
    // Handle capabilities request
    if len(os.Args) > 1 && os.Args[1] == "capabilities" {
        capabilities := types.PluginCapabilities{
            Types: map[string][]types.TypeInfo{
                "dataProcessor": {{
                    Type: "my-processor",
                    JSONSchema: []byte(`{"type": "object"}`),
                }},
            },
        }
        
        json.NewEncoder(os.Stdout).Encode(capabilities)
        return
    }
    
    // Parse configuration and start plugin
    // ... configuration parsing ...
    
    plugin := sdk.NewPlugin(context.Background(), logger, config, os.Stdout)
    
    // Register HTTP handlers
    plugin.RegisterHandlers(/* your handlers */)
    
    plugin.Start(context.Background())
}
```

### 3. Host Application

```go
package main

import (
    "context"
    "log/slog"
    
    "github.com/Skarlso/go-plugin-framework/manager"
)

func main() {
    ctx := context.Background()
    pm := manager.NewPluginManager(ctx)
    
    // Register plugins from directory
    err := pm.RegisterPlugins(ctx, "./plugins",
        manager.WithIdleTimeout(5*time.Minute),
    )
    if err != nil {
        panic(err)
    }
    
    // Get and use a plugin
    plugin, err := pm.GetPlugin(ctx, "dataProcessor")
    if err != nil {
        panic(err)
    }
    
    // Use the plugin
    plugin.Ping(ctx)
    
    // Cleanup
    pm.Shutdown(ctx)
}
```

## Architecture

```
┌─────────────────┐    ┌───────────────────┐    ┌──────────────────┐
│   Host App      │    │  Plugin Manager   │    │   External       │
│                 │◄──►│                   │◄──►│   Plugin         │
│ ┌─────────────┐ │    │ ┌───────────────┐ │    │ ┌──────────────┐ │
│ │ Your Code   │ │    │ │   Registry    │ │    │ │ HTTP Server  │ │
│ └─────────────┘ │    │ └───────────────┘ │    │ └──────────────┘ │
└─────────────────┘    └───────────────────┘    └──────────────────┘
         │                        │                        │
         │                        │                        │
    ┌────▼────┐              ┌────▼────┐              ┌────▼────┐
    │Internal │              │ TCP or  │              │Plugin   │
    │Plugins  │              │Unix     │              │Binary   │
    └─────────┘              │Sockets  │              └─────────┘
                             └─────────┘
```

### Key Components

- **Plugin Manager**: Central orchestrator for plugin discovery and lifecycle
- **Registry**: Manages both internal and external plugin instances
- **SDK**: Framework for building external plugins with HTTP servers
- **Contracts**: Type-safe interfaces defining plugin capabilities
- **Communication**: HTTP-based protocol over TCP or Unix sockets

## Plugin Lifecycle

1. **Discovery**: Manager scans directory for executable files
2. **Capabilities**: Calls `./plugin capabilities` to get metadata
3. **Configuration**: Passes config via `--config` JSON flag
4. **Startup**: Plugin starts HTTP server and outputs connection details
5. **Registration**: Manager connects and registers plugin endpoints
6. **Runtime**: Plugin handles requests until idle timeout or shutdown
7. **Cleanup**: Graceful shutdown removes sockets and processes

## Communication Protocol

External plugins communicate via HTTP:

- **Connection**: TCP (host:port) or Unix sockets
- **Content-Type**: `application/json`
- **Health Check**: `GET /healthz` 
- **Shutdown**: `POST /shutdown`
- **Custom Endpoints**: Defined by plugin contract

Example request/response:
```json
POST /process
{
  "data": "aGVsbG8gd29ybGQ=",
  "format": "text/plain"
}

Response:
{
  "data": "SEVMTE8gV09STEQ=",
  "metadata": {"processed_by": "my-plugin"}
}
```

## Security Features

- **Process Isolation**: External plugins run in separate processes
- **Lock Files**: Prevent socket conflicts with PID tracking
- **Path Sanitization**: Removes potentially malicious characters
- **Timeout Controls**: Automatic idle shutdown prevents resource leaks
- **Signal Handling**: Graceful SIGINT/SIGTERM handling

## Configuration

### Plugin Configuration
```json
{
  "id": "my-plugin-instance",
  "type": "tcp",
  "idleTimeout": "5m",
  "configTypes": [
    {
      "type": "my-config-type",
      "data": "{\"setting\": \"value\"}"
    }
  ]
}
```

### Registration Options
```go
pm.RegisterPlugins(ctx, dir,
    manager.WithIdleTimeout(10*time.Minute),
    manager.WithConfigData(configData),
    manager.WithPluginFilter(func(name string) bool {
        return strings.HasPrefix(name, "my-")
    }),
)
```

## Examples

See the [`examples/`](examples/) directory for:

- **simple-processor**: Basic data processing plugin
- **host**: Example host application using the plugin system
- **transformer**: Advanced plugin with multiple operations

## Testing

Run tests with:
```bash
go test ./...
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.