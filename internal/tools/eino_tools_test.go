package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThinkTool_Info(t *testing.T) {
	tool := NewThinkTool()
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "think", info.Name)
	assert.Contains(t, info.Desc, "Record thoughts")
	assert.NotNil(t, info.ParamsOneOf)
}

func TestThinkTool_InvokableRun(t *testing.T) {
	tool := NewThinkTool()
	ctx := context.Background()

	t.Run("successful thought recording", func(t *testing.T) {
		args := map[string]interface{}{
			"thought": "I need to analyze this problem carefully",
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.NoError(t, err)
		assert.Contains(t, result, "I need to analyze this problem carefully")
	})

	t.Run("missing thought parameter", func(t *testing.T) {
		args := map[string]interface{}{}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "thought")
	})

	t.Run("empty thought", func(t *testing.T) {
		args := map[string]interface{}{
			"thought": "   ",
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		_, err := tool.InvokableRun(ctx, "invalid json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parse arguments")
	})
}

func TestFileReadTool_Info(t *testing.T) {
	tool := NewFileReadTool()
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "read_file", info.Name)
	assert.Contains(t, info.Desc, "Read the contents")
	assert.NotNil(t, info.ParamsOneOf)
}

func TestFileReadTool_InvokableRun(t *testing.T) {
	tool := NewFileReadTool()
	ctx := context.Background()

	// Create a test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, Eino World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	t.Run("successful file read", func(t *testing.T) {
		args := map[string]interface{}{
			"path": testFile,
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.NoError(t, err)
		assert.Equal(t, testContent, result)
	})

	t.Run("file not found", func(t *testing.T) {
		args := map[string]interface{}{
			"path": filepath.Join(tempDir, "nonexistent.txt"),
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("missing path parameter", func(t *testing.T) {
		args := map[string]interface{}{}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})
}

func TestFileWriteTool_Info(t *testing.T) {
	tool := NewFileWriteTool()
	ctx := context.Background()

	info, err := tool.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "write_file", info.Name)
	assert.Contains(t, info.Desc, "Write content")
	assert.NotNil(t, info.ParamsOneOf)
}

func TestFileWriteTool_InvokableRun(t *testing.T) {
	tool := NewFileWriteTool()
	ctx := context.Background()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "output.txt")
	testContent := "Hello from Eino!"

	t.Run("successful file write", func(t *testing.T) {
		args := map[string]interface{}{
			"path":    testFile,
			"content": testContent,
		}
		argsJSON, _ := json.Marshal(args)

		result, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.NoError(t, err)
		assert.Contains(t, result, "Successfully wrote")

		// Verify file was written
		content, err := os.ReadFile(testFile)
		assert.NoError(t, err)
		assert.Equal(t, testContent, string(content))
	})

	t.Run("missing path parameter", func(t *testing.T) {
		args := map[string]interface{}{
			"content": testContent,
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path")
	})

	t.Run("missing content parameter", func(t *testing.T) {
		args := map[string]interface{}{
			"path": testFile,
		}
		argsJSON, _ := json.Marshal(args)

		_, err := tool.InvokableRun(ctx, string(argsJSON))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content")
	})
}

func TestParseJSONArgs(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		jsonStr := `{"key1": "value1", "key2": 123}`
		args, err := parseJSONArgs(jsonStr)
		assert.NoError(t, err)
		assert.Equal(t, "value1", args["key1"])
		assert.Equal(t, float64(123), args["key2"]) // JSON numbers become float64
	})

	t.Run("invalid JSON", func(t *testing.T) {
		jsonStr := `{"key1": "value1", "key2":}`
		_, err := parseJSONArgs(jsonStr)
		assert.Error(t, err)
	})

	t.Run("empty JSON", func(t *testing.T) {
		jsonStr := `{}`
		args, err := parseJSONArgs(jsonStr)
		assert.NoError(t, err)
		assert.Empty(t, args)
	})
}
