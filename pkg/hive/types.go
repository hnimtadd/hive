package hive

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Request is the request format for tool invocation.
type Request struct {
	// Action specifies what to do: "invoke" or "inspect"
	Action string `json:"action"`

	// Payload contains the tool input (JSON object)
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Response is the response format for tool invocation.
type Response struct {
	// Success indicates whether execution succeeded
	Success bool `json:"success"`

	// Result contains the output on success (JSON object)
	Result json.RawMessage `json:"result,omitempty"`

	// Error contains error message on failure
	Error string `json:"error,omitempty"`
}

// Success creates a successful response.
func (r *Request) Success(result any) *Response {
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return &Response{Success: false, Error: fmt.Sprintf("marshal error: %v", err)}
		}
		resultJSON = data
	}

	return &Response{
		Success: true,
		Result:  resultJSON,
	}
}

// Error creates an error response.
func (r *Request) Error(message string) *Response {
	return &Response{
		Success: false,
		Error:   message,
	}
}

// ToJSON serializes the response.
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON deserializes a request.
func (r *Request) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// ExtractPayload unmarshals payload into target.
func (r *Request) ExtractPayload(target any) error {
	if r.Payload == nil {
		return errors.New("payload is empty")
	}
	return json.Unmarshal(r.Payload, target)
}

// SetPayload marshals source into payload.
func (r *Request) SetPayload(source any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	r.Payload = data
	return nil
}

// ToolMetadata contains tool information.
type ToolMetadata struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}
