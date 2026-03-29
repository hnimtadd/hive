package hive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/eino-contrib/jsonschema"
	"github.com/hnimtadd/hive/pkg/secret"
)

// Tool is a function-based tool that serves itself via stdin/stdout.
// It reads HiveRequest from stdin and writes HiveResponse to stdout.
type Tool[I, O any] struct {
	name        string
	description string
	schema      *jsonschema.Schema
	handler     func(ctx context.Context, input I) (O, error)
	timeout     time.Duration
	reader      io.Reader
	writer      io.Writer

	secret []secret.Requirement
}

// NewTool creates a new tool with automatic schema inference from input type I.
// The tool can be run with Serve() to handle requests via stdin/stdout.
func NewTool[I, O any](
	name string,
	description string,
	handler func(context.Context, I) (O, error),
	opts ...ToolOption[I, O],
) (*Tool[I, O], error) {
	toolInfo, err := utils.GoStruct2ToolInfo[I](name, description)
	if err != nil {
		return nil, fmt.Errorf("failed to infer type: %w", err)
	}
	schema, err := toolInfo.ToJSONSchema()
	if err != nil {
		return nil, fmt.Errorf("failed to infer schema: %w", err)
	}

	tool := &Tool[I, O]{
		name:        name,
		description: description,
		schema:      schema,
		handler:     handler,
		timeout:     30 * time.Second,
		reader:      os.Stdin,
		writer:      os.Stdout,
		secret:      []secret.Requirement{},
	}
	for _, opt := range opts {
		tool, err = opt(tool)
		if err != nil {
			return nil, err
		}
	}
	return tool, nil
}

type ToolOption[I, O any] func(t *Tool[I, O]) (*Tool[I, O], error)

// WithTimeout sets a custom timeout for request handling.
func WithTimeout[I, O any](timeout time.Duration) ToolOption[I, O] {
	return func(t *Tool[I, O]) (*Tool[I, O], error) {
		if timeout.Seconds() == 0 {
			return nil, errors.New("tool timeout must be larger than 0")
		}
		t.timeout = timeout
		return t, nil
	}
}

func WithSecret[I, O any](ptr any) ToolOption[I, O] {
	return func(t *Tool[I, O]) (*Tool[I, O], error) {
		var reqs []secret.Requirement
		v := reflect.ValueOf(ptr)
		if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
			return nil, errors.New("input must be a pointer to a struct")
		}
		structVal := v.Elem()
		structType := structVal.Type()
		if structType.Kind() == reflect.Ptr {
			structType = structType.Elem()
		}
		for i := range structType.NumField() {
			field := structType.Field(i)
			key, defined := field.Tag.Lookup("hive")
			if !defined {
				continue
			}

			tag := parseHiveTag(key)
			if tag.Key == "" {
				return nil, fmt.Errorf("env key is not defined for: %s", field.Name)
			}

			if key != "" && !strings.Contains(key, ",") {
				reqs = append(reqs, secret.Requirement{
					Key:         tag.Key,
					Description: tag.Description,
					Required:    !tag.OmitEmpty,
				})
			}
		}
		t.secret = reqs
		return t, nil
	}
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
		"parameters":  t.schema,
		"entrypoint":  []string{"go", "run", "."},
		"runtime":     "hive",
		"timeout":     int(t.timeout.Seconds()),
		"secret":      t.secret,
	})
}
