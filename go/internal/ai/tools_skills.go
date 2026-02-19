package ai

import (
	"fmt"
	"strings"

	"github.com/pocketbrain/pocketbrain/internal/skills"
)

// RegisterSkillsTools adds the 4 skills tools to the registry.
func RegisterSkillsTools(reg *Registry, svc *skills.Service) {
	reg.Register(&Tool{
		Name:        "skill_list",
		Description: "List all available skills in the workspace.",
		Parameters:  []ToolParam{},
		Execute: func(args map[string]any) (string, error) {
			list, err := svc.List()
			if err != nil {
				return fmt.Sprintf("Error listing skills: %v", err), nil
			}
			if len(list) == 0 {
				return "No skills found. Create one with skill_create or install from GitHub with install_skill.", nil
			}
			var b strings.Builder
			b.WriteString(fmt.Sprintf("Found %d skill(s):\n", len(list)))
			for _, s := range list {
				b.WriteString(fmt.Sprintf("- %s", s.Name))
				if s.Description != "" {
					b.WriteString(fmt.Sprintf(": %s", s.Description))
				}
				if s.Trigger != "" {
					b.WriteString(fmt.Sprintf(" [trigger: %s]", s.Trigger))
				}
				b.WriteString("\n")
			}
			return b.String(), nil
		},
	})

	reg.Register(&Tool{
		Name:        "skill_load",
		Description: "Load a specific skill by name and return its full content.",
		Parameters: []ToolParam{
			{Name: "name", Type: "string", Description: "Name of the skill to load", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			name := argString(args, "name")
			if name == "" {
				return "Error: skill name is required", nil
			}
			skill, err := svc.Load(name)
			if err != nil {
				return fmt.Sprintf("Error loading skill %q: %v", name, err), nil
			}
			return skill.Content, nil
		},
	})

	reg.Register(&Tool{
		Name:        "skill_create",
		Description: "Create a new skill with the given name and markdown content.",
		Parameters: []ToolParam{
			{Name: "name", Type: "string", Description: "Name for the skill (alphanumeric, hyphens, underscores)", Required: true},
			{Name: "content", Type: "string", Description: "Full markdown content including optional YAML frontmatter", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			name := argString(args, "name")
			content := argString(args, "content")
			if name == "" {
				return "Error: skill name is required", nil
			}
			if content == "" {
				return "Error: skill content is required", nil
			}
			if err := svc.Create(name, content); err != nil {
				return fmt.Sprintf("Error creating skill %q: %v", name, err), nil
			}
			return fmt.Sprintf("Skill %q created successfully.", name), nil
		},
	})

	reg.Register(&Tool{
		Name:        "install_skill",
		Description: "Install skills from a GitHub repository tree URL.",
		Parameters: []ToolParam{
			{Name: "url", Type: "string", Description: "GitHub tree URL (e.g. https://github.com/owner/repo/tree/main/skills)", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			rawURL := argString(args, "url")
			if rawURL == "" {
				return "Error: GitHub URL is required", nil
			}
			if err := svc.Install(rawURL); err != nil {
				return fmt.Sprintf("Error installing skills: %v", err), nil
			}
			return "Skills installed successfully.", nil
		},
	})
}
