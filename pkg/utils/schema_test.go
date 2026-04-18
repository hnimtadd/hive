package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			var obj any
			if err := json.Unmarshal([]byte(got), &obj); err != nil {
				t.Errorf("Result is not valid JSON: %v\nGot: %s", err, got)
			}

			if got != tt.expected {
				t.Errorf("fixUnescapedNewlines() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestHeristicallyExtractJSONString(t *testing.T) {
	var tcs = []struct {
		name     string
		input    string
		wantErr  bool
		expected map[string]any
	}{
		{
			name:     "plain JSON object",
			input:    `{"name": "alice", "age": 30}`,
			expected: map[string]any{"name": "alice", "age": float64(30)},
		},
		{
			name:     "JSON in markdown code block",
			input:    "```json\n{\"name\": \"alice\", \"age\": 30}\n```",
			expected: map[string]any{"name": "alice", "age": float64(30)},
		},
		{
			name:     "JSON with surrounding prose",
			input:    `Here is the result: {"name": "alice", "age": 30} as requested.`,
			expected: map[string]any{"name": "alice", "age": float64(30)},
		},
		{
			name:     "JSON with leading and trailing whitespace",
			input:    `   {"name": "alice"}   `,
			expected: map[string]any{"name": "alice"},
		},
		{
			name:     "JSON with unescaped newlines in string value",
			input:    "{\"text\": \"line one\nline two\"}",
			expected: map[string]any{"text": "line one\nline two"},
		},
		{
			name:     "code block with extra whitespace after json tag",
			input:    "```json   \n{\"key\": \"value\"}\n```",
			expected: map[string]any{"key": "value"},
		},
		{
			name:  "nested JSON object",
			input: `{"user": {"name": "alice", "age": 30}}`,
			expected: map[string]any{
				"user": map[string]any{"name": "alice", "age": float64(30)},
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no JSON object found",
			input:   "hello there, no json here",
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   `{"name": "alice"`,
			wantErr: true,
		},
		{
			name:    "closing brace before opening brace",
			input:   `} "name": "alice" {`,
			wantErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := HeuristicallyExtractJSONString(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
			} else {
				require.NoError(t, err)

				// Verify it's valid JSON
				var obj map[string]any
				require.NoError(t, json.Unmarshal([]byte(got), &obj))
				assert.Equal(t, tc.expected, obj)
			}
		})
	}
}
