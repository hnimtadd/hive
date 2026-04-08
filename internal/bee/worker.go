package bee

import (
	"github.com/hnimtadd/hive/pkg/types"
)

type WorkerOutput struct {
	Status       types.Status      `json:"status"               jsonschema:"Updated job state, either: not_started, in_progress, completed, failed, paused"`
	Observations string            `json:"observations"         jsonschema:"What did you find? This will be added to history."`
	NewArtifacts map[string]string `json:"new_artifacts"        jsonschema:"Any data found (e.g., ticket_details, log_snippet)"`
	NextSteps    string            `json:"next_steps,omitempty" jsonschema:"Optional suggestion for the supervisor"`
}

type WorkerInput struct {
	Context   string            `json:"status"    jsonschema:"High-level goal for the entire run"`
	Task      string            `json:"task"      jsonschema:"The exact instruction from the supervisor"`
	Artifacts map[string]string `json:"artifacts" jsonschema:"specfic data relevant to your task"`
}
