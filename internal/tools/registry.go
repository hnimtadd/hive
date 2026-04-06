package tools

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/internal/tools/system"
	"github.com/hnimtadd/hive/pkg/config"
)

type Registry interface {
	ListTools() map[string]tool.InvokableTool
}

type registry struct {
	config *config.Config
	tools  map[string]tool.InvokableTool
	path   string
}

func NewRegistry(appConfig *config.Config) (Registry, error) {
	r := &registry{
		config: appConfig,
		path:   appConfig.Tools.Dir,
	}
	tools, err := r.scan()
	if err != nil {
		return nil, fmt.Errorf("failed to scan tools: %w", err)
	}
	r.tools = tools
	systemTools, err := system.Tools()
	if err != nil {
		return nil, fmt.Errorf("failed to scan system tool: %w", err)
	}
	maps.Copy(r.tools, systemTools)
	return r, nil
}

func (r *registry) scan() (map[string]tool.InvokableTool, error) {
	entries, err := os.ReadDir(r.path)
	if err != nil {
		log.Printf("failed to read tools home: %s\n", err)
		return map[string]tool.InvokableTool{}, nil
	}
	tools := map[string]tool.InvokableTool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mdPath := filepath.Join(r.path, entry.Name(), "tool.yaml")
		config, err := LoadToolConfig(mdPath) //nolint: govet// ignore lint
		if err != nil {
			log.Printf("failed to load tool configuration from :%s, err: %s\n", mdPath, err)
			continue
		}
		for _, secret := range config.Secret {
			if !secret.IsValid() {
				log.Printf("failed to load tool: %s, secret is not fullfilled:\n%s", entry.Name(), secret.String())
				return nil, fmt.Errorf("failed to load tool: %s, secret is not fullfilled", entry.Name())
			}
		}
		fmt.Println("=====", config, "=====")
		tool, err := NewHiveTool(config)
		if err != nil {
			log.Printf("failed to initialize tool: %s", err)
			continue
		}
		info, err := tool.Info(context.Background())
		if err != nil {
			log.Printf("failed to fetch tool info: %s", err)
			continue
		}
		tools[info.Name] = tool
	}
	log.Printf("successfully scanned %d tools\n", len(tools))
	return tools, nil
}

// ListTools implements [Registry].
func (r *registry) ListTools() map[string]tool.InvokableTool {
	return r.tools
}
