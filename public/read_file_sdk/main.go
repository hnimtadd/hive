package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hnimtadd/hive/pkg/hive"
)

type ReadFileInput struct {
	Path string `json:"path" jsonschema:"description=Path to the file to read (absolute or relative)"`
}

type ReadFileOutput struct {
	Content  string `json:"content" jsonschema:"description=The file contents"`
	Path     string `json:"path" jsonschema:"description=Absolute path of the file read"`
	Size     int64  `json:"size" jsonschema:"description=File size in bytes"`
	IsText   bool   `json:"is_text" jsonschema:"description=Whether the file is text (not binary)"`
}

func readFile(ctx context.Context, input ReadFileInput) (ReadFileOutput, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(input.Path)
	if err != nil {
		return ReadFileOutput{}, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Get file info
	info, err := os.Stat(absPath)
	if err != nil {
		return ReadFileOutput{}, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's a directory
	if info.IsDir() {
		return ReadFileOutput{}, fmt.Errorf("path is a directory, not a file: %s", absPath)
	}

	// Check file size (limit to 10MB for safety)
	const maxSize = 10 * 1024 * 1024 // 10MB
	if info.Size() > maxSize {
		return ReadFileOutput{}, fmt.Errorf("file too large: %d bytes (max %d bytes)", info.Size(), maxSize)
	}

	// Read file contents
	content, err := os.ReadFile(absPath)
	if err != nil {
		return ReadFileOutput{}, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if it's likely text (no null bytes in first 512 bytes)
	isText := isLikelyText(content)

	if !isText {
		return ReadFileOutput{}, fmt.Errorf("file appears to be binary (not text): %s", absPath)
	}

	return ReadFileOutput{
		Content: string(content),
		Path:    absPath,
		Size:    info.Size(),
		IsText:  isText,
	}, nil
}

// isLikelyText checks if content is likely text by looking for null bytes
func isLikelyText(data []byte) bool {
	// Check first 512 bytes (or full content if smaller)
	checkLen := 512
	if len(data) < checkLen {
		checkLen = len(data)
	}

	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return false // Null byte indicates binary
		}
	}
	return true
}

func main() {
	tool, err := hive.NewTool(
		"read_file",
		"Read contents of a text file from the filesystem",
		readFile,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tool: %v\n", err)
		os.Exit(1)
	}

	tool.Serve()
}
