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
	Provider   string            `mapstructure:"provider"`
	Model      string            `mapstructure:"model"`
	APIKeyEnv  string            `mapstructure:"api_key_env"`
	BaseURL    string            `mapstructure:"base_url"`
	Parameters map[string]string `mapstructure:"parameters"`
}

// GitLabConfig holds GitLab integration settings.
type GitLabConfig struct {
	URL          string `mapstructure:"url"`
	TokenEnv     string `mapstructure:"token_env"`
	WorkspaceDir string `mapstructure:"workspace_dir"`
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

// setDefaults sets default configuration values
func setDefaults() {
	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// AI defaults
	viper.SetDefault("ai.provider", "claude")
	viper.SetDefault("ai.model", "claude-3-5-sonnet-20241022")
	viper.SetDefault("ai.api_key_env", "ANTHROPIC_API_KEY")
	viper.SetDefault("ai.base_url", "")

	// GitLab defaults
	viper.SetDefault("gitlab.url", "https://gitlab.com")
	viper.SetDefault("gitlab.token_env", "GITLAB_TOKEN")
	viper.SetDefault("gitlab.workspace_dir", getDefaultWorkspaceDir())

	// Agent defaults
	viper.SetDefault("agents.max_concurrent", 5)
	viper.SetDefault("agents.timeout_seconds", 300)
	viper.SetDefault("agents.ai_code_editor.enabled", true)
	viper.SetDefault("agents.ai_code_editor.max_tasks", 2)
	viper.SetDefault("agents.ai_code_editor.capabilities", []string{
		"ai_code_generation", "feature_development", "gitlab_integration",
	})

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

	if config.AI.APIKeyEnv == "" {
		return fmt.Errorf("ai.api_key_env cannot be empty")
	}

	// Check if API key environment variable exists
	if os.Getenv(config.AI.APIKeyEnv) == "" {
		return fmt.Errorf("environment variable %s is not set", config.AI.APIKeyEnv)
	}

	// Validate GitLab config
	if config.GitLab.URL == "" {
		return fmt.Errorf("gitlab.url cannot be empty")
	}

	if config.GitLab.TokenEnv == "" {
		return fmt.Errorf("gitlab.token_env cannot be empty")
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
	if err := os.MkdirAll(config.GitLab.WorkspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory %s: %w",
			config.GitLab.WorkspaceDir, err)
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

// GetConfigExample returns an example configuration file content.
func GetConfigExample() string {
	return `# Hive Configuration File
# Place this file as ~/.hive.yaml or .hive.yaml in project root

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 10

ai:
  provider: "claude"
  model: "claude-3-5-sonnet-20241022"
  api_key_env: "ANTHROPIC_API_KEY"
  # base_url: ""  # Optional: custom API endpoint
  parameters:
    max_tokens: "4096"
    temperature: "0.1"

gitlab:
  url: "https://gitlab.com"
  token_env: "GITLAB_TOKEN"
  workspace_dir: "$HOME/.hive/workspace"

agents:
  max_concurrent: 5
  timeout_seconds: 300

  ai_code_editor:
    enabled: true
    max_tasks: 2
    capabilities:
      - "ai_code_generation"
      - "feature_development"
      - "gitlab_integration"
      - "automated_commits"
      - "merge_request_creation"

server:
  port: 8080
  host: "localhost"
  metrics_port: 9090

# Environment Variables Required:
# - ANTHROPIC_API_KEY: Your Claude API key
# - GITLAB_TOKEN: Your GitLab personal access token
`
}

