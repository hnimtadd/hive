package pipeline

type PipelineCommandKey string

var (
	PipelineSubmitInputKey PipelineCommandKey = "submit-input"
)

type PipelineCommand struct {
	Key     PipelineCommandKey
	Payload any
}

type PipelineSubmitInputPayload struct {
	CorrelationID string
	Input         string
}
