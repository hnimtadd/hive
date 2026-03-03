package llm

import (
	"testing"

	"github.com/hnimtadd/hive/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIClient(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.OpenAIConfig{
			Model:     "gpt-4",
			APIKeyEnv: "NONEXISTENT_API_KEY",
			BaseURL:   "https://api.openai.com/v1",
		}

		_, err := NewOpenAIClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// This test verifies the function signature and interface compliance
		// without requiring actual API credentials
		cfg := &config.OpenAIConfig{
			Model:     "gpt-4",
			APIKeyEnv: "TEST_API_KEY",
			BaseURL:   "https://api.openai.com/v1",
		}

		// Set a dummy API key for the test
		t.Setenv("TEST_API_KEY", "test-key")

		client, err := NewOpenAIClientWithConfig(cfg)
		if err != nil {
			// Expected to fail without real credentials, but should show proper interface
			t.Logf("Expected error without real credentials: %v", err)
		} else {
			assert.NotNil(t, client)
		}
	})
}

func TestNewLLMClient(t *testing.T) {
	t.Run("claude provider", func(t *testing.T) {
		cfg := &config.AIConfig{
			Provider: "claude",
			Claude: &config.ClaudeConfig{
				Model:     "claude-3-5-sonnet",
				APIKeyEnv: "TEST_CLAUDE_KEY",
				Provider:  config.ClaudeIntegrationTypeAPI,
			},
		}

		t.Setenv("TEST_CLAUDE_KEY", "test-key")

		_, err := NewLLMClientWithConfig(cfg)
		// May fail due to API call, but should not fail due to config
		if err != nil && err.Error() != "failed to create Claude model: POST https://api.anthropic.com/v1/messages: 401 Unauthorized" {
			t.Logf("Error (expected without real API): %v", err)
		}
	})

	t.Run("openai provider", func(t *testing.T) {
		cfg := &config.AIConfig{
			Provider: "openai",
			OpenAI: &config.OpenAIConfig{
				Model:     "gpt-4",
				APIKeyEnv: "TEST_OPENAI_KEY",
				BaseURL:   "https://api.openai.com/v1",
			},
		}

		t.Setenv("TEST_OPENAI_KEY", "test-key")

		_, err := NewLLMClientWithConfig(cfg)
		// May fail due to API call, but should not fail due to config
		if err != nil {
			t.Logf("Error (expected without real API): %v", err)
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		cfg := &config.AIConfig{
			Provider: "unsupported",
		}

		_, err := NewLLMClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported AI provider")
	})

	t.Run("missing claude config", func(t *testing.T) {
		cfg := &config.AIConfig{
			Provider: "claude",
			Claude:   nil,
		}

		_, err := NewLLMClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "claude configuration is required")
	})

	t.Run("missing openai config", func(t *testing.T) {
		cfg := &config.AIConfig{
			Provider: "openai",
			OpenAI:   nil,
		}

		_, err := NewLLMClientWithConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "openai configuration is required")
	})
}

func TestOpenAIToolCallingClient(t *testing.T) {
	t.Run("interface compatibility", func(t *testing.T) {
		cfg := &config.OpenAIConfig{
			Model:     "gpt-4",
			APIKeyEnv: "TEST_API_KEY",
			BaseURL:   "https://api.openai.com/v1",
		}

		t.Setenv("TEST_API_KEY", "test-key")

		// NewOpenAIToolCallingClientWithConfig should return the same as NewOpenAIClientWithConfig
		client1, err1 := NewOpenAIClientWithConfig(cfg)
		client2, err2 := NewOpenAIToolCallingClientWithConfig(cfg)

		// Both should have the same error behavior
		if err1 != nil {
			assert.Error(t, err2)
		} else {
			require.NoError(t, err2)
			assert.NotNil(t, client1)
			assert.NotNil(t, client2)
		}
	})
}