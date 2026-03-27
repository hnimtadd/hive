package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/hnimtadd/hive/internal/secret"
)

type Config struct {
	Name         string               `json:"name"        yaml:"name"`
	Description  string               `json:"description" yaml:"description"`
	Parameters   map[string]any       `json:"parameters"  yaml:"parameters"`
	Runtime      string               `json:"runtime"     yaml:"runtime"`
	Entrypoint   []string             `json:"entrypoint"  yaml:"entrypoint"`
	TimeoutInSec int                  `json:"timeout"     yaml:"timeout"`
	Secret       []secret.Requirement `json:"secret"      yaml:"secret"`

	path string
}

type hiveTool struct {
	config *Config
	schema *jsonschema.Schema
}

func NewHiveTool(config *Config) (tool.InvokableTool, error) {
	schemaBytes, err := json.Marshal(config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tool parameters: %w", err)
	}

	schema := &jsonschema.Schema{}
	if err = schema.UnmarshalJSON(schemaBytes); err != nil {
		return nil, fmt.Errorf("failed to parse jsonschema of tool parameters: %w", err)
	}
	return hiveTool{
		config: config,
		schema: schema,
	}, nil
}

// Info implements [tool.InvokableTool].
func (h hiveTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        h.config.Name,
		Desc:        h.config.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(h.schema),
	}, nil
}

// InvokableRun implements [tool.InvokableTool].
func (h hiveTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	switch h.config.Runtime {
	case "native":
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(h.config.TimeoutInSec)*time.Second)
		defer cancel()

		fmt.Println(h.config.Entrypoint)
		cmd := exec.CommandContext(ctx, h.config.Entrypoint[0], h.config.Entrypoint[1:]...)
		fmt.Println(h.config.path)
		cmd.Dir = h.config.path
		env := []string{}
		secrets := h.config.ResolveSecret()
		for key, secret := range secrets {
			env = append(env, fmt.Sprintf("%s=%s", key, secret))
		}
		env = append(env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdin = bytes.NewReader([]byte(argumentsInJSON))
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to execute tools: %w", err)
		}
		if stderr.Len() > 0 {
			log.Printf("Tool debug: %s\n", stderr.String())
		}
		log.Printf("Tool output: %s\n", stdout.String())
		return stdout.String(), nil

	default:
		log.Printf("not supported runtime: %s", h.config.Runtime)
		return "", fmt.Errorf("not supported runtime: %s", h.config.Runtime)
	}
}

func (c Config) ResolveSecret() map[string]string {
	secret := map[string]string{}
	for _, required := range c.Secret {
		secret[required.Key] = os.Getenv(required.Key)
	}
	return secret
}
