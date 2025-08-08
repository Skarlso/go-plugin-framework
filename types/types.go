package types

import (
	"os/exec"
	"time"
)

// ConnectionType defines the connection type for plugin communication.
type ConnectionType string

const (
	TCP    ConnectionType = "tcp"
	Socket ConnectionType = "unix"
)

// Config holds the configuration for a plugin.
type Config struct {
	// ID is a unique identifier for the plugin instance.
	ID string `json:"id"`
	// Type is the connection type (tcp or unix).
	Type ConnectionType `json:"type"`
	// IdleTimeout specifies how long a plugin can be idle before shutting down.
	IdleTimeout *time.Duration `json:"idleTimeout,omitempty"`
	// ConfigTypes holds configuration data passed to the plugin during startup.
	ConfigTypes []ConfigData `json:"configTypes,omitempty"`
}

// ConfigData represents a single configuration item.
type ConfigData struct {
	Type string `json:"type"`
	Data []byte `json:"data"`
}

// Plugin represents a discovered plugin with its metadata and runtime information.
type Plugin struct {
	// ID is the plugin's unique identifier.
	ID string
	// Path is the filesystem path to the plugin executable.
	Path string
	// Config holds the plugin's configuration.
	Config Config
	// Cmd is the command used to start the plugin process.
	Cmd *exec.Cmd
	// Types holds the plugin's declared capabilities.
	Types map[string][]TypeInfo
}

// TypeInfo defines a plugin's supported type and its JSON schema.
type TypeInfo struct {
	// Type defines the type name that this plugin supports.
	Type string `json:"type"`
	// JSONSchema holds the schema for the type.
	JSONSchema []byte `json:"jsonSchema"`
}

// PluginCapabilities holds the capabilities declared by a plugin.
type PluginCapabilities struct {
	// Types define a plugin type specific list of types that the plugin supports.
	Types map[string][]TypeInfo `json:"types"`
	// ConfigTypes define a list of configuration types the plugin understands.
	ConfigTypes []string `json:"configTypes,omitempty"`
}

// Location describes where plugin data can be found.
type Location struct {
	LocationType `json:"type"`
	Value        string `json:"value"`
}

// LocationType defines how data is made available.
type LocationType string

const (
	// LocationTypeRemoteURL is a remote URL accessible to the plugin.
	LocationTypeRemoteURL LocationType = "remoteURL"
	// LocationTypeUnixNamedPipe is a Unix named pipe.
	LocationTypeUnixNamedPipe LocationType = "unixNamedPipe" 
	// LocationTypeLocalFile is a local file on the filesystem.
	LocationTypeLocalFile LocationType = "localFile"
)