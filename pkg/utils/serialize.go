package utils

import (
	"encoding/json"
	"fmt"
)

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
