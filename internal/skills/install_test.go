package skills

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ParseGithubTreeURL
// ---------------------------------------------------------------------------

func TestParseGithubTreeURL_Valid(t *testing.T) {
	tests := []struct {
		url    string
		owner  string
		repo   string
		branch string
		path   string
	}{
		{
			url:    "https://github.com/acme/skills-pack/tree/main/productivity",
			owner:  "acme",
			repo:   "skills-pack",
			branch: "main",
			path:   "productivity",
		},
		{
			url:    "https://github.com/user/repo/tree/develop/path/to/skills",
			owner:  "user",
			repo:   "repo",
			branch: "develop",
			path:   "path/to/skills",
		},
	}

	for _, tc := range tests {
		owner, repo, branch, path, err := ParseGithubTreeURL(tc.url)
		if err != nil {
			t.Errorf("ParseGithubTreeURL(%q) error: %v", tc.url, err)
			continue
		}
		if owner != tc.owner {
			t.Errorf("owner = %q; want %q", owner, tc.owner)
		}
		if repo != tc.repo {
			t.Errorf("repo = %q; want %q", repo, tc.repo)
		}
		if branch != tc.branch {
			t.Errorf("branch = %q; want %q", branch, tc.branch)
		}
		if path != tc.path {
			t.Errorf("path = %q; want %q", path, tc.path)
		}
	}
}

func TestParseGithubTreeURL_RejectsTraversal(t *testing.T) {
	_, _, _, _, err := ParseGithubTreeURL("https://github.com/owner/repo/tree/main/../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestParseGithubTreeURL_RejectsNonGithub(t *testing.T) {
	urls := []string{
		"https://gitlab.com/owner/repo/tree/main/skills",
		"https://example.com/owner/repo/tree/main/skills",
		"not-a-url",
		"",
	}
	for _, u := range urls {
		_, _, _, _, err := ParseGithubTreeURL(u)
		if err == nil {
			t.Errorf("expected error for non-github URL %q", u)
		}
	}
}

// ---------------------------------------------------------------------------
// SafeName
// ---------------------------------------------------------------------------

func TestSafeName_StripsSpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello-world", "hello-world"},
		{"My Skill!", "my-skill"},
		{"test@#$%skill", "testskill"},
		{"  spaces  ", "spaces"},
		{"UPPER_case", "upper_case"},
		{"dots.and.stuff", "dotsandstuff"},
	}
	for _, tc := range tests {
		got := SafeName(tc.input)
		if got != tc.want {
			t.Errorf("SafeName(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// IsSafeSubpath
// ---------------------------------------------------------------------------

func TestIsSafeSubpath_Valid(t *testing.T) {
	paths := []string{
		"skills/greeting.md",
		"folder/subfolder/file.md",
		"simple.md",
		"a-b_c/d.md",
	}
	for _, p := range paths {
		if !IsSafeSubpath(p) {
			t.Errorf("IsSafeSubpath(%q) = false; want true", p)
		}
	}
}

func TestIsSafeSubpath_TraversalAttempt(t *testing.T) {
	paths := []string{
		"../escape.md",
		"../../etc/passwd",
		"skills/../../../etc/passwd",
		"/absolute/path",
		"",
	}
	for _, p := range paths {
		if IsSafeSubpath(p) {
			t.Errorf("IsSafeSubpath(%q) = true; want false", p)
		}
	}
}
