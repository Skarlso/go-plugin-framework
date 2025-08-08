package contracts

import "context"

// DataProcessor is an example generic plugin contract for data processing.
type DataProcessor interface {
	PluginBase
	
	// ProcessData processes input data and returns the result.
	ProcessData(ctx context.Context, input []byte) ([]byte, error)
	
	// GetSupportedFormats returns a list of data formats this plugin supports.
	GetSupportedFormats(ctx context.Context) ([]string, error)
}

// DataProcessorRequest represents a request to process data.
type DataProcessorRequest struct {
	Data   []byte            `json:"data"`
	Format string            `json:"format"`
	Config map[string]string `json:"config,omitempty"`
}

// DataProcessorResponse represents the response from data processing.
type DataProcessorResponse struct {
	Data     []byte `json:"data"`
	Format   string `json:"format"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}