package bee

import (
	"fmt"

	"github.com/hnimtadd/hive/pkg/utils"
)

func GetSystemPrompt[I, O any](persona string) (string, error) {
	inputDescription, err := utils.DescribeJSONSchema[I]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the input: %w", err)
	}
	// Use text description instead of raw JSON schema to avoid LLM confusion
	outputDescription, err := utils.DescribeOutputJSON[O]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the output: %w", err)
	}
	return fmt.Sprintf(`%s

========= INPUT FORMAT ========
You receive this JSON as input:
%s

========= OUTPUT FORMAT ========
%s

Remember: Output DATA in the specified format, not a schema definition!
`, persona, inputDescription, outputDescription), nil
}
