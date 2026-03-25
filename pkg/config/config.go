package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/hnimtadd/hive/pkg/utils"
	"github.com/spf13/viper"
)

// Config represents the complete Hive configuration.
type Config struct {
	AI           AIConfig     `mapstructure:"ai"`
	Gitlab       GitlabConfig `mapstructure:"gitlab"`
	Jira         JiraConfig   `mapstructure:"jira"`
	Server       ServerConfig `mapstructure:"server"`
	WorkspaceDir string       `mapstructure:"workspace"`
	BeeHiveDir   string       `mapstructure:"beehive"`
	ToolsDir     string
	BeesDir      string
}

// AIConfig holds AI/LLM configuration.
type AIConfig struct {
	Provider string        `mapstructure:"provider"`
	Claude   *ClaudeConfig `mapstructure:"claude"`
	OpenAI   *OpenAIConfig `mapstructure:"openai"`
	MaxStep  int           `mapstructure:"max_step"`
}
type AgentConfig struct {
	Dir string `mapstructure:"home"`
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
	Model                string         `mapstructure:"model"`
	APIKeyEnv            string         `mapstructure:"api_key_env"`
	BaseURL              string         `mapstructure:"base_url"`
	ExtraFields          map[string]any `mapstructure:"extra_fields"`
	PreferResponseSchema *openai.ChatCompletionResponseFormat
}

// GitlabConfig holds GitLab integration settings.
type GitlabConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	URL      string `mapstructure:"url"`
	TokenEnv string `mapstructure:"token_env"`
}

// JiraConfig holds Jira integration settings.
type JiraConfig struct {
	BaseURL      string            `mapstructure:"base_url"`
	UserName     string            `mapstructure:"username"`
	APITokenEnv  string            `mapstructure:"api_token_env"`
	Enabled      bool              `mapstructure:"enabled"`
	CustomFields map[string]string `mapstructure:"custom_fields"` // Map of field ID to friendly name
}

// ServerConfig holds server-specific settings.
type ServerConfig struct {
	Port                    int           `mapstructure:"port"`
	Host                    string        `mapstructure:"host"`
	DefaultBeeTimeout       time.Duration `mapstructure:"default_bee_timeout"`
	MaxBeeTimeout           time.Duration `mapstructure:"max_bee_timeout"`
	LLMTimeout              time.Duration `mapstructure:"llm_timeout"`
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"`
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

	// AI defaults - supporting both standard Anthropic and company setup
	viper.SetDefault("ai.provider", "claude")
	viper.SetDefault("ai.model", "claude-3-5-sonnet-20241022")

	// GitLab defaults

	// Agent defaults
	viper.SetDefault("agents.max_concurrent", 5)
	viper.SetDefault("agents.timeout_seconds", 300)

	viper.SetDefault("ai.max_step", 5)

	// Jira defaults
	viper.SetDefault("jira.enabled", false)
	viper.SetDefault("jira.is_cloud", true)

	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.metrics_port", 9090)

	// Timeout defaults
	viper.SetDefault("server.default_bee_timeout", 2*time.Minute)
	viper.SetDefault("server.max_bee_timeout", 10*time.Minute)
	viper.SetDefault("server.llm_timeout", 30*time.Second)
	viper.SetDefault("server.graceful_shutdown_timeout", 30*time.Second)

	hiveSpace := getDefaultHiveSpace()
	viper.SetDefault("workspace", hiveSpace+"/workspace")
	viper.SetDefault("beehive", hiveSpace+"/behive")
}

