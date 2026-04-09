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
	content = content[first : last+1]

	// Fix unescaped newlines inside JSON strings
	// LLMs often output multi-line strings but don't escape them for JSON
	content = fixUnescapedNewlines(content)

	return content, nil
}

// fixUnescapedNewlines escapes literal newlines that are inside JSON string values
func fixUnescapedNewlines(content string) string {
	var result strings.Builder
	inString := false
	escaped := false

	for i := 0; i < len(content); i++ {
		char := content[i]

		if escaped {
			result.WriteByte(char)
			escaped = false
			continue
		}

		if char == '\\' {
			result.WriteByte(char)
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			result.WriteByte(char)
			continue
		}

		// If we're inside a string and hit a literal newline, escape it
		if inString && (char == '\n' || char == '\r') {
			if char == '\n' {
				result.WriteString("\\n")
			} else {
				result.WriteString("\\r")
			}
		} else {
			result.WriteByte(char)
		}
	}

	return result.String()
}
