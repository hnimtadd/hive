package tools

import (
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/hnimtadd/hive/pkg/config"
)

type Registry interface {
	ListTools() map[string]tool.InvokableTool
}

type registry struct {
	config *config.Config
	tools  map[string]tool.InvokableTool
}

func NewRegistry(appConfig *config.Config) (Registry, error) {
	r := &registry{
		config: appConfig,
	}
	if err := r.scan(); err != nil {
		return nil, fmt.Errorf("failed to scan tools: %w", err)
	}
	return r, nil
}

func (r *registry) scan() error {
	return nil
}

// ListTools implements [Registry].
func (r *registry) ListTools() map[string]tool.InvokableTool {
	return r.tools
}
