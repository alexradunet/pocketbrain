package core

import (
	"strings"
	"testing"
)

func TestPromptBuilder_BuildAgentSystemPrompt_ContainsPocketBrain(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	prompt := b.BuildAgentSystemPrompt(nil)
	if !strings.Contains(prompt, "PocketBrain") {
		t.Error("expected prompt to contain 'PocketBrain'")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_IncludesMemory(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	src := "test"
	entries := []MemoryEntry{
		{ID: 1, Fact: "user prefers dark mode"},
		{ID: 2, Fact: "user is a gopher", Source: &src},
	}
	prompt := b.BuildAgentSystemPrompt(entries)
	if !strings.Contains(prompt, "user prefers dark mode") {
		t.Error("expected memory fact 1 in prompt")
	}
	if !strings.Contains(prompt, "user is a gopher") {
		t.Error("expected memory fact 2 in prompt")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_EmptyMemory(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	prompt := b.BuildAgentSystemPrompt(nil)
	if !strings.Contains(prompt, "No saved durable facts") {
		t.Error("expected 'No saved durable facts' in prompt")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_WorkspaceEnabled(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30, WorkspaceEnabled: true})
	prompt := b.BuildAgentSystemPrompt(nil)
	if !strings.Contains(prompt, "WORKSPACE ACCESS") {
		t.Error("expected workspace instructions in prompt")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_WorkspaceDisabled(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30, WorkspaceEnabled: false})
	prompt := b.BuildAgentSystemPrompt(nil)
	if strings.Contains(prompt, "WORKSPACE ACCESS") {
		t.Error("expected no workspace instructions when workspace disabled")
	}
	if !strings.Contains(prompt, "chat-only") {
		t.Error("expected chat-only mode text when workspace disabled")
	}
}

func TestPromptBuilder_BuildHeartbeatPrompt_ContainsTasks(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	tasks := []string{"check email", "update calendar"}
	prompt := b.BuildHeartbeatPrompt(tasks, "")
	if !strings.Contains(prompt, "check email") {
		t.Error("expected task 'check email' in heartbeat prompt")
	}
	if !strings.Contains(prompt, "update calendar") {
		t.Error("expected task 'update calendar' in heartbeat prompt")
	}
}

func TestPromptBuilder_BuildHeartbeatPrompt_IncludesRecentContext(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	tasks := []string{"task-a"}
	recentCtx := "user asked about go modules"
	prompt := b.BuildHeartbeatPrompt(tasks, recentCtx)
	if !strings.Contains(prompt, "Recent main session context") {
		t.Error("expected 'Recent main session context' header")
	}
	if !strings.Contains(prompt, recentCtx) {
		t.Error("expected recent context text in prompt")
	}
}

func TestPromptBuilder_BuildHeartbeatPrompt_EmptyContext(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	tasks := []string{"task-a"}
	prompt := b.BuildHeartbeatPrompt(tasks, "")
	if strings.Contains(prompt, "Recent main session context") {
		t.Error("expected no recent context header when context is empty")
	}
}

func TestPromptBuilder_BuildProactiveNotificationPrompt_ContainsSendMessage(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	prompt := b.BuildProactiveNotificationPrompt()
	if !strings.Contains(prompt, "send_channel_message") {
		t.Error("expected 'send_channel_message' in proactive notification prompt")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_IncludesHeartbeatInterval(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 45})
	prompt := b.BuildAgentSystemPrompt(nil)
	if !strings.Contains(prompt, "45 minutes") {
		t.Error("expected heartbeat interval '45 minutes' in prompt")
	}
}

func TestPromptBuilder_BuildAgentSystemPrompt_MemoryWithSource(t *testing.T) {
	b := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	src := "slack"
	entries := []MemoryEntry{
		{ID: 1, Fact: "user location: NYC", Source: &src},
	}
	prompt := b.BuildAgentSystemPrompt(entries)
	if !strings.Contains(prompt, "(slack)") {
		t.Error("expected source '(slack)' in prompt")
	}
	if !strings.Contains(prompt, "user location: NYC") {
		t.Error("expected fact text in prompt")
	}
}
