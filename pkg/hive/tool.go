package hive

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/eino-contrib/jsonschema"
)

// Tool is a function-based tool that serves itself via stdin/stdout.
// It reads HiveRequest from stdin and writes HiveResponse to stdout.
type Tool[I, O any] struct {
	name        string
	description string
	schema      *jsonschema.Schema
	handler     func(ctx context.Context, input I) (O, error)
	timeout     time.Duration
	reader      *bufio.Reader
	writer      io.Writer
}

// NewTool creates a new tool with automatic schema inference from input type I.
// The tool can be run with Serve() to handle requests via stdin/stdout.
func NewTool[I, O any](
	name string,
	description string,
	handler func(context.Context, I) (O, error),
) (*Tool[I, O], error) {
	toolInfo, err := utils.GoStruct2ToolInfo[I](name, description)
	if err != nil {
		return nil, fmt.Errorf("failed to infer type: %w", err)
	}
	schema, err := toolInfo.ToJSONSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to infer schema: %w", err)
	}

	return &Tool[I, O]{
		name:        name,
		description: description,
		schema:      schema,
		handler:     handler,
		timeout:     30 * time.Second,
		reader:      bufio.NewReader(os.Stdin),
		writer:      os.Stdout,
	}, nil
}

// WithTimeout sets a custom timeout for request handling.
func (t *Tool[I, O]) WithTimeout(timeout time.Duration) *Tool[I, O] {
	t.timeout = timeout
	return t
}

// WithIO sets custom input/output (useful for testing).
func (t *Tool[I, O]) WithIO(reader io.Reader, writer io.Writer) *Tool[I, O] {
	if br, ok := reader.(*bufio.Reader); ok {
		t.reader = br
	} else {
		t.reader = bufio.NewReader(reader)
	}
	t.writer = writer
	return t
}

// Name returns the tool name.
func (t *Tool[I, O]) Name() string {
	return t.name
}

// Description returns the tool description.
func (t *Tool[I, O]) Description() string {
	return t.description
}

// Schema returns the input JSON schema.
func (t *Tool[I, O]) Schema() *jsonschema.Schema {
	return t.schema
}

// Serve starts the tool server, reading requests from stdin and writing responses to stdout.
// It runs until stdin is closed or an unrecoverable error occurs.
func (t *Tool[I, O]) Serve() {
	decoder := json.NewDecoder(t.reader)
	encoder := json.NewEncoder(t.writer)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		if err == io.EOF {
			return // stdin closed, exit cleanly
		}
		resp := &Response{
			Success: false,
			Error:   fmt.Sprintf("decode error: %v", err),
		}
		_ = encoder.Encode(resp)
	}

	resp := t.handle(&req)
	_ = encoder.Encode(resp)
}

func (t *Tool[I, O]) handle(req *Request) *Response {
	if req.Action == "" {
		return req.Error("action is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()

	switch req.Action {
	case "invoke":
		return t.handleInvoke(ctx, req)
	case "inspect":
		return t.handleInspect(req)
	default:
		return req.Error(fmt.Sprintf("unknown action: %s", req.Action))
	}
}

func (t *Tool[I, O]) handleInvoke(ctx context.Context, req *Request) *Response {
	// Extract input
	var input I
	if err := req.ExtractPayload(&input); err != nil {
		return req.Error(fmt.Sprintf("invalid input: %v", err))
	}

	// Call handler
	output, err := t.handler(ctx, input)
	if err != nil {
		return req.Error(fmt.Sprintf("execution failed: %v", err))
	}

	return req.Success(output)
}

func (t *Tool[I, O]) handleInspect(req *Request) *Response {
	return req.Success(map[string]any{
		"name":        t.name,
		"description": t.description,
		"schema":      t.schema,
	})
}

