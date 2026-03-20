package utils

import (
	"errors"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

func DescribeJSONSchema[T any]() (string, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return "", err
	}
	schemaJSON, err := schema.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(schemaJSON), nil
}

func HeristicallyExtractJSONString(content string) (string, error) {
	// The Pipeline:
	//     Clean: Trim whitespace.
	//     Strip: Remove ```json and ``` tags if present.
	//     Snip: Use the strings.Index / strings.LastIndex method above.
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")
	first, last := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if first == -1 || last < first {
		return "", errors.New("invalid JSON, could not find JSON object")
	}
	return content[first : last+1], nil
}
