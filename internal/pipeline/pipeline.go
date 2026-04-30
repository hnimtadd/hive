package pipeline

type Pipeline struct {
	interation []Stage
}

func NewPipeline() *Pipeline {
	iteration := []Stage{
		NewContextStage(),
	}
}
