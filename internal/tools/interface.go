package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
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
	config Config
}

func NewHiveTool(config Config) tool.InvokableTool {
	return hiveTool{config: config}
}

// Info implements [tool.InvokableTool].
func (h hiveTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        h.config.Name,
		Desc:        h.config.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(jsonschema.Reflect(h.config.Parameters)),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (h hiveTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	switch h.config.Runtime {
	case "native":
		ctx, cancel := context.WithTimeout(ctx, time.Duration(h.config.TimeoutInSec)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, h.config.Entrypoint)
		cmd.Stdin = bytes.NewReader([]byte(argumentsInJSON))
		soR, soW := io.Pipe()
		cmd.Stdout = soW
		seR, seW := io.Pipe()
		cmd.Stderr = seW
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to execute tools: %w", err)
		}
		outputBytes, err := io.ReadAll(soR)
		if err != nil {
			return "", fmt.Errorf("failed to read stdout output: %w", err)
		}
		debugBytes, err := io.ReadAll(seR)
		if err != nil {
			return "", fmt.Errorf("failed to read stderr output: %w", err)
		}
		log.Printf("Tool debug: %s\n", string(debugBytes))
		return string(outputBytes), nil

	default:
		return "", fmt.Errorf("Not supported runtime")
	}
}
