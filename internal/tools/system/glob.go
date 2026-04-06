package system

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

func GlobTool() (tool.InvokableTool, error) {
	return utils.InferTool("glob", "a pattern-matching utility used to find files and directories whose names match a specific pattern", glob)
}

type GlobInput struct {
	Query string `json:"query" jsonschema:"required" jsonschema_description:"Search keyword"`
}

func glob(_ context.Context, input *GlobInput) (*schema.ToolResult, error) {
	matches, err := filepath.Glob(input.Query)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to run glob: %s", err)},
			},
		}, nil
	}
	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{Type: schema.ToolPartTypeText, Text: strings.Join(matches, "\n")},
		},
	}, nil
}
