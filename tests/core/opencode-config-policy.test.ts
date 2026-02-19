import { describe, test, expect } from "bun:test"

interface OpenCodeConfig {
  plugin?: string[]
}

const ALLOWED_PLUGINS = [
  "./src/adapters/plugins/install-skill.plugin.ts",
  "./src/adapters/plugins/memory.plugin.ts",
  "./src/adapters/plugins/channel-message.plugin.ts",
  "./src/adapters/plugins/vault.plugin.ts",
]

describe("OpenCode config policy", () => {
  test("uses only approved vault-only plugin surface", async () => {
    const config = (await Bun.file("opencode.json").json()) as OpenCodeConfig
    expect(config.plugin).toEqual(ALLOWED_PLUGINS)
  })

  test("does not include system execution style plugins", async () => {
    const config = (await Bun.file("opencode.json").json()) as OpenCodeConfig
    const pluginList = config.plugin ?? []
    const forbiddenPattern = /exec|shell|terminal|system|command-run|bash|sh/i

    for (const pluginPath of pluginList) {
      expect(forbiddenPattern.test(pluginPath)).toBe(false)
    }
  })
})
