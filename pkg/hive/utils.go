package hive

import "strings"

type hiveTag struct {
	Key         string
	Description string
	OmitEmpty   bool
}

func parseHiveTag(tag string) hiveTag {
	result := hiveTag{}
	if tag == "" {
		return result
	}

	parts := strings.Split(tag, ";")
	for _, part := range parts {
		part := strings.TrimSpace(part)
		if part == "omitempty" {
			result.OmitEmpty = true
			continue
		}

		// handle key-value pair like "key=KEY"
		// We use splitn with 2 to ensure we focus on first '=' character only
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			switch key {
			case "key":
				result.Key = val
			case "description":
				result.Description = val
			}
		}
	}

	return result
}
