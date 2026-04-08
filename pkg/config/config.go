package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hnimtadd/hive/pkg/utils"
	"github.com/spf13/viper"
)

// Config represents the complete Hive configuration.
type Config struct {
	AI           AIConfig     `mapstructure:"ai"`
	Server       ServerConfig `mapstructure:"server"`
	WorkspaceDir string       `mapstructure:"workspace"`
	BeeHiveDir   string       `mapstructure:"beehive"`
	Bees         BeeConfig    `mapstructure:"bee"`
	Tools        ToolConfig   `mapstructure:"tool"`
	Tasks        TaskConfig   `mapstructure:"task"`
	Tracing      TraceConfig  `mapstructure:"tracing"`
}

// AIConfig holds AI/LLM configuration.
type AIConfig struct {
	Provider  string           `mapstructure:"provider"`
	Anthropic *AnthropicConfig `mapstructure:"anthropic"`
	OpenAI    *OpenAIConfig    `mapstructure:"openai"`
	Ollama    *OllamaConfig    `mapstructure:"ollama"`
	MaxStep   int              `mapstructure:"max_step"`
}
type BeeConfig struct {
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	Dir            string
}

type ToolConfig struct {
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	Dir            string
}

type TaskConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
	Storage string        `mapstructure:"storage"`
}

type AnthropicProvider string

const (
	AnthropicIntegrationTypeBedrock AnthropicProvider = "bedrock"
	AnthropicIntegrationTypeAPI     AnthropicProvider = "api"
)

// AnthropicConfig holds Anthropic-specific configuration.
type AnthropicConfig struct {
	Provider         AnthropicProvider `mapstructure:"provider"`
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

// OllamaConfig holds OllamaConfig-specific configuration.
type OllamaConfig struct {
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

// ServerConfig holds server-specific settings.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
	// MaxTimeout is the maximum allowed timeout for any workflow
	MaxTimeout time.Duration `mapstructure:"max_timeout"`
	// GracefulShutdownTimeout is the timeout for graceful server shutdown
	GracefulShutdownTimeout time.Duration `mapstructure:"graceful_shutdown_timeout"`
}

type TraceConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
	LogFile   string `mapstructure:"log_file"`
	AddSource bool   `mapstructure:"add_source"`
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
	viper.SetDefault("ai.provider", "Anthropic")
	viper.SetDefault("ai.model", "Anthropic-3-5-sonnet-20241022")

	// Agent defaults
	viper.SetDefault("agents.max_concurrent", 5)
	viper.SetDefault("agents.timeout_seconds", 300)

	viper.SetDefault("ai.max_step", 5)

	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.metrics_port", 9090)
	viper.SetDefault("server.max_timeout", 10*time.Minute)
	viper.SetDefault("server.graceful_shutdown_timeout", 30*time.Second)

	hiveSpace := getDefaultHiveSpace()
	// Tasks defaults
	viper.SetDefault("task.timeout", 10*time.Minute)
	viper.SetDefault("task.storage", hiveSpace+"/storage")
	viper.SetDefault("bee.default_timeout", 2*time.Minute)
	viper.SetDefault("tool.default_timeout", 1*time.Minute)

	viper.SetDefault("workspace", hiveSpace+"/workspace")
	viper.SetDefault("beehive", hiveSpace+"/behive")

	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.log_level", "info")
	viper.SetDefault("tracing.log_format", "json")
	viper.SetDefault("tracing.log_file", "")
	viper.SetDefault("tracing.add_source", false)
}

// validateConfig validates the configuration values.
func validateConfig(config *Config) error {
	// Validate AI config
	if config.AI.Provider == "" {
		return errors.New("ai.provider cannot be empty")
	}

	switch config.AI.Provider {
	case "anthropic":
		if config.AI.Anthropic == nil {
			return errors.New("ai.anthropic configuration is required when provider is 'anthropic'")
		}
		if config.AI.Anthropic.APIKeyEnv == "" {
			return errors.New("ai.anthropic.api_key_env cannot be empty")
		}
		// Check if API key environment variable exists
		if os.Getenv(config.AI.Anthropic.APIKeyEnv) == "" {
			return fmt.Errorf("environment variable %s is not set", config.AI.Anthropic.APIKeyEnv)
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
	case "ollama":
		if config.AI.Ollama == nil {
			return errors.New("ai.ollama configuration is required when provider is 'ollama'")
		}
	default:
		return fmt.Errorf("unsupported ai.provider: %s", config.AI.Provider)
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

	config.BeeHiveDir = beehiveDir
	if err = os.MkdirAll(config.BeeHiveDir, 0750); err != nil {
		return fmt.Errorf("failed to create bees home %s: %w", config.BeeHiveDir, err)
	}
	beesDir := filepath.Join(config.BeeHiveDir, "bees")
	if err = os.MkdirAll(beesDir, 0750); err != nil {
		return fmt.Errorf("failed to create bees home %s: %w", beesDir, err)
	}
	config.Bees.Dir = beesDir

	toolsDir := filepath.Join(config.BeeHiveDir, "tools")
	if err = os.MkdirAll(toolsDir, 0750); err != nil {
		return fmt.Errorf("failed to create tools dir %s: %w", toolsDir, err)
	}
	config.Tools.Dir = toolsDir

	// Validate execution configuration
	if config.Tasks.Timeout <= 0 {
		return errors.New("task.timeout must be positive")
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
