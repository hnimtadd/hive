package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// ThinkTool implements Eino's InvokableTool interface for reasoning
type ThinkTool struct{}

func NewThinkTool() *ThinkTool {
	return &ThinkTool{}
}

func (t *ThinkTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "think",
		Desc: "Record thoughts, reasoning, or analysis. Use this to process information, plan next steps, or reflect on previous actions.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "thought",
					Value: &jsonschema.Schema{
						Type:        "string",
						Description: "The thought or reasoning to record",
					},
				},
			)),
			Required: []string{"thought"},
		}),
	}, nil
}

func (t *ThinkTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse JSON arguments
	args, err := parseJSONArgs(argumentsInJSON)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	thought, ok := args["thought"].(string)
	if !ok || strings.TrimSpace(thought) == "" {
		return "", fmt.Errorf("'thought' parameter is required and must be a non-empty string")
	}

	return fmt.Sprintf("Recorded thought: %s", thought), nil
}

// FileReadTool implements Eino's InvokableTool interface for reading files
type FileReadTool struct{}

func NewFileReadTool() *FileReadTool {
	return &FileReadTool{}
}

func (t *FileReadTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_file",
		Desc: "Read the contents of a file from the filesystem.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "path",
					Value: &jsonschema.Schema{
						Type:        "string",
						Description: "The file path to read",
					},
				},
			)),
			Required: []string{"path"},
		}),
	}, nil
}

func (t *FileReadTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	args, err := parseJSONArgs(argumentsInJSON)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("'path' parameter is required and must be a non-empty string")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// FileWriteTool implements Eino's InvokableTool interface for writing files
type FileWriteTool struct{}

func NewFileWriteTool() *FileWriteTool {
	return &FileWriteTool{}
}

func (t *FileWriteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "write_file",
		Desc: "Write content to a file on the filesystem.",
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&jsonschema.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](orderedmap.WithInitialData[string, *jsonschema.Schema](
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "path",
					Value: &jsonschema.Schema{
						Type:        "string",
						Description: "The file path to write to",
					},
				},
				orderedmap.Pair[string, *jsonschema.Schema]{
					Key: "content",
					Value: &jsonschema.Schema{
						Type:        "string",
						Description: "The content to write to the file",
					},
				},
			)),
			Required: []string{"path", "content"},
		}),
	}, nil
}

func (t *FileWriteTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	args, err := parseJSONArgs(argumentsInJSON)
	if err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("'path' parameter is required and must be a non-empty string")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("'content' parameter is required and must be a string")
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path), nil
}

// parseJSONArgs is a helper function to parse JSON arguments
func parseJSONArgs(argumentsInJSON string) (map[string]interface{}, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return nil, err
	}
	return args, nil
}
