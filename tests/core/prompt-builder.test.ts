import { describe, test, expect } from "bun:test"
import { PromptBuilder } from "../../src/core/prompt-builder"

describe("PromptBuilder", () => {
  test("includes vault instructions when vault is enabled", () => {
    const builder = new PromptBuilder({
      heartbeatIntervalMinutes: 30,
      vaultEnabled: true,
      vaultPath: "/data/vault",
    })

    const prompt = builder.buildAgentSystemPrompt([{ id: 1, fact: "test fact", source: "test" }])
    expect(prompt).toContain("VAULT ACCESS")
    expect(prompt).toContain("vault_read")
  })

  test("omits vault instructions when vault is disabled", () => {
    const builder = new PromptBuilder({
      heartbeatIntervalMinutes: 30,
      vaultEnabled: false,
    })

    const prompt = builder.buildAgentSystemPrompt([{ id: 1, fact: "test fact" }])
    expect(prompt).not.toContain("VAULT ACCESS")
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
