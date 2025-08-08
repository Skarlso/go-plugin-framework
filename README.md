# Go Plugin Framework

A flexible, production-ready plugin system for Go applications that supports both internal and external plugins with HTTP-based communication.

## Features

This framework provides dual plugin support, allowing you to use both internal plugins (compiled directly into your
application) and external plugins (separate binary executables). Communication happens over HTTP using either TCP
connections or Unix sockets.

The system includes automatic lifecycle management with idle timeouts, graceful shutdown, and process monitoring.
Type safety is maintained through JSON schema validation and strongly typed contracts. Security features include
process isolation, lock file management for socket conflicts, and path sanitization.

The framework is extensible through generic plugin contracts that allow custom implementations for different use cases.

## Quick Start

First, define a plugin contract that specifies what your plugins can do:

```go
package contracts

import "context"

type DataProcessor interface {
    PluginBase
    ProcessData(ctx context.Context, input []byte) ([]byte, error)
    GetSupportedFormats(ctx context.Context) ([]string, error)
}
```

Next, create an external plugin that implements your contract:

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

Finally, create a host application that uses your plugins:

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

The framework consists of several key components that work together. The Plugin Manager serves as the central orchestrator for plugin discovery and lifecycle management. The Registry manages both internal and external plugin instances, keeping track of what's available and how to access it.

The SDK provides a framework for building external plugins with HTTP servers, while Contracts define type-safe interfaces that specify plugin capabilities. Communication between components uses an HTTP-based protocol that can run over either TCP connections or Unix sockets.

## Plugin Lifecycle

Plugin management follows a predictable lifecycle. During discovery, the manager scans a directory for executable files that could be plugins. It then queries each potential plugin by calling `./plugin capabilities` to get metadata about what the plugin can do.

Once a plugin is identified, the manager passes configuration data via a `--config` JSON flag when starting it up. The plugin responds by starting an HTTP server and outputting connection details so the manager knows how to reach it.

After registration, where the manager connects to the plugin and registers its endpoints, the plugin enters its runtime phase. During this time, it handles requests until either an idle timeout is reached or a shutdown is requested. Finally, during cleanup, the system performs a graceful shutdown that removes socket files and terminates processes properly.

## Communication Protocol

External plugins communicate with the host application using HTTP. Connections can be made over TCP using a host:port combination, or through Unix sockets for local communication. All communication uses `application/json` as the content type.

The framework provides standard endpoints for health checking (`GET /healthz`) and shutdown (`POST /shutdown`). Beyond these, plugins can define custom endpoints based on their specific contracts and functionality.

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

The framework includes several security measures to protect both the host application and the plugins. External plugins run in separate processes, providing isolation from the main application. Lock files prevent socket conflicts by tracking process IDs, ensuring that only one plugin can use a socket at a time.

Path sanitization removes potentially malicious characters from file paths before use. Timeout controls automatically shut down idle plugins to prevent resource leaks, and the system handles SIGINT and SIGTERM signals gracefully to ensure clean shutdowns.

## Configuration

Plugins can be configured using JSON. Here's an example plugin configuration:
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

You can also configure registration options when setting up the plugin manager:
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

The [`examples/`](examples/) directory contains working examples to help you get started. The simple-processor shows a basic data processing plugin, while the host directory contains an example host application that demonstrates how to use the plugin system. There's also a transformer example that shows more advanced plugin operations.

## Testing

Run the test suite with `go test ./...` to verify everything works correctly.

## Contributing

To contribute to this project, fork the repository and create a feature branch for your changes. Make sure to add tests for any new functionality and verify that all existing tests still pass before submitting a pull request.

## License

MIT License - see [LICENSE](LICENSE) file for details.