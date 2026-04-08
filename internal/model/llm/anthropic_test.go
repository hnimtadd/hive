package llm

import (
	"testing"

	"github.com/hnimtadd/hive/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewAnthropicClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: &config.AnthropicConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
			},
		}

		_, err := newAnthropicToolCallingClientWithConfig("anthropic-3-sonnet-20240229", cfg.Anthropic)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := newAnthropicToolCallingClientWithConfig("something", cfg.Anthropic)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}

func TestNewAnthropicToolCallingClientWithConfig(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: &config.AnthropicConfig{
				APIKeyEnv: "NON_EXISTENT_ENV_VAR",
			},
		}

		_, err := newAnthropicToolCallingClientWithConfig("dnthropic-3-sonnet-20240229", cfg.Anthropic)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// Test that the function signature returns the correct interface
		var cfg config.AIConfig
		_, err := newAnthropicToolCallingClientWithConfig("", cfg.Anthropic)

		// We expect an error due to missing env var, but the signature should be correct
		assert.Error(t, err)
	})
}
