package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

type Config struct {
	Name         string
	Description  string
	Parameters   map[string]any
	Runtime      string
	Entrypoint   string
	TimeoutInSec int

	path string
}

type hiveTool struct {
	config *Config
	schema *jsonschema.Schema
}

func NewHiveTool(config *Config) (tool.InvokableTool, error) {
	schemaBytes, err := json.Marshal(config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool parameters: %w", err)
	}

	schema := &jsonschema.Schema{}
	if err = schema.UnmarshalJSON(schemaBytes); err != nil {
		return nil, fmt.Errorf("failed to parse jsonschema of tool parameters: %w", err)
	}
	return hiveTool{
		config: config,
		schema: schema,
	}, nil
}

// Info implements [tool.InvokableTool].
func (h hiveTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        h.config.Name,
		Desc:        h.config.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(h.schema),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (h hiveTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	switch h.config.Runtime {
	case "native":
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(h.config.TimeoutInSec)*time.Second)
		defer cancel()

		executionPath := filepath.Join(h.config.path, h.config.Entrypoint)
		st, err := os.Stat(executionPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", errors.New("tool didn't exists, install it first")
			}
			return "", fmt.Errorf("failed to find tool: %w", err)
		}
		if st.Mode().Perm()&0100 == 0 {
			return "", errors.New("tools is not executable")
		}
		cmd := exec.CommandContext(ctx, executionPath)
		var stdout, stderr bytes.Buffer
		cmd.Stdin = bytes.NewReader([]byte(argumentsInJSON))
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err = cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to execute tools: %w", err)
		}
		if stderr.Len() > 0 {
			log.Printf("Tool debug: %s\n", stderr.String())
		}
		return stdout.String(), nil

	default:
		return "", fmt.Errorf("not supported runtime: %s", h.config.Runtime)
	}
}
