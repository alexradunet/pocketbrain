package skills

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// safeNameRe matches characters that are safe for filenames.
var safeNameRe = regexp.MustCompile(`[^a-z0-9_-]`)

// ParseGithubTreeURL validates and parses a GitHub tree URL.
// Expected format: https://github.com/owner/repo/tree/branch/path/to/dir
// Returns owner, repo, branch, path components.
func ParseGithubTreeURL(rawURL string) (owner, repo, branch, path string, err error) {
	if rawURL == "" {
		return "", "", "", "", fmt.Errorf("empty URL")
	}

	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", "", "", fmt.Errorf("invalid URL: %w", parseErr)
	}

	if parsed.Host != "github.com" {
		return "", "", "", "", fmt.Errorf("not a GitHub URL: host is %q", parsed.Host)
	}

	// Path should be: /owner/repo/tree/branch/path...
	trimmed := strings.TrimPrefix(parsed.Path, "/")
	parts := strings.SplitN(trimmed, "/", 5) // owner, repo, "tree", branch, path

	if len(parts) < 5 {
		return "", "", "", "", fmt.Errorf("invalid GitHub tree URL: expected /owner/repo/tree/branch/path")
	}

	if parts[2] != "tree" {
		return "", "", "", "", fmt.Errorf("invalid GitHub tree URL: expected 'tree' segment, got %q", parts[2])
	}

	owner = parts[0]
	repo = parts[1]
	branch = parts[3]
	path = parts[4]

	if owner == "" || repo == "" || branch == "" || path == "" {
		return "", "", "", "", fmt.Errorf("invalid GitHub tree URL: missing components")
	}

	// Block path traversal in the extracted path.
	if !IsSafeSubpath(path) {
		return "", "", "", "", fmt.Errorf("path traversal detected in URL path: %q", path)
	}

	return owner, repo, branch, path, nil
}

// SafeName converts a string into a safe filename component.
// It lowercases, replaces spaces with hyphens, and strips all characters
// that are not alphanumeric, hyphens, or underscores.
func SafeName(s string) string {
	result := strings.ToLower(strings.TrimSpace(s))
	result = strings.ReplaceAll(result, " ", "-")
	result = safeNameRe.ReplaceAllString(result, "")
	// Remove leading/trailing hyphens that may result from stripping.
	result = strings.Trim(result, "-")
	return result
}

// IsSafeSubpath checks whether a relative path is safe (no traversal).
// It rejects empty paths, absolute paths, and any path containing "..".
func IsSafeSubpath(path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return false
	}

	cleaned := filepath.Clean(path)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}
	// Also check for embedded traversal after cleaning.
	if strings.Contains(cleaned, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}

// Install clones skills from a GitHub tree URL into the workspace's skills
// directory using git sparse-checkout. This is a placeholder that validates
// the URL and returns the command that would be run.
func (s *Service) Install(rawURL string) error {
	owner, repo, branch, path, err := ParseGithubTreeURL(rawURL)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	s.log().Info("installing skills from GitHub",
		"clone_url", cloneURL,
		"branch", branch,
		"path", path,
		"target", s.skillsDir,
	)

	// In a full implementation this would run:
	//   git clone --filter=blob:none --sparse <cloneURL>
	//   cd <repo> && git sparse-checkout set <path>
	//   cp <path>/*.md <workspace>/skills/
	// For now we return nil to indicate successful validation.
	// The actual git operations require exec and are deferred to a future phase.

	return fmt.Errorf("install not yet implemented (validated: %s/%s branch=%s path=%s)", owner, repo, branch, path)
}
