package system

import (
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

// Tools returns all system tools in a map.
func Tools() (map[string]tool.InvokableTool, error) {
	tools := make(map[string]tool.InvokableTool)

	globTool, err := GlobTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create glob tool: %w", err)
	}
	tools["glob"] = globTool

	grepTool, err := GrepTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create grep tool: %w", err)
	}
	tools["grep"] = grepTool

	fileReadTool, err := FileReadTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create file_read tool: %w", err)
	}
	tools["file_read"] = fileReadTool

	fileWriteTool, err := FileWriteTool()
	if err != nil {
		return nil, fmt.Errorf("failed to create file_edit tool: %w", err)
	}
	tools["file_write"] = fileWriteTool

	return tools, nil
}
