package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ListFilesTool lists files and directories
type ListFilesTool struct {
	baseDir string
}

// Info implements [tool.InvokableTool].
func (t *ListFilesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_files",
		Desc: "List files and directories in a given path. Returns file names, sizes, and types (file/directory).",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"path": {
					Type:     schema.String,
					Desc:     "Path to list (relative or absolute). Use '.' for current directory",
					Required: true,
				},
				"recursive": {
					Type:     schema.Boolean,
					Desc:     "List files recursively (default: false)",
					Required: false,
				},
			},
		),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (t *ListFilesTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive *bool  `json:"recursive,omitempty"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	recursive := false
	if args.Recursive != nil {
		recursive = *args.Recursive
	}

	// Resolve path
	fullPath := args.Path
	if !filepath.IsAbs(fullPath) {
		if t.baseDir != "" {
			fullPath = filepath.Join(t.baseDir, fullPath)
		} else {
			var err error
			fullPath, err = filepath.Abs(fullPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve path: %w", err)
			}
		}
	}

	var entries []map[string]any

	if recursive {
		// Recursive listing
		err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip the root directory itself
			if path == fullPath {
				return nil
			}

			relPath, err := filepath.Rel(fullPath, path)
			if err != nil {
				return err
			}

			entryType := "file"
			if info.IsDir() {
				entryType = "directory"
			}

			entries = append(entries, map[string]any{
				"name": relPath,
				"type": entryType,
				"size": info.Size(),
			})

			return nil
		})

		if err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// Non-recursive listing
		dirEntries, err := os.ReadDir(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range dirEntries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			entryType := "file"
			if entry.IsDir() {
				entryType = "directory"
			}

			entries = append(entries, map[string]any{
				"name": entry.Name(),
				"type": entryType,
				"size": info.Size(),
			})
		}
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"path":      args.Path,
		"full_path": fullPath,
		"count":     len(entries),
		"entries":   entries,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// NewListFilesTool creates a new list files tool
func NewListFilesTool(baseDir string) tool.InvokableTool {
	return &ListFilesTool{
		baseDir: baseDir,
	}
}

// LocalFileReadTool reads local files (enhanced version of FileReadTool)
type LocalFileReadTool struct {
	baseDir string
}

// Info implements [tool.InvokableTool].
func (t *LocalFileReadTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_local_file",
		Desc: "Read the contents of a local file from the filesystem. Returns the complete file content.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"path": {
					Type:     schema.String,
					Desc:     "File path to read (relative or absolute)",
					Required: true,
				},
			},
		),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (t *LocalFileReadTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	fullPath := args.Path
	if !filepath.IsAbs(fullPath) {
		if t.baseDir != "" {
			fullPath = filepath.Join(t.baseDir, fullPath)
		} else {
			var err error
			fullPath, err = filepath.Abs(fullPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve path: %w", err)
			}
		}
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", args.Path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"path":      args.Path,
		"full_path": fullPath,
		"content":   string(content),
		"size":      len(content),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// NewLocalFileReadTool creates a new local file read tool
func NewLocalFileReadTool(baseDir string) tool.InvokableTool {
	return &LocalFileReadTool{
		baseDir: baseDir,
	}
}

// LocalFileWriteTool writes to local files (enhanced version of FileWriteTool)
type LocalFileWriteTool struct {
	baseDir string
}

// Info implements [tool.InvokableTool].
func (t *LocalFileWriteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "write_local_file",
		Desc: "Write content to a local file on the filesystem. Creates parent directories if needed. Overwrites existing files.",
		ParamsOneOf: schema.NewParamsOneOfByParams(
			map[string]*schema.ParameterInfo{
				"path": {
					Type:     schema.String,
					Desc:     "File path to write to (relative or absolute)",
					Required: true,
				},
				"content": {
					Type:     schema.String,
					Desc:     "Content to write to the file",
					Required: true,
				},
			},
		),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (t *LocalFileWriteTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}

	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if args.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	fullPath := args.Path
	if !filepath.IsAbs(fullPath) {
		if t.baseDir != "" {
			fullPath = filepath.Join(t.baseDir, fullPath)
		} else {
			var err error
			fullPath, err = filepath.Abs(fullPath)
			if err != nil {
				return "", fmt.Errorf("failed to resolve path: %w", err)
			}
		}
	}

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	result, err := json.Marshal(map[string]any{
		"status":    "success",
		"path":      args.Path,
		"full_path": fullPath,
		"size":      len(args.Content),
		"message":   fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(result), nil
}

// NewLocalFileWriteTool creates a new local file write tool
func NewLocalFileWriteTool(baseDir string) tool.InvokableTool {
	return &LocalFileWriteTool{
		baseDir: baseDir,
	}
}
