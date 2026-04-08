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
	outputDescription, err := utils.DescribeJSONSchema[O]()
	if err != nil {
		return "", fmt.Errorf("failed to self describe the output: %w", err)
	}
	return fmt.Sprintf(`%s
		You suppose to handle input and output with these specific formats
		========= INPUT ========
		YOU ONLY RECEIVE THIS JSON ONLY AS INPUT
		%s

		========= OUTPUT ======
		YOU HAVE TO RESPONSE A RAW JSON THAT FOLLOWS THIS SCHEMA
		%s

		`, persona, inputDescription, outputDescription), nil
}
