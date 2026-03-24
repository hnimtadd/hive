package tools

import (
	"os"

	"github.com/adrg/frontmatter"
)

func LoadToolConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	_, err = frontmatter.Parse(f, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
