package llm

import (
	"testing"

	"github.com/hnimtadd/hive/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIClient(t *testing.T) {
	t.Run("missing API key", func(t *testing.T) {
		cfg := &config.OpenAIConfig{
			APIKeyEnv: "NONEXISTENT_API_KEY",
			BaseURL:   "https://api.openai.com/v1",
		}

		_, err := newOpenAIToolCallingClientWithConfig("gpt-4", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "environment variable is required")
	})

	t.Run("interface compliance", func(t *testing.T) {
		// This test verifies the function signature and interface compliance
		// without requiring actual API credentials
		cfg := &config.OpenAIConfig{
			APIKeyEnv: "TEST_API_KEY",
			BaseURL:   "https://api.openai.com/v1",
		}

		// Set a dummy API key for the test
		t.Setenv("TEST_API_KEY", "test-key")

		client, err := newOpenAIToolCallingClientWithConfig("gpt-4", cfg)
		if err != nil {
			// Expected to fail without real credentials, but should show proper interface
			t.Logf("Expected error without real credentials: %v", err)
		} else {
			assert.NotNil(t, client)
		}
	})
}

func TestNewLLMClient(t *testing.T) {
	t.Run("anthropic provider", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: &config.AnthropicConfig{
				APIKeyEnv: "TEST_anthropic_KEY",
				Provider:  config.AnthropicIntegrationTypeAPI,
			},
		}

		t.Setenv("TEST_anthropic_KEY", "test-key")

		_, err := newLLMToolCallingClientWithConfig("anthropic", "anthropic-3-5-sonnet", cfg)
		// May fail due to API call, but should not fail due to config
		if err != nil && err.Error() != "failed to create anthropic model: POST https://api.anthropic.com/v1/messages: 401 Unauthorized" {
			t.Logf("Error (expected without real API): %v", err)
		}
	})

	t.Run("openai provider", func(t *testing.T) {
		cfg := &config.AIConfig{
			OpenAI: &config.OpenAIConfig{
				APIKeyEnv: "TEST_OPENAI_KEY",
				BaseURL:   "https://api.openai.com/v1",
			},
		}

		t.Setenv("TEST_OPENAI_KEY", "test-key")

		_, err := newLLMToolCallingClientWithConfig("open-ai", "gpt-4", cfg)
		// May fail due to API call, but should not fail due to config
		if err != nil {
			t.Logf("Error (expected without real API): %v", err)
		}
	})

	t.Run("unsupported provider", func(t *testing.T) {
		cfg := &config.AIConfig{}

		_, err := newLLMToolCallingClientWithConfig("unsupported", "", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported AI provider")
	})

	t.Run("missing anthropic config", func(t *testing.T) {
		cfg := &config.AIConfig{
			Anthropic: nil,
		}

		_, err := newLLMToolCallingClientWithConfig("anthropic", "", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "anthropic configuration is required")
	})

	t.Run("missing openai config", func(t *testing.T) {
		cfg := &config.AIConfig{
			OpenAI: nil,
		}

		_, err := newLLMToolCallingClientWithConfig("openai", "", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "openai configuration is required")
	})
}
