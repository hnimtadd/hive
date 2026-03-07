package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the complete Hive configuration.
type Config struct {
	Redis  RedisConfig  `mapstructure:"redis"`
	AI     AIConfig     `mapstructure:"ai"`
	GitLab GitLabConfig `mapstructure:"gitlab"`
	Jira   JiraConfig   `mapstructure:"jira"`
	Agents AgentsConfig `mapstructure:"agents"`
	Server ServerConfig `mapstructure:"server"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// AIConfig holds AI/LLM configuration.
type AIConfig struct {
	Provider string        `mapstructure:"provider"`
	Claude   *ClaudeConfig `mapstructure:"claude"`
	OpenAI   *OpenAIConfig `mapstructure:"openai"`
}

type ClaudeProvider string

const (
	ClaudeIntegrationTypeBedrock ClaudeProvider = "bedrock"
	ClaudeIntegrationTypeAPI     ClaudeProvider = "api"
)

// ClaudeConfig holds Claude-specific configuration.
type ClaudeConfig struct {
	Provider         ClaudeProvider    `mapstructure:"provider"`
	Model            string            `mapstructure:"model"`
	AnthropicVersion string            `mapstructure:"anthropic_version"`
	BaseURL          string            `mapstructure:"api_base_url"`
	Headers          map[string]string `mapstructure:"api_headers"`
	Region           string            `mapstructure:"bedrock_region"`
	APIKeyEnv        string            `mapstructure:"api_key_env"`
}

// OpenAIConfig holds OpenAI-specific configuration.
type OpenAIConfig struct {
	Model       string         `mapstructure:"model"`
	APIKeyEnv   string         `mapstructure:"api_key_env"`
	BaseURL     string         `mapstructure:"base_url"`
	ExtraFields map[string]any `mapstructure:"extra_fields"`
}

// GitLabConfig holds GitLab integration settings.
type GitLabConfig struct {
	URL          string `mapstructure:"url"`
	TokenEnv     string `mapstructure:"token_env"`
	WorkspaceDir string `mapstructure:"workspace_dir"`
}

// JiraConfig holds Jira integration settings.
type JiraConfig struct {
	BaseURL      string            `mapstructure:"base_url"`
	UserName     string            `mapstructure:"username"`
	APITokenEnv  string            `mapstructure:"api_token_env"`
	Enabled      bool              `mapstructure:"enabled"`
	CustomFields map[string]string `mapstructure:"custom_fields"` // Map of field ID to friendly name
}

// AgentsConfig holds agent-specific settings.
type AgentsConfig struct {
	MaxConcurrent int                    `mapstructure:"max_concurrent"`
	Timeout       int                    `mapstructure:"timeout_seconds"`
	AICodeEditor  AICodeEditorConfig     `mapstructure:"ai_code_editor"`
	Types         map[string]AgentConfig `mapstructure:"types"`
}

// AICodeEditorConfig holds specific settings for AI Code Editor agent.
type AICodeEditorConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	MaxTasks     int      `mapstructure:"max_tasks"`
	Capabilities []string `mapstructure:"capabilities"`
}

// AgentConfig holds generic agent configuration.
type AgentConfig struct {
	Enabled      bool              `mapstructure:"enabled"`
	MaxTasks     int               `mapstructure:"max_tasks"`
	Environment  map[string]string `mapstructure:"environment"`
	Capabilities []string          `mapstructure:"capabilities"`
}

// ServerConfig holds server-specific settings.
type ServerConfig struct {
	Port        int    `mapstructure:"port"`
	Host        string `mapstructure:"host"`
	MetricsPort int    `mapstructure:"metrics_port"`
}

// LoadConfig loads configuration from file and environment variables.
func LoadConfig() (*Config, error) {
	// Set defaults
	setDefaults()

	// Set config name and paths
	viper.SetConfigName(".hive")
	viper.SetConfigType("yaml")

	// Add config paths
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath("/etc/hive/")

	// Enable environment variable support
	viper.AutomaticEnv()
	viper.SetEnvPrefix("HIVE")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if errors.As(err, &viper.ConfigFileNotFoundError{}) {
			// Config file not found, use defaults
			log.Println("configuration file not found, use environment variable")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	log.Println("using configuration file at", viper.ConfigFileUsed())
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate and normalize config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values.
func setDefaults() {
	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// AI defaults - supporting both standard Anthropic and company GrabGPT setup
	viper.SetDefault("ai.provider", "claude")
	viper.SetDefault("ai.model", "claude-3-5-sonnet-20241022")

	// GitLab defaults
	viper.SetDefault("gitlab.workspace_dir", getDefaultWorkspaceDir())

	// Agent defaults
	viper.SetDefault("agents.max_concurrent", 5)
	viper.SetDefault("agents.timeout_seconds", 300)
	viper.SetDefault("agents.ai_code_editor.enabled", true)
	viper.SetDefault("agents.ai_code_editor.max_tasks", 2)
	viper.SetDefault("agents.ai_code_editor.capabilities", []string{
		"ai_code_generation", "feature_development", "gitlab_integration",
	})

	// Jira defaults
	viper.SetDefault("jira.enabled", false)
	viper.SetDefault("jira.is_cloud", true)

	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.metrics_port", 9090)
}

// validateConfig validates the configuration values
func validateConfig(config *Config) error {
	// Validate Redis config
	if config.Redis.Addr == "" {
		return fmt.Errorf("redis.addr cannot be empty")
	}

	// Validate AI config
	if config.AI.Provider == "" {
		return fmt.Errorf("ai.provider cannot be empty")
	}

	switch config.AI.Provider {
	case "claude":
		if config.AI.Claude == nil {
			return fmt.Errorf("ai.claude configuration is required when provider is 'claude'")
		}
		if config.AI.Claude.APIKeyEnv == "" {
			return fmt.Errorf("ai.claude.api_key_env cannot be empty")
		}
		// Check if API key environment variable exists
		if os.Getenv(config.AI.Claude.APIKeyEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.AI.Claude.APIKeyEnv)
		}
	case "openai":
		if config.AI.OpenAI == nil {
			return fmt.Errorf("ai.openai configuration is required when provider is 'openai'")
		}
		if config.AI.OpenAI.APIKeyEnv == "" {
			return fmt.Errorf("ai.openai.api_key_env cannot be empty")
		}
		// Check if API key environment variable exists
		if os.Getenv(config.AI.OpenAI.APIKeyEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.AI.OpenAI.APIKeyEnv)
		}
	default:
		return fmt.Errorf("unsupported ai.provider: %s", config.AI.Provider)
	}

	// Validate GitLab config
	if config.GitLab.URL == "" {
		return fmt.Errorf("gitlab.url cannot be empty")
	}

	if config.GitLab.TokenEnv == "" {
		return errors.New("gitlab.token_env cannot be empty")
	}

	// Check if GitLab token environment variable exists
	if os.Getenv(config.GitLab.TokenEnv) == "" {
		return fmt.Errorf("environment variable %s is not set", config.GitLab.TokenEnv)
	}

	// Validate workspace directory
	if config.GitLab.WorkspaceDir == "" {
		config.GitLab.WorkspaceDir = getDefaultWorkspaceDir()
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(config.GitLab.WorkspaceDir, 0750); err != nil {
		return fmt.Errorf("failed to create workspace directory %s: %w",
			config.GitLab.WorkspaceDir, err)
	}

	// Validate Jira config if enabled
	if config.Jira.Enabled {
		if config.Jira.BaseURL == "" {
			return fmt.Errorf("jira.base_url cannot be empty when jira is enabled")
		}
		if config.Jira.UserName == "" {
			return fmt.Errorf("jira.username cannot be empty when jira is enabled")
		}
		if config.Jira.APITokenEnv == "" {
			return fmt.Errorf("jira.api_token_env cannot be empty when jira is enabled")
		}
		// Check if API token environment variable exists
		if os.Getenv(config.Jira.APITokenEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.Jira.APITokenEnv)
		}
	}

	return nil
}

// getDefaultWorkspaceDir returns the default workspace directory.
func getDefaultWorkspaceDir() string {
	if dir := os.Getenv("HIVE_WORKSPACE_DIR"); dir != "" {
		return dir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/hive-workspace"
	}

	return filepath.Join(homeDir, ".hive", "workspace")
}