// validateConfig validates the configuration values.
func validateConfig(config *Config) error {
	// Validate AI config
	if config.AI.Provider == "" {
		return errors.New("ai.provider cannot be empty")
	}

	switch config.AI.Provider {
	case "claude":
		if config.AI.Claude == nil {
			return errors.New("ai.claude configuration is required when provider is 'claude'")
		}
		if config.AI.Claude.APIKeyEnv == "" {
			return errors.New("ai.claude.api_key_env cannot be empty")
		}
		// Check if API key environment variable exists
		if os.Getenv(config.AI.Claude.APIKeyEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.AI.Claude.APIKeyEnv)
		}
	case "openai":
		if config.AI.OpenAI == nil {
			return errors.New("ai.openai configuration is required when provider is 'openai'")
		}
		if config.AI.OpenAI.APIKeyEnv == "" {
			return errors.New("ai.openai.api_key_env cannot be empty")
		}
		// Check if API key environment variable exists
		if os.Getenv(config.AI.OpenAI.APIKeyEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.AI.OpenAI.APIKeyEnv)
		}
	default:
		return fmt.Errorf("unsupported ai.provider: %s", config.AI.Provider)
	}

	// Validate GitLab config
	if config.Gitlab.URL == "" {
		return errors.New("gitlab.url cannot be empty")
	}

	if config.Gitlab.TokenEnv == "" {
		return errors.New("gitlab.token_env cannot be empty")
	}

	// Check if GitLab token environment variable exists
	if os.Getenv(config.Gitlab.TokenEnv) == "" {
		return fmt.Errorf("environment variable %s is not set", config.Gitlab.TokenEnv)
	}

	workspaceDir, err := utils.ExpandPath(config.WorkspaceDir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	config.WorkspaceDir = workspaceDir
	// Ensure workspace directory exists
	if err = os.MkdirAll(config.WorkspaceDir, 0750); err != nil {
		return fmt.Errorf("failed to create workspace directory %s: %w",
			config.WorkspaceDir, err)
	}

	beehiveDir, err := utils.ExpandPath(config.BeeHiveDir)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}
	fmt.Println("<====", beehiveDir)

	config.BeeHiveDir = beehiveDir
	if err = os.MkdirAll(config.BeeHiveDir, 0750); err != nil {
		return fmt.Errorf("failed to create bees home %s: %w", config.BeeHiveDir, err)
	}
	config.BeesDir = filepath.Join(config.BeeHiveDir, "bees")
	if err = os.MkdirAll(config.BeesDir, 0750); err != nil {
		return fmt.Errorf("failed to create bees home %s: %w", config.BeesDir, err)
	}

	config.ToolsDir = filepath.Join(config.BeeHiveDir, "tools")
	if err = os.MkdirAll(config.ToolsDir, 0750); err != nil {
		return fmt.Errorf("failed to create tools dir %s: %w", config.ToolsDir, err)
	}

	// Validate timeout configuration
	if config.Server.DefaultBeeTimeout <= 0 {
		return errors.New("server.default_bee_timeout must be positive")
	}
	if config.Server.MaxBeeTimeout <= 0 {
		return errors.New("server.max_bee_timeout must be positive")
	}
	if config.Server.DefaultBeeTimeout > config.Server.MaxBeeTimeout {
		return errors.New("server.default_bee_timeout cannot exceed server.max_bee_timeout")
	}
	if config.Server.LLMTimeout <= 0 {
		return errors.New("server.llm_timeout must be positive")
	}

	// Validate Jira config if enabled
	if config.Jira.Enabled {
		if config.Jira.BaseURL == "" {
			return errors.New("jira.base_url cannot be empty when jira is enabled")
		}
		if config.Jira.UserName == "" {
			return errors.New("jira.username cannot be empty when jira is enabled")
		}
		if config.Jira.APITokenEnv == "" {
			return errors.New("jira.api_token_env cannot be empty when jira is enabled")
		}
		// Check if API token environment variable exists
		if os.Getenv(config.Jira.APITokenEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.Jira.APITokenEnv)
		}
	}

	return nil
}

// getDefaultHiveSpace returns the default workspace directory.
func getDefaultHiveSpace() string {
	if dir := os.Getenv("HIVE_WORKSPACE_DIR"); dir != "" {
		return dir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/hive"
	}

	return filepath.Join(homeDir, ".hive")
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
