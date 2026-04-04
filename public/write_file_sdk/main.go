package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hnimtadd/hive/pkg/hive"
)

type WriteFileInput struct {
	Path    string `json:"path" jsonschema:"description=Path to the file to write (absolute or relative)"`
	Content string `json:"content" jsonschema:"description=Content to write to the file"`
	Append  bool   `json:"append" jsonschema:"description=If true, append to file instead of overwriting (default: false)"`
}

type WriteFileOutput struct {
	Path        string `json:"path" jsonschema:"description=Absolute path of the file written"`
	BytesWritten int    `json:"bytes_written" jsonschema:"description=Number of bytes written"`
	Created     bool   `json:"created" jsonschema:"description=Whether the file was newly created"`
}

func writeFile(ctx context.Context, input WriteFileInput) (WriteFileOutput, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(input.Path)
	if err != nil {
		return WriteFileOutput{}, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists before writing
	_, err = os.Stat(absPath)
	created := os.IsNotExist(err)

	// Ensure parent directory exists
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return WriteFileOutput{}, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Determine file mode
	var flags int
	if input.Append {
		flags = os.O_CREATE | os.O_APPEND | os.O_WRONLY
	} else {
		flags = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	}

	// Open/create file
	file, err := os.OpenFile(absPath, flags, 0644)
	if err != nil {
		return WriteFileOutput{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Write content
	n, err := file.WriteString(input.Content)
	if err != nil {
		return WriteFileOutput{}, fmt.Errorf("failed to write to file: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		return WriteFileOutput{}, fmt.Errorf("failed to sync file: %w", err)
	}

	return WriteFileOutput{
		Path:        absPath,
		BytesWritten: n,
		Created:     created,
	}, nil
}

func main() {
	tool, err := hive.NewTool(
		"write_file",
		"Write content to a file on the filesystem (creates file and parent directories if needed)",
		writeFile,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tool: %v\n", err)
		os.Exit(1)
	}

	tool.Serve()
}
