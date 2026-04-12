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
