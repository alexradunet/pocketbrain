/**
 * Prompt Builder
 * Responsible for building system prompts.
 * Follows Single Responsibility Principle.
 */

export interface PromptBuilderOptions {
  heartbeatIntervalMinutes: number
  vaultEnabled?: boolean
  vaultPath?: string
}

export class PromptBuilder {
  private readonly options: PromptBuilderOptions

  constructor(options: PromptBuilderOptions) {
    this.options = options
  }

  /**
   * Build the main agent system prompt
   */
  buildAgentSystemPrompt(memoryEntries: MemoryEntry[]): string {
    return [
      "You are PocketBrain, an autonomous assistant agent running on top of OpenCode.",
      "You help with coding and non-coding work: planning, research, writing, operations, and execution tasks.",
      "Be concise, practical, and proactive.",
      "Use native OpenCode plugin tools when relevant.",
      "Output plain text only.",
      "No Markdown under any circumstances.",
      "Never use Markdown markers or structure: no headings, no lists, no code fences, no inline code, no bold/italic, no blockquotes, no links.",
      "Avoid characters commonly used for Markdown formatting (e.g. # * _ ` > -). Use simple sentences instead.",
      "Do not use tables or any rich formatting because replies are shown in non-Markdown chat surfaces.",
      "A heartbeat cron runs in a separate session and its summary is added to the main session.",
      "After heartbeat summaries are added, if the user should be informed, call send_channel_message.",
      "send_channel_message delivers to the last used channel/user.",
      "",
      `Heartbeat interval: ${this.options.heartbeatIntervalMinutes} minutes`,
      "",
      this.buildVaultInstructions(),
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
      "",
      "Current memory:",
      this.buildMemoryContext(memoryEntries),
    ].join("\n")
  }

  /**
   * Build the heartbeat task prompt
   */
  buildHeartbeatPrompt(tasks: string[], recentContext: string): string {
    return [
      "Run these recurring cron tasks for the project.",
      "Return concise actionable bullet points with findings and next actions.",
      "This is routine task execution, not a healthcheck.",
      "If nothing requires action, explicitly say no action is needed.",
      "",
      recentContext ? "Recent main session context:" : "",
      recentContext,
      "",
      "Task list:",
      ...tasks.map((t, i) => `${i + 1}. ${t}`),
    ].join("\n")
  }

  /**
   * Build the proactive notification decision prompt
   */
  buildProactiveNotificationPrompt(): string {
    return [
      "Heartbeat summary was added to context.",
      "Decide whether the user should be proactively informed now.",
      "If yes, call send_channel_message with a concise plain-text message.",
      "If not needed, do nothing.",
    ].join("\n")
  }

  /**
   * Build vault instructions for the system prompt
   */
  private buildVaultInstructions(): string {
    if (!this.options.vaultEnabled) {
      return ""
    }

    return [
      "VAULT ACCESS:",
      "You have access to a personal knowledge vault organized as markdown files.",
      "The vault follows this structure:",
      "- inbox/: Quick captures and fleeting notes",
      "- daily/: Daily notes (YYYY-MM-DD.md format)",
      "- journal/: Long-form writing and reflections",
      "- projects/: Active projects with goals and tasks",
      "- areas/: Ongoing areas of responsibility (health, finance, etc)",
      "- resources/: Reference material (books, articles, recipes)",
      "- archive/: Completed or dormant items",
      "",
      "Vault tools available:",
      "- vault_read: Read any file by path",
      "- vault_write: Create or overwrite a file",
      "- vault_append: Append to a file (good for daily notes)",
      "- vault_list: List files in a folder",
      "- vault_search: Search files by name",
      "- vault_move: Move/rename files between folders",
      "- vault_backlinks: Find notes linking to a wiki-link target",
      "- vault_tag_search: Find notes containing a tag",
      "- vault_daily: Access today's daily note",
      "- vault_stats: Get vault statistics",
      "",
      "When using the vault:",
      "- Prefer linking between notes using relative paths",
      "- Use daily notes for timestamped entries and quick captures",
      "- Move items from inbox/ to appropriate folders after processing",
      "- Archive completed projects instead of deleting",
    ].join("\n")
  }

  private buildMemoryContext(entries: MemoryEntry[]): string {
    if (entries.length === 0) {
      return "No saved durable facts."
    }
    return entries
      .map((entry) => (entry.source ? `- (${entry.source}) ${entry.fact}` : `- ${entry.fact}`))
      .join("\n")
  }
}
import type { MemoryEntry } from "./ports/memory-repository"
