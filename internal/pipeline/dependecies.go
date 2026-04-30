package pipeline

import (
	"github.com/hnimtadd/hive/internal/bee/registry"
	"github.com/hnimtadd/hive/internal/eventbus"
	"github.com/hnimtadd/hive/internal/model/llm"
	"github.com/hnimtadd/hive/internal/observability"
	"github.com/hnimtadd/hive/pkg/config"
	"github.com/hnimtadd/hive/pkg/types"
)

type PipelineDependencies struct {
	EventBus      *eventbus.EventBus[*types.HiveEvent]
	SessionLogger *observability.SessionLogger
	Config        config.Config
	Registry      registry.Registry
	Provider      llm.Provider
}
