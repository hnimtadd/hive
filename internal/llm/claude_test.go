ackage llm

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/hnimtadd/hive/pkg/config"
)

func TestNewClaudeClient(t *testing.T) {
	t.Run("returns model.ChatModel interface", func(t *testing.T) {
		// Skip if no API key is available (we can't test without real config)
		t.Skip("Skipping test that requires real API key configuration")

		client, err := NewClaudeClient()
		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify it implements the correct interface
		var _ model.ChatModel = client
	})
}

func TestNewClaudeClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			APIKeyEnv: "NON_EXISTENT_ENV_VAR",
			Model:     "claude-3-sonnet-20240229",
		}

		_, err := NewClaudeToolClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewClaudeToolClientWithConfig(&cfg)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestNewClaudeToolCallingClient(t *testing.T) {
	t.Run("returns ToolCallingChatModel interface", func(t *testing.T) {
		t.Skip("Skipping test that requires real API key configuration")

		client, err := NewClaudeToolCallingClient()
		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify it implements the correct interface
		var _ model.ToolCallingChatModel = client
	})
}

func TestNewClaudeToolCallingClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			APIKeyEnv: "NON_EXISTENT_ENV_VAR",
			Model:     "claude-3-sonnet-20240229",
		}

		_, err := NewClaudeToolCallingClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewClaudeToolCallingClientWithConfig(&cfg)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestInterfaceCompatibility(t *testing.T) {
	t.Run("ChatModel interface", func(t *testing.T) {
		// Test that our function signature is compatible with Eino's interface
		var f func(*config.AIConfig) (model.ChatModel, error) = NewClaudeToolClientWithConfig
		assert.NotNil(t, f)
	})

	t.Run("ToolCallingChatModel interface", func(t *testing.T) {
		// Test that our function signature is compatible with Eino's interface
		var f func(*config.AIConfig) (model.ToolCallingChatModel, error) = NewClaudeToolCallingClientWithConfig
		assert.NotNil(t, f)
	})
}
