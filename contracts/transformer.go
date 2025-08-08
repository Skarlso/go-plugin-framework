package contracts

import "context"

// Transformer is a generic plugin contract for data transformation.
type Transformer interface {
	PluginBase
	
	// Transform applies a transformation to the input data.
	Transform(ctx context.Context, request *TransformRequest) (*TransformResponse, error)
	
	// GetTransformations returns a list of available transformations.
	GetTransformations(ctx context.Context) ([]TransformationInfo, error)
}

// TransformRequest represents a transformation request.
type TransformRequest struct {
	Data           []byte            `json:"data"`
	Transformation string            `json:"transformation"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
}

// TransformResponse represents the result of a transformation.
type TransformResponse struct {
	Data     []byte                 `json:"data"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TransformationInfo describes an available transformation.
type TransformationInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  []ParameterInfo        `json:"parameters,omitempty"`
}

// ParameterInfo describes a transformation parameter.
type ParameterInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"description,omitempty"`
}