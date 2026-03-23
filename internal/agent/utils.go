package agent

import (
	"os"

	"github.com/adrg/frontmatter"
)

func LoadAgentConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	// This reads the --- YAML --- block into 'cfg'
	// and returns the remaining Markdown as 'prompt'
	prompt, err := frontmatter.Parse(f, &cfg)
	if err != nil {
		return nil, err
	}

	// Set the Markdown body as the Description/Persona
	cfg.Description = string(prompt)

	return &cfg, nil
}
