package queue

type Priority int

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
)
