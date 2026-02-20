package ai

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/skills"
)

// Skills tool input types.

type skillListInput struct{}

type skillLoadInput struct {
	Name string `json:"name" description:"Name of the skill to load"`
}

type skillCreateInput struct {
	Name    string `json:"name" description:"Name for the skill (alphanumeric, hyphens, underscores)"`
	Content string `json:"content" description:"Full markdown content including optional YAML frontmatter"`
}

type installSkillInput struct {
	URL string `json:"url" description:"GitHub tree URL (e.g. https://github.com/owner/repo/tree/main/skills)"`
}

// SkillsTools returns the 4 skills tools as Fantasy AgentTools.
func SkillsTools(svc *skills.Service) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool(
			"skill_list",
			"List all available skills in the workspace.",
			func(_ context.Context, _ skillListInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				list, err := svc.List()
				if err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error listing skills: %v", err)), nil
				}
				if len(list) == 0 {
					return fantasy.NewTextResponse("No skills found. Create one with skill_create or install from GitHub with install_skill."), nil
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
				return fantasy.NewTextResponse(b.String()), nil
			},
		),

		fantasy.NewAgentTool(
			"skill_load",
			"Load a specific skill by name and return its full content.",
			func(_ context.Context, input skillLoadInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if input.Name == "" {
					return fantasy.NewTextResponse("Error: skill name is required"), nil
				}
				skill, err := svc.Load(input.Name)
				if err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error loading skill %q: %v", input.Name, err)), nil
				}
				return fantasy.NewTextResponse(skill.Content), nil
			},
		),

		fantasy.NewAgentTool(
			"skill_create",
			"Create a new skill with the given name and markdown content.",
			func(_ context.Context, input skillCreateInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if input.Name == "" {
					return fantasy.NewTextResponse("Error: skill name is required"), nil
				}
				if input.Content == "" {
					return fantasy.NewTextResponse("Error: skill content is required"), nil
				}
				if err := svc.Create(input.Name, input.Content); err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error creating skill %q: %v", input.Name, err)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Skill %q created successfully.", input.Name)), nil
			},
		),

		fantasy.NewAgentTool(
			"install_skill",
			"Install skills from a GitHub repository tree URL.",
			func(_ context.Context, input installSkillInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if input.URL == "" {
					return fantasy.NewTextResponse("Error: GitHub URL is required"), nil
				}
				if err := svc.Install(input.URL); err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error installing skills: %v", err)), nil
				}
				return fantasy.NewTextResponse("Skills installed successfully."), nil
			},
		),
	}
}
