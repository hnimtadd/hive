package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/hnimtadd/hive/pkg/hive"
	"github.com/hnimtadd/hive/pkg/secret"
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
		return h.handleNativeTool(ctx, argumentsInJSON)

	case "hive":
		return h.handleHiveTool(ctx, argumentsInJSON)

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
	log.Println("resolved to", secret)
	return secret
}

// handleNativeTool use normal stdin/out transport layer to execute executable file
// native tools are tool that are executable and have some agreements on the stdin,stdout
// expected format based on tool.yaml configuration.
func (h hiveTool) handleNativeTool(ctx context.Context, argumentsInJSON string) (string, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(h.config.TimeoutInSec)*time.Second)
	defer cancel()
	cmdPath, err := exec.LookPath(h.config.Entrypoint[0])
	if err != nil {
		log.Printf("tool is not executable: %s", err)
		return "", fmt.Errorf("tool is not executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, cmdPath, h.config.Entrypoint[1:]...)
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
	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to execute tools: %w", err)
	}
	if stderr.Len() > 0 {
		log.Printf("Tool debug: %s\n", stderr.String())
	}
	log.Printf("Tool output: %s\n", stdout.String())
	return stdout.String(), nil
}

// handleHiveTool use hive sdk client with stdin,out based transport layer
// to trigger the hive tool.
// Benefit of hive tool, is hive tool is  self-describe binary, so user maintain
// both secret requirements and description inside the code, ignore the tool.yaml dependencies.
func (h hiveTool) handleHiveTool(ctx context.Context, argumentsInJSON string) (string, error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Duration(h.config.TimeoutInSec)*time.Second)
	defer cancel()
	cmdPath, err := exec.LookPath(h.config.Entrypoint[0])
	if err != nil {
		log.Printf("tool is not executable: %s", err)
		return "", fmt.Errorf("tool is not executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, cmdPath, h.config.Entrypoint[1:]...)
	cmd.Dir = h.config.path
	env := []string{}
	secrets := h.config.ResolveSecret()
	for key, secret := range secrets {
		env = append(env, fmt.Sprintf("%s=%s", key, secret))
	}
	env = append(env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	env = append(env, fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")))
	env = append(env, fmt.Sprintf("GOCACHE=%s", os.Getenv("GOCACHE")))
	env = append(env, fmt.Sprintf("GOPROXY=%s", os.Getenv("GOPROXY")))
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start tools: %w", err)
	}

	client := hive.NewToolClient(stdout, stdin)
	var input json.RawMessage
	if err = json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}
	resp, err := client.Invoke(ctx, input)
	if err != nil {
		bytes, _ := io.ReadAll(stderr)
		log.Println(string(bytes))
		return "", fmt.Errorf("invoke tool failed: %w", err)
	}
	if !resp.Success {
		return resp.Error, nil
	}

	output, _ := json.Marshal(resp)
	return string(output), nil
}
