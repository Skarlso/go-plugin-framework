package plugins

import (
	"encoding/json"
	"net/http"
)

// Error represents a plugin error response.
type Error struct {
	Message    string `json:"message"`
	StatusCode int    `json:"statusCode"`
}

// NewError creates a new plugin error.
func NewError(err error, statusCode int) *Error {
	return &Error{
		Message:    err.Error(),
		StatusCode: statusCode,
	}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// Write writes the error to the HTTP response.
func (e *Error) Write(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	
	if err := json.NewEncoder(w).Encode(e); err != nil {
		// If we can't encode the error, write a plain text response
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal server error"))
	}
}