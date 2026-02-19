import { afterEach, beforeEach, describe, expect, test } from "bun:test"
import { loadConfig } from "../../src/config"

const ENV_KEYS = [
  "DATA_DIR",
  "LOG_LEVEL",
  "OPENCODE_PORT",
  "HEARTBEAT_INTERVAL_MINUTES",
  "OPENCODE_SERVER_URL",
  "OPENCODE_MODEL",
  "ENABLE_WHATSAPP",
  "WHATSAPP_AUTH_DIR",
  "WHATSAPP_WHITELIST_NUMBERS",
  "WHATSAPP_WHITELIST_NUMBER",
  "VAULT_ENABLED",
  "VAULT_PATH",
] as const

type EnvKey = (typeof ENV_KEYS)[number]
type Snapshot = Record<EnvKey, string | undefined>

function captureSnapshot(): Snapshot {
  const snapshot = {} as Snapshot
  for (const key of ENV_KEYS) {
    snapshot[key] = Bun.env[key]
  }
  return snapshot
}

function restoreSnapshot(snapshot: Snapshot): void {
  for (const key of ENV_KEYS) {
    const value = snapshot[key]
    if (value === undefined) {
      delete Bun.env[key]
      continue
    }
    Bun.env[key] = value
  }
}

describe("loadConfig", () => {
  let envSnapshot: Snapshot

  beforeEach(() => {
    envSnapshot = captureSnapshot()
  })

  afterEach(() => {
    restoreSnapshot(envSnapshot)
  })

  test("throws on invalid log level", () => {
    Bun.env.LOG_LEVEL = "verbose"
    expect(() => loadConfig()).toThrow()
  })

  test("throws on invalid OPENCODE_PORT", () => {
    Bun.env.OPENCODE_PORT = "70000"
    expect(() => loadConfig()).toThrow()
  })

  test("throws on invalid OPENCODE_SERVER_URL", () => {
    Bun.env.OPENCODE_SERVER_URL = "not-a-url"
    expect(() => loadConfig()).toThrow()
  })

  test("throws on invalid OPENCODE_MODEL format", () => {
    Bun.env.OPENCODE_MODEL = "invalid-model"
    expect(() => loadConfig()).toThrow()
  })

  test("accepts valid explicit configuration", () => {
    Bun.env.LOG_LEVEL = "debug"
    Bun.env.OPENCODE_PORT = "4096"
    Bun.env.HEARTBEAT_INTERVAL_MINUTES = "10"
    Bun.env.OPENCODE_SERVER_URL = "http://127.0.0.1:4096"
    Bun.env.OPENCODE_MODEL = "openai/gpt-5"
    Bun.env.DATA_DIR = ".data"
    Bun.env.ENABLE_WHATSAPP = "true"
    Bun.env.WHATSAPP_AUTH_DIR = ".data/whatsapp-auth"
    Bun.env.WHATSAPP_WHITELIST_NUMBERS = "15551234567,+44 7700 900123,15551234567"
    Bun.env.WHATSAPP_WHITELIST_NUMBER = "12025550123"
    Bun.env.VAULT_ENABLED = "true"
    Bun.env.VAULT_PATH = ".data/vault"

    const config = loadConfig()
    expect(config.logLevel).toBe("debug")
    expect(config.opencodePort).toBe(4096)
    expect(config.opencodeServerUrl).toBe("http://127.0.0.1:4096")
    expect(config.opencodeModel).toBe("openai/gpt-5")
    expect(config.whatsAppWhitelistNumbers).toEqual(["15551234567", "447700900123", "12025550123"])
  })
})
