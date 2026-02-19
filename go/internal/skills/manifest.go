package skills

import (
	"strings"
)

// Manifest holds the metadata parsed from a skill's YAML frontmatter.
type Manifest struct {
	Name        string
	Description string
	Trigger     string
}

// ParseManifest extracts YAML frontmatter fields from markdown content.
// It looks for content between opening and closing "---" delimiters and
// parses simple "key: value" lines. No external YAML library is used.
func ParseManifest(content string) Manifest {
	var m Manifest

	// The content must start with "---".
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return m
	}

	// Find the closing "---".
	rest := trimmed[3:] // skip opening "---"
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	} else if len(rest) == 0 {
		return m
	}

	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		return m
	}

	frontmatter := rest[:closeIdx]

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			m.Name = value
		case "description":
			m.Description = value
		case "trigger":
			m.Trigger = value
		}
	}

	return m
}
