package hive

import (
	"fmt"
	"os"
	"strings"
)

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

	for part := range strings.SplitSeq(tag, ";") {
		part = strings.TrimSpace(part)
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

func Debugln(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
}

func Debugf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}
