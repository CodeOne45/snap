package deps

import (
	"fmt"
	"strings"
)

// AddEnvEntry adds a "key=value" line to an env file (e.g. .env.example),
// preserving every existing line untouched. changed is false when key is
// already present with the exact value wanted (idempotent no-op); present
// with a different value is a *ConflictError, never silently replaced.
func AddEnvEntry(content []byte, key, value string) ([]byte, bool, error) {
	str := string(content)
	for _, line := range strings.Split(str, "\n") {
		trimmed := strings.TrimSpace(line)
		existing, ok := strings.CutPrefix(trimmed, key+"=")
		if !ok {
			continue
		}
		if existing == value {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: key, Existing: existing, Wanted: value}
	}

	entry := fmt.Sprintf("%s=%s\n", key, value)
	if len(str) == 0 {
		return []byte(entry), true, nil
	}
	if !strings.HasSuffix(str, "\n") {
		str += "\n"
	}
	return []byte(str + entry), true, nil
}
