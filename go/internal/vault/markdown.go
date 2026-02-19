// Package vault provides high-level vault operations for PocketBrain.
package vault

import (
	"regexp"
	"sort"
	"strings"
)

// WikiLinkMatch represents a single parsed wiki-link from markdown content.
type WikiLinkMatch struct {
	Raw             string
	Target          string
	Alias           string // empty when no alias present
	NormalizedTarget string
}

// NormalizeWikiLinkTarget returns the canonical lower-cased, trimmed form of a
// wiki-link target used for backlink comparisons.
func NormalizeWikiLinkTarget(target string) string {
	return strings.ToLower(strings.TrimSpace(target))
}

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]\n]+)\]\]`)

// ParseWikiLinks extracts all [[...]] wiki-links from content.
func ParseWikiLinks(content string) []WikiLinkMatch {
	rawMatches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
	matches := make([]WikiLinkMatch, 0, len(rawMatches))

	for _, m := range rawMatches {
		rawInner := strings.TrimSpace(m[1])
		if rawInner == "" {
			continue
		}

		parts := strings.SplitN(rawInner, "|", 2)
		target := strings.TrimSpace(parts[0])
		if target == "" {
			continue
		}

		alias := ""
		if len(parts) == 2 {
			alias = strings.TrimSpace(parts[1])
		}

		matches = append(matches, WikiLinkMatch{
			Raw:              "[[" + rawInner + "]]",
			Target:           target,
			Alias:            alias,
			NormalizedTarget: NormalizeWikiLinkTarget(target),
		})
	}

	return matches
}

// headingPattern matches markdown headings (# through ######).
var headingPattern = regexp.MustCompile(`^#{1,6}\s`)

// tagPattern matches inline tags: #word or #word/sub, not preceded by a word char or /.
// Uses the same logic as the TypeScript version.
var tagPattern = regexp.MustCompile(`(^|[^\w/])#([A-Za-z0-9][A-Za-z0-9_\-]*(?:/[A-Za-z0-9][A-Za-z0-9_\-]*)*)`)

// ExtractMarkdownTags returns a sorted, deduplicated list of #tags found in
// content. Heading lines are skipped. Tags are normalised to lower-case.
func ExtractMarkdownTags(content string) []string {
	seen := make(map[string]struct{})
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for _, line := range lines {
		if headingPattern.MatchString(line) {
			continue
		}

		submatches := tagPattern.FindAllStringSubmatch(line, -1)
		for _, sm := range submatches {
			// sm[2] is the tag body without the leading #
			tagBody := sm[2]
			if tagBody == "" {
				continue
			}
			tagValue := "#" + strings.ToLower(tagBody)
			if len(tagValue) > 1 {
				seen[tagValue] = struct{}{}
			}
		}
	}

	tags := make([]string, 0, len(seen))
	for t := range seen {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	return tags
}
