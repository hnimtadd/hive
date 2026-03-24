package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadToolConfig(toolYamlPath string) (*Config, error) {
	f, err := os.Open(toolYamlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	err = yaml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, err
	}
	cfg.path = filepath.Dir(toolYamlPath)

	if cfg.Name == "" || cfg.Entrypoint == "" {
		return nil, fmt.Errorf("tool at %s is missing name or entrypoint", toolYamlPath)
	}

	return &cfg, nil
}
