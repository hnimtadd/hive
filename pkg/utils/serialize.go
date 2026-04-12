package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SanitizeUTF8 replaces invalid UTF-8 sequences with the Unicode replacement character (U+FFFD).
// This is useful when sending strings over gRPC, as protobuf requires valid UTF-8 for string fields.
func SanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "\uFFFD")
}

func JSONConvert[to any](val any) (to, error) {
	var result to
	jsonBytes, err := json.Marshal(val)
	if err != nil {
		return result, fmt.Errorf("failed to marshal json data: %w", err)
	}
	if err = json.Unmarshal(jsonBytes, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal json data to target type: %w", err)
	}
	return result, nil
}
