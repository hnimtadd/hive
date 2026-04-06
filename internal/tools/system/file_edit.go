package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

func FileEditTool() (tool.InvokableTool, error) {
	return utils.InferTool("file_edit", "edit file contents by replacing, appending, prepending, or inserting text", fileEdit)
}

type FileEditInput struct {
	Path        string `json:"path"          jsonschema:"required" jsonschema_description:"Path to the file to edit"`
	Operation   string `json:"operation"     jsonschema:"required" jsonschema_description:"Edit operation: 'replace', 'append', 'prepend', 'insert_at_line', 'replace_line', 'delete_line', 'regex_replace'"`
	Content     string `json:"content"                             jsonschema_description:"Content to write/insert (required for replace, append, prepend, insert_at_line, replace_line, regex_replace)"`
	OldContent  string `json:"old_content"                         jsonschema_description:"Content to replace (required for replace operation)"`
	LineNumber  int    `json:"line_number"                         jsonschema_description:"Line number for insert_at_line, replace_line, or delete_line operations (1-indexed)"`
	Pattern     string `json:"pattern"                             jsonschema_description:"Regex pattern for regex_replace operation"`
	ReplaceAll  bool   `json:"replace_all"                         jsonschema_description:"Replace all occurrences for replace operation (default: false)"`
	CreateIfNew bool   `json:"create_if_new"                       jsonschema_description:"Create file if it doesn't exist (default: false)"`
}

func fileEdit(_ context.Context, input *FileEditInput) (*schema.ToolResult, error) {
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
	_, err = os.Stat(absPath)
	fileExists := err == nil

	if !fileExists && !input.CreateIfNew {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: "File does not exist. Set create_if_new=true to create it."},
			},
		}, nil
	}

	var content string
	if fileExists {
		var data []byte
		data, err = os.ReadFile(absPath)
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to read file: %s", err)},
				},
			}, nil
		}
		content = string(data)
	}

	var newContent string
	var opResult string

	switch input.Operation {
	case "replace":
		if input.OldContent == "" {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: "old_content is required for replace operation"},
				},
			}, nil
		}
		if input.ReplaceAll {
			newContent = strings.ReplaceAll(content, input.OldContent, input.Content)
			count := strings.Count(content, input.OldContent)
			opResult = fmt.Sprintf("Replaced %d occurrence(s)", count)
		} else {
			newContent = strings.Replace(content, input.OldContent, input.Content, 1)
			if content != newContent {
				opResult = "Replaced 1 occurrence"
			} else {
				opResult = "No matches found"
			}
		}

	case "append":
		newContent = content + input.Content
		opResult = "Content appended to file"

	case "prepend":
		newContent = input.Content + content
		opResult = "Content prepended to file"

	case "insert_at_line":
		if input.LineNumber <= 0 {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: "line_number must be greater than 0"},
				},
			}, nil
		}
		lines := strings.Split(content, "\n")
		if input.LineNumber > len(lines)+1 {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("line_number %d exceeds file length %d", input.LineNumber, len(lines))},
				},
			}, nil
		}
		// Insert at line (1-indexed)
		if input.LineNumber == len(lines)+1 {
			lines = append(lines, input.Content)
		} else {
			idx := input.LineNumber - 1
			lines = append(lines[:idx], append([]string{input.Content}, lines[idx:]...)...)
		}
		newContent = strings.Join(lines, "\n")
		opResult = fmt.Sprintf("Content inserted at line %d", input.LineNumber)

	case "replace_line":
		if input.LineNumber <= 0 {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: "line_number must be greater than 0"},
				},
			}, nil
		}
		lines := strings.Split(content, "\n")
		if input.LineNumber > len(lines) {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("line_number %d exceeds file length %d", input.LineNumber, len(lines))},
				},
			}, nil
		}
		lines[input.LineNumber-1] = input.Content
		newContent = strings.Join(lines, "\n")
		opResult = fmt.Sprintf("Line %d replaced", input.LineNumber)

	case "delete_line":
		if input.LineNumber <= 0 {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: "line_number must be greater than 0"},
				},
			}, nil
		}
		lines := strings.Split(content, "\n")
		if input.LineNumber > len(lines) {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("line_number %d exceeds file length %d", input.LineNumber, len(lines))},
				},
			}, nil
		}
		lines = append(lines[:input.LineNumber-1], lines[input.LineNumber:]...)
		newContent = strings.Join(lines, "\n")
		opResult = fmt.Sprintf("Line %d deleted", input.LineNumber)

	case "regex_replace":
		if input.Pattern == "" {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: "pattern is required for regex_replace operation"},
				},
			}, nil
		}
		var re *regexp.Regexp
		re, err = regexp.Compile(input.Pattern)
		if err != nil {
			return &schema.ToolResult{
				Parts: []schema.ToolOutputPart{
					{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Invalid regex pattern: %s", err)},
				},
			}, nil
		}
		newContent = re.ReplaceAllString(content, input.Content)
		if content != newContent {
			opResult = "Regex replacement completed"
		} else {
			opResult = "No matches found for regex pattern"
		}

	default:
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{
					Type: schema.ToolPartTypeText,
					Text: fmt.Sprintf("Unknown operation: %s. Valid operations are: replace, append, prepend, insert_at_line, replace_line, delete_line, regex_replace", input.Operation),
				},
			},
		}, nil
	}

	// Write the new content
	err = os.WriteFile(absPath, []byte(newContent), 0600)
	if err != nil {
		return &schema.ToolResult{
			Parts: []schema.ToolOutputPart{
				{Type: schema.ToolPartTypeText, Text: fmt.Sprintf("Failed to write file: %s", err)},
			},
		}, nil
	}

	return &schema.ToolResult{
		Parts: []schema.ToolOutputPart{
			{Type: schema.ToolPartTypeText, Text: opResult},
		},
	}, nil
}
