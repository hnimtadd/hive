package secret

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Requirement struct {
	Key         string `json:"key"         yaml:"key"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required"    yaml:"required"`
}

func (r Requirement) IsValid() bool {
	if !r.Required {
		return true
	}

	_, exists := os.LookupEnv(r.Key)
	return exists
}

func (r Requirement) String() string {
	yamlBytes, _ := yaml.Marshal(r)
	return string(yamlBytes)
}
