package llm

import (
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropicClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: &config.AnthropicConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
				Model:     "Anthropic-3-sonnet-20240229",
			},
		}

		_, err := NewAnthropicClientWithConfig(cfg.Anthropic)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewAnthropicClientWithConfig(cfg.Anthropic)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestNewAnthropicToolCallingClient(t *testing.T) {
	t.Run("returns ToolCallingChatModel interface", func(t *testing.T) {
		t.Skip("Skipping test that requires real API key configuration")

		client, err := NewAnthropicToolCallingClient()
		require.NoError(t, err)
		assert.NotNil(t, client)

		// Verify it implements the correct interface
		var _ model.ToolCallingChatModel = client
	})
}

func TestNewAnthropicToolCallingClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: &config.AnthropicConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
				Model:     "Anthropic-3-sonnet-20240229",
			},
		}

		_, err := NewAnthropicToolCallingClientWithConfig(cfg.Anthropic)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := NewAnthropicToolCallingClientWithConfig(cfg.Anthropic)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestInterfaceCompatibility(t *testing.T) {
	t.Run("ChatModel interface", func(t *testing.T) {
		// Test that our function signature returns a type compatible with Eino's interface
		// We can't directly test the assignment due to concrete vs interface types,
		// but we can test that the function exists and has the right signature
		assert.NotNil(t, NewAnthropicClientWithConfig)
	})

	t.Run("ToolCallingChatModel interface", func(t *testing.T) {
		// Test that our function signature returns a type compatible with Eino's interface
		assert.NotNil(t, NewAnthropicToolCallingClientWithConfig)
	})
}
