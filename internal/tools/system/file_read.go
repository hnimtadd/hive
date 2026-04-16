package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

func FileReadTool() (tool.InvokableTool, error) {
	return utils.InferTool("file_read", "read the contents of a file", fileRead)
}

type FileReadInput struct {
	Path   string `json:"path"   jsonschema:"required" jsonschema_description:"Path to the file to read"`
	Offset int64  `json:"offset"                       jsonschema_description:"Byte offset to start reading from (default: 0)"`
	Limit  int64  `json:"limit"                        jsonschema_description:"Maximum number of bytes to read (default: 0 for entire file)"`
}

func fileRead(_ context.Context, input *FileReadInput) (*schema.ToolResult, error) {
	// Validate input
	if input == nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "ERROR: file_read received nil input"},
			},
		}, nil
	}
	if input.Path == "" {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "ERROR: file path is required"},
			},
		}, nil
	}

	// Resolve path
	absPath, err := filepath.Abs(input.Path)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to resolve path: %s", err)},
			},
		}, nil
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to access file: %s", err)},
			},
		}, nil
	}

	if info.IsDir() {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "Path is a directory, not a file"},
			},
		}, nil
	}

	// Open file
	file, err := os.Open(absPath)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to open file: %s", err)},
			},
		}, nil
	}
	defer file.Close()

	// Seek to offset if specified
	if input.Offset > 0 {
		_, err = file.Seek(input.Offset, 0)
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to seek to offset: %s", err)},
				},
			}, nil
		}
	}

	// Read file content
	var content []byte
	if input.Limit > 0 {
		content = make([]byte, input.Limit)
		n, err := file.Read(content)
		if err != nil && err.Error() != "EOF" {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to read file: %s", err)},
				},
			}, nil
		}
		content = content[:n]
	} else {
		content, err = os.ReadFile(absPath)
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to read file: %s", err)},
				},
			}, nil
		}
	}

	if !utf8.ValidString(string(content)) {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "File is not a text file, not readable"},
			},
		}, nil
	}

	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{Type: schema.ToolPartTypeText, Text: string(content)},
		},
	}, nil
}
