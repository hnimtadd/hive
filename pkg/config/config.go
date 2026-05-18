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
	AI           AIConfig      `mapstructure:"ai"`
	Server       ServerConfig  `mapstructure:"server"`
	WorkspaceDir string        `mapstructure:"workspace"`
	BeeHiveDir   string        `mapstructure:"beehive"`
	Bees         BeeConfig     `mapstructure:"bee"`
	Tools        ToolConfig    `mapstructure:"tool"`
	Tracing      TraceConfig   `mapstructure:"tracing"`
	Storage      StorageConfig `mapstructure:"storage"`
}

// AIConfig holds AI/LLM configuration.
type AIConfig struct {
	Tiers     map[string]ModelTier `mapstructure:"tiers"`
	Anthropic *AnthropicConfig     `mapstructure:"anthropic"`
	OpenAI    *OpenAIConfig        `mapstructure:"openai"`
	Ollama    *OllamaConfig        `mapstructure:"ollama"`
	MaxStep   int                  `mapstructure:"max_step"`
	Context   ContextConfig        `mapstructure:"context"`
}
type BeeConfig struct {
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	Dir            string
	PoolSize       int `mapstructure:"pool_size"`
}

type ToolConfig struct {
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	Dir            string
}

type TaskConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
	Storage string        `mapstructure:"storage"`
}

// ContextConfig holds context management configuration.
type ContextConfig struct {
	MaxMessagesPerTask       int `mapstructure:"max_messages_per_task"`
	SummaryTriggerThreshold  int `mapstructure:"summary_trigger_threshold"`
	SummaryTargetTokens      int `mapstructure:"summary_target_tokens"`
	MaxTaskDescriptionTokens int `mapstructure:"max_task_description_tokens"`
}

type ModelTiers struct {
	Fast  ModelTier `mapstructure:"fast"`
	Smart ModelTier `mapstructure:"smart"`
}

type ModelTier struct {
	Provider string   `mapstructure:"provider"`
	Model    string   `mapstructure:"model"`
	Models   []string `mapstructure:"models"`
}

type AnthropicProvider string

const (
	AnthropicIntegrationTypeBedrock AnthropicProvider = "bedrock"
	AnthropicIntegrationTypeAPI     AnthropicProvider = "api"
)

// AnthropicConfig holds Anthropic-specific configuration.
type AnthropicConfig struct {
	Provider         AnthropicProvider `mapstructure:"provider"`
	AnthropicVersion string            `mapstructure:"anthropic_version"`
	BaseURL          string            `mapstructure:"api_base_url"`
	Headers          map[string]string `mapstructure:"api_headers"`
	Region           string            `mapstructure:"bedrock_region"`
	APIKeyEnv        string            `mapstructure:"api_key_env"`
}

// OpenAIConfig holds OpenAI-specific configuration.
type OpenAIConfig struct {
	APIKeyEnv   string         `mapstructure:"api_key_env"`
	BaseURL     string         `mapstructure:"base_url"`
	ExtraFields map[string]any `mapstructure:"extra_fields"`
}

// OllamaConfig holds OllamaConfig-specific configuration.
type OllamaConfig struct {
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
	Enabled    bool             `mapstructure:"enabled"`
	LogLevel   string           `mapstructure:"log_level"`
	LogFormat  string           `mapstructure:"log_format"`
	LogFile    string           `mapstructure:"log_file"`
	AddSource  bool             `mapstructure:"add_source"`
	SessionLog SessionLogConfig `mapstructure:"session_log"`
}

// SessionLogConfig configures session logging for agent/LLM interactions.
type SessionLogConfig struct {
	// Enabled turns on session logging
	Enabled bool `mapstructure:"enabled"`
	// AccessLogDir is the output directory for session logs (default: ./hive/sessions)
	AccessLogDir string `mapstructure:"dir"`
	// LogRequests logs incoming LLM requests
	LogRequests bool `mapstructure:"log_requests"`
	// LogResponses logs outgoing LLM responses
	LogResponses bool `mapstructure:"log_responses"`
	// LogTools logs tool calls
	LogTools bool `mapstructure:"log_tools"`
	// MaxContentLength limits logged content size (0 = no limit)
	MaxContentLength int `mapstructure:"max_content_length"`
}

// StorageConfig configures storage related configuration.
type StorageConfig struct {
	Dir string `mapstructure:"dir"`
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
	// Agent defaults
	viper.SetDefault("agents.max_concurrent", 5)
	viper.SetDefault("agents.timeout_seconds", 300)

	viper.SetDefault("ai.max_step", 5)

	// Context management defaults
	viper.SetDefault("ai.context.max_messages_per_task", 10)
	viper.SetDefault("ai.context.summary_trigger_threshold", 8)
	viper.SetDefault("ai.context.summary_target_tokens", 500)
	viper.SetDefault("ai.context.max_task_description_tokens", 2000)

	// Server defaults
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.metrics_port", 9090)
	viper.SetDefault("server.max_timeout", 10*time.Minute)
	viper.SetDefault("server.graceful_shutdown_timeout", 30*time.Second)

	hiveSpace := getDefaultHiveSpace()
	// Tasks defaults
	viper.SetDefault("task.timeout", 10*time.Minute)
	viper.SetDefault("bee.default_timeout", 2*time.Minute)
	viper.SetDefault("bee.pool_size", 3)
	viper.SetDefault("tool.default_timeout", 1*time.Minute)

	viper.SetDefault("workspace", hiveSpace+"/workspace")
	viper.SetDefault("beehive", hiveSpace+"/behive")

	viper.SetDefault("tracing.enabled", true)
	viper.SetDefault("tracing.log_level", "info")
	viper.SetDefault("tracing.log_format", "json")
	viper.SetDefault("tracing.log_file", "")
	viper.SetDefault("tracing.add_source", false)
	viper.SetDefault("tracing.session_log.enabled", true)
	viper.SetDefault("tracing.session_log.dir", "")
	viper.SetDefault("tracing.session_log.log_requests", true)
	viper.SetDefault("tracing.session_log.log_responses", true)
	viper.SetDefault("tracing.session_log.log_tools", true)
	viper.SetDefault("tracing.session_log.max_content_length", 2000)
	viper.SetDefault("storage.dir", hiveSpace+"/storage")
}

// validateConfig validates the configuration values.
func validateConfig(config *Config) error {
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

	// Validate and create session log directory
	if config.Tracing.SessionLog.Enabled {
		sessionLogDir := config.Tracing.SessionLog.AccessLogDir
		if sessionLogDir == "" {
			sessionLogDir = filepath.Join(config.BeeHiveDir, "sessions")
		} else {
			sessionLogDir, err = utils.ExpandPath(sessionLogDir)
			if err != nil {
				return fmt.Errorf("failed to expand session_log.dir: %w", err)
			}
		}
		config.Tracing.SessionLog.AccessLogDir = sessionLogDir
		if err = os.MkdirAll(sessionLogDir, 0750); err != nil {
			return fmt.Errorf("failed to create session log directory %s: %w", sessionLogDir, err)
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
