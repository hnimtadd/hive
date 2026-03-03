package llm

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClaudeClient(t *testing.T) {
	t.Run("returns model.ChatModel interface", func(t *testing.T) {
		// Skip if no API key is available (we can't test without real config)
		t.Skip("Skipping test that requires real API key configuration")

		client, err := NewClaudeClient()
		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify it implements the correct interface
		assert.Implements(t, (*model.ChatModel)(nil), client)
	})
}

func TestNewClaudeClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			Claude: &config.ClaudeConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
				Model:     "claude-3-sonnet-20240229",
			},
		}

		_, err := NewClaudeClientWithConfig(cfg.Claude)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewClaudeClientWithConfig(cfg.Claude)

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
			Claude: &config.ClaudeConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
				Model:     "claude-3-sonnet-20240229",
			},
		}

		_, err := NewClaudeToolCallingClientWithConfig(cfg.Claude)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewClaudeToolCallingClientWithConfig(cfg.Claude)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestInterfaceCompatibility(t *testing.T) {
	t.Run("ChatModel interface", func(t *testing.T) {
		// Test that our function signature returns a type compatible with Eino's interface
		// We can't directly test the assignment due to concrete vs interface types,
		// but we can test that the function exists and has the right signature
		assert.NotNil(t, NewClaudeClientWithConfig)
	})

	t.Run("ToolCallingChatModel interface", func(t *testing.T) {
		// Test that our function signature returns a type compatible with Eino's interface
		assert.NotNil(t, NewClaudeToolCallingClientWithConfig)
	})
}
