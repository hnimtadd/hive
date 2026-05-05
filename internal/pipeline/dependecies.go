package pipeline

import (
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/eventbus"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/pkg/config"
	agentv1 "github.com/hnimtadd/hive/proto/agent/v1"
)

type PipelineDependencies struct {
	EventBus      *eventbus.EventBus[*agentv1.SessionEvent]
	SessionLogger *observability.SessionLogger
	Config        config.Config
	Registry      registry.Registry
	Provider      llm.Provider

	Parent *Pipeline
}
