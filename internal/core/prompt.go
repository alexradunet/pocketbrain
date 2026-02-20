package core

import (
	"fmt"
	"strings"
)

// PromptBuilderOptions configures the prompt builder.
type PromptBuilderOptions struct {
	HeartbeatIntervalMinutes int
	WorkspaceEnabled         bool
}

// PromptBuilder constructs system prompts for the assistant.
type PromptBuilder struct {
	opts PromptBuilderOptions
}

// NewPromptBuilder creates a PromptBuilder with the given options.
func NewPromptBuilder(opts PromptBuilderOptions) *PromptBuilder {
	return &PromptBuilder{opts: opts}
}

// BuildAgentSystemPrompt returns the main agent system prompt populated with
// the caller's durable memory entries.
func (b *PromptBuilder) BuildAgentSystemPrompt(memoryEntries []MemoryEntry) string {
	lines := []string{
		"You are PocketBrain, an autonomous assistant agent running on top of OpenCode.",
		"You help with coding and non-coding work: planning, research, writing, operations, and execution tasks.",
		"Be concise, practical, and proactive.",
		"Use native OpenCode plugin tools when relevant.",
	}

	lines = append(lines, b.buildRuntimeBoundaryInstructions()...)

	lines = append(lines,
		"Output plain text only.",
		"No Markdown under any circumstances.",
		"Never use Markdown markers or structure: no headings, no lists, no code fences, no inline code, no bold/italic, no blockquotes, no links.",
		"Avoid characters commonly used for Markdown formatting (e.g. # * _ ` > -). Use simple sentences instead.",
		"Do not use tables or any rich formatting because replies are shown in non-Markdown chat surfaces.",
		"A heartbeat cron runs in a separate session and its summary is added to the main session.",
		"After heartbeat summaries are added, if the user should be informed, call send_channel_message.",
		"send_channel_message delivers to the last used channel/user.",
		"",
		fmt.Sprintf("Heartbeat interval: %d minutes", b.opts.HeartbeatIntervalMinutes),
		"",
		b.buildWorkspaceInstructions(),
		"",
		"Memory rules:",
		"- Memory is durable user memory only (stable preferences, profile, constraints, recurring goals).",
		"- Do not store transient one-off chat details.",
		"- When you discover durable memory, call the save_memory tool.",
		"- save_memory takes one short, atomic durable fact per call.",
		"",
		"Skills rules:",
		"- If the user asks to install/pull a skill, use the install_skill tool.",
		"- install_skill supports GitHub tree URLs only.",
		"- Installed skills must be placed under .agents/skills.",
		"- Self-improve: if a task would benefit from a reusable workflow, or repeats, or could be standardized, proactively use skill-creator to draft a new skill after the task is handled.",
		"- Also suggest skill-creator when the user asks for something new that seems like a reusable capability.",
		"- When vault access is enabled, proactively apply pocketbrain-vault-autoconfig behavior at session start and after vault imports.",
		"",
		"Current memory:",
		b.buildMemoryContext(memoryEntries),
	)

	return strings.Join(lines, "\n")
}

// BuildHeartbeatPrompt returns the prompt used to drive heartbeat task execution.
func (b *PromptBuilder) BuildHeartbeatPrompt(tasks []string, recentContext string) string {
	lines := []string{
		"Run these recurring cron tasks for the project.",
		"Return concise actionable bullet points with findings and next actions.",
		"This is routine task execution, not a healthcheck.",
		"If nothing requires action, explicitly say no action is needed.",
		"",
	}

	if recentContext != "" {
		lines = append(lines, "Recent main session context:")
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, recentContext, "", "Task list:")

	for i, t := range tasks {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, t))
	}

	return strings.Join(lines, "\n")
}

// BuildProactiveNotificationPrompt returns the prompt that asks the model to
// decide whether to send a proactive message after a heartbeat summary.
func (b *PromptBuilder) BuildProactiveNotificationPrompt() string {
	lines := []string{
		"Heartbeat summary was added to context.",
		"Decide whether the user should be proactively informed now.",
		"If yes, call send_channel_message with a concise plain-text message.",
		"If not needed, do nothing.",
	}
	return strings.Join(lines, "\n")
}

// buildRuntimeBoundaryInstructions returns mode-specific capability constraints.
func (b *PromptBuilder) buildRuntimeBoundaryInstructions() []string {
	if !b.opts.WorkspaceEnabled {
		return []string{
			"Runtime mode: chat-only without workspace access.",
			"Do not claim to run host or system commands.",
		}
	}
	return []string{
		"Runtime mode: workspace-only.",
		"You do not have shell, host, or system command execution capabilities.",
		"If a user requests host-level changes, explain that an operator must perform them outside chat.",
	}
}

// buildWorkspaceInstructions returns workspace access instructions, or an empty
// string when workspace access is disabled.
func (b *PromptBuilder) buildWorkspaceInstructions() string {
	if !b.opts.WorkspaceEnabled {
		return ""
	}

	lines := []string{
		"WORKSPACE ACCESS:",
		"You have access to a file workspace exposed as a WebDAV directory.",
		"Files are plain markdown and other document formats.",
		"Adapt to the user's existing directory structure and naming conventions.",
		"",
		"Workspace tools available:",
		"- workspace_read: Read any file by path",
		"- workspace_write: Create or overwrite a file",
		"- workspace_append: Append to a file",
		"- workspace_list: List files in a folder",
		"- workspace_search: Search files by name, content, or both",
		"- workspace_move: Move/rename files between folders",
		"- workspace_stats: Get workspace statistics",
		"",
		"When using the workspace:",
		"- Before major write operations, inspect the workspace with workspace_list and workspace_search to mirror existing organization",
		"- Prefer relative paths when linking between notes",
		"- Archive completed projects instead of deleting",
	}

	return strings.Join(lines, "\n")
}

// buildMemoryContext formats memory entries for inclusion in the system prompt.
func (b *PromptBuilder) buildMemoryContext(entries []MemoryEntry) string {
	if len(entries) == 0 {
		return "No saved durable facts."
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Source != nil && *e.Source != "" {
			lines = append(lines, fmt.Sprintf("- (%s) %s", *e.Source, e.Fact))
		} else {
			lines = append(lines, "- "+e.Fact)
		}
	}
	return strings.Join(lines, "\n")
}
