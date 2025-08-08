package plugins

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Skarlso/go-plugin-framework/types"
)

// WaitForPlugin waits for a plugin to start up and become available.
// It reads the plugin's stdout to get the connection details and then
// creates an HTTP client to communicate with the plugin.
func WaitForPlugin(ctx context.Context, plugin *types.Plugin) (*http.Client, string, error) {
	scanner := bufio.NewScanner(plugin.Cmd.Stdout)

	// Read the first line which should contain the connection location
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, "", fmt.Errorf("failed to read plugin location: %w", err)
		}
		return nil, "", fmt.Errorf("plugin did not output connection location")
	}

	location := strings.TrimSpace(scanner.Text())
	if location == "" {
		return nil, "", fmt.Errorf("plugin output empty location")
	}

	client, err := createHTTPClient(plugin.Config.Type, location)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Wait for the plugin to be ready
	if err := waitForPluginReady(ctx, client, plugin.Config.Type, location); err != nil {
		return nil, "", fmt.Errorf("plugin failed to become ready: %w", err)
	}

	return client, location, nil
}

func createHTTPClient(connType types.ConnectionType, location string) (*http.Client, error) {
	switch connType {
	case types.TCP:
		// For TCP, location is already a host:port
		return &http.Client{
			Timeout: 30 * time.Second,
		}, nil
	case types.Socket:
		// For Unix socket, extract the socket path from the URL
		socketURL, err := url.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("failed to parse socket URL: %w", err)
		}

		socketPath := socketURL.Path
		if socketURL.Scheme == "http+unix" {
			// Remove the scheme to get the actual socket path
			socketPath = strings.TrimPrefix(location, "http+unix://")
		}

		return &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: 30 * time.Second,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported connection type: %s", connType)
	}
}

func waitForPluginReady(ctx context.Context, client *http.Client, connType types.ConnectionType, location string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for plugin to become ready")
		case <-ticker.C:
			if err := Call(ctx, client, connType, location, "/healthz", http.MethodGet); err == nil {
				return nil
			}
			// Continue trying if health check fails
		}
	}
}
