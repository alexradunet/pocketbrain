// Package skills provides a service for listing, loading, and creating
// markdown-based skill files within a workspace.
package skills

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

// validName matches alphanumeric characters, hyphens, and underscores.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Skill represents a single skill loaded from the workspace.
type Skill struct {
	Name        string // from frontmatter or filename
	Description string // from frontmatter
	Trigger     string // when to activate
	Content     string // full markdown content
	FilePath    string // relative path within the workspace
}

// Service manages skills stored in the workspace.
type Service struct {
	ws        *workspace.Workspace
	skillsDir string // relative directory within the workspace
	logger    *slog.Logger
}

// New creates a new skills service backed by the given workspace.
func New(ws *workspace.Workspace, logger *slog.Logger) *Service {
	return &Service{
		ws:        ws,
		skillsDir: "skills",
		logger:    logger,
	}
}

// List returns all skills found in the skills directory.
// Only .md files are considered. Non-markdown files are ignored.
func (s *Service) List() ([]Skill, error) {
	files, err := s.ws.ListFiles(s.skillsDir)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	var skills []Skill
	for _, f := range files {
		if f.IsDirectory {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(f.Name), ".md") {
			continue
		}

		content, ok := s.ws.ReadFile(f.Path)
		if !ok {
			s.log().Warn("could not read skill file", "path", f.Path)
			continue
		}

		manifest := ParseManifest(content)
		name := manifest.Name
		if name == "" {
			// Derive name from filename without extension.
			name = strings.TrimSuffix(f.Name, ".md")
		}

		skills = append(skills, Skill{
			Name:        name,
			Description: manifest.Description,
			Trigger:     manifest.Trigger,
			Content:     content,
			FilePath:    f.Path,
		})
	}

	return skills, nil
}

// Load reads a specific skill by name. The name must be a simple identifier
// (alphanumeric, hyphens, underscores). Path traversal is blocked.
func (s *Service) Load(name string) (*Skill, error) {
	if !validName.MatchString(name) {
		return nil, fmt.Errorf("invalid skill name: %q", name)
	}

	relPath := s.skillsDir + "/" + name + ".md"
	content, ok := s.ws.ReadFile(relPath)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", name)
	}

	manifest := ParseManifest(content)
	skillName := manifest.Name
	if skillName == "" {
		skillName = name
	}

	return &Skill{
		Name:        skillName,
		Description: manifest.Description,
		Trigger:     manifest.Trigger,
		Content:     content,
		FilePath:    relPath,
	}, nil
}

// Create writes a new skill file to the workspace. The name must be a simple
// identifier (alphanumeric, hyphens, underscores).
func (s *Service) Create(name, content string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid skill name: %q (must be alphanumeric, hyphens, underscores)", name)
	}

	relPath := s.skillsDir + "/" + name + ".md"
	if !s.ws.WriteFile(relPath, content) {
		return fmt.Errorf("failed to write skill file: %s", relPath)
	}

	s.log().Info("skill created", "name", name, "path", relPath)
	return nil
}

func (s *Service) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}
