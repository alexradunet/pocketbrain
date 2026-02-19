import { describe, test, expect } from "bun:test"
import { PromptBuilder } from "../../src/core/prompt-builder"

describe("PromptBuilder", () => {
  test("includes vault instructions when vault is enabled", () => {
    const builder = new PromptBuilder({
      heartbeatIntervalMinutes: 30,
      vaultEnabled: true,
      vaultPath: "/data/vault",
      vaultFolders: {
        inbox: "00-inbox",
        daily: "01-daily-journey",
        journal: "01-daily-journey",
        projects: "02-projects",
        areas: "03-areas",
        resources: "04-resources",
        archive: "05-archive",
      },
    })

    const prompt = builder.buildAgentSystemPrompt([{ id: 1, fact: "test fact", source: "test" }])
    expect(prompt).toContain("VAULT ACCESS")
    expect(prompt).toContain("vault_read")
    expect(prompt).toContain("vault_obsidian_config")
    expect(prompt).toContain("vault_daily_track")
    expect(prompt).toContain("After a vault is imported or first connected, call vault_obsidian_config")
    expect(prompt).toContain("Runtime mode: vault-only.")
    expect(prompt).toContain("You do not have shell, host, or system command execution capabilities.")
    expect(prompt).toContain("Do not assume a fixed folder taxonomy.")
  })

  test("omits vault instructions when vault is disabled", () => {
    const builder = new PromptBuilder({
      heartbeatIntervalMinutes: 30,
      vaultEnabled: false,
    })

    const prompt = builder.buildAgentSystemPrompt([{ id: 1, fact: "test fact" }])
    expect(prompt).not.toContain("VAULT ACCESS")
    expect(prompt).toContain("Runtime mode: chat-only without vault access.")
  })

  test("builds heartbeat prompt with tasks and recent context", () => {
    const builder = new PromptBuilder({ heartbeatIntervalMinutes: 30 })
    const prompt = builder.buildHeartbeatPrompt(["Task A", "Task B"], "ASSISTANT: prior summary")

    expect(prompt).toContain("Task list:")
    expect(prompt).toContain("1. Task A")
    expect(prompt).toContain("2. Task B")
    expect(prompt).toContain("Recent main session context")
  })
})
