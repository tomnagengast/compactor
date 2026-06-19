package capsule

import (
	"strings"
)

const DefaultMaxBytes = 2048

func Trim(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	if maxBytes < 16 {
		return value[:maxBytes]
	}
	return strings.TrimSpace(value[:maxBytes-15]) + "\n[truncated]\n"
}
