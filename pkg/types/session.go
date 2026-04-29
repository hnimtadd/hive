package types

type HiveSession struct {
	ID       string    `json:"_id"`
	Messages []Message `json:"messages"          jsonschema:"session messages"`
	Summary  string    `json:"summary,omitempty" jsonschema:"Compressed history of the session"`
}
