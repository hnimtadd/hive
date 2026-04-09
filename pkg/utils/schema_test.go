package utils

import (
	"encoding/json"
	"testing"
)

func TestFixUnescapedNewlines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name: "multiline string with actual newlines",
			// input has literal newlines inside the JSON string (invalid JSON)
			input: "{\"code\": \"line1\nline2\nline3\"}",
			// expected has escaped newlines (valid JSON)
			expected: "{\"code\": \"line1\\nline2\\nline3\"}",
		},
		{
			name:     "already escaped newlines",
			input:    `{"code": "line1\nline2"}`,
			expected: `{"code": "line1\nline2"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixUnescapedNewlines(tt.input)

			// Verify it's valid JSON
			var obj interface{}
			if err := json.Unmarshal([]byte(got), &obj); err != nil {
				t.Errorf("Result is not valid JSON: %v\nGot: %s", err, got)
			}

			if got != tt.expected {
				t.Errorf("fixUnescapedNewlines() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHeristicallyExtractJSONString_WithNewlines(t *testing.T) {
	// This simulates what the agent outputs - raw JSON with literal newlines in strings
	input := "{\n  \"summary\": \"test\",\n  \"code\": \"line1\nline2\nline3\"\n}"

	got, err := HeristicallyExtractJSONString(input)
	if err != nil {
		t.Fatalf("HeristicallyExtractJSONString() error = %v", err)
	}

	// Verify it's valid JSON
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Errorf("Result is not valid JSON: %v\nGot: %s", err, got)
	}

	// Check the code field has the correct value
	code, ok := obj["code"].(string)
	if !ok {
		t.Fatal("code field not found or not a string")
	}

	// After parsing, the newlines should be actual newlines again
	if code != "line1\nline2\nline3" {
		t.Errorf("code field = %q, want %q", code, "line1\nline2\nline3")
	}
}
