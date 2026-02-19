import { afterEach, beforeEach, describe, expect, test } from "bun:test"
import { loadConfig } from "../../src/config"

const ENV_KEYS = [
  "DATA_DIR",
  "LOG_LEVEL",
  "OPENCODE_PORT",
  "HEARTBEAT_INTERVAL_MINUTES",
  "OPENCODE_SERVER_URL",
  "OPENCODE_MODEL",
  "OPENCODE_CONFIG_DIR",
  "ENABLE_WHATSAPP",
  "WHATSAPP_AUTH_DIR",
  "WHATSAPP_WHITELIST_NUMBERS",
  "WHATSAPP_WHITELIST_NUMBER",
  "TAILDRIVE_ENABLED",
  "TAILDRIVE_SHARE_NAME",
  "TAILDRIVE_AUTO_SHARE",
  "WHATSAPP_PAIR_MAX_FAILURES",
  "WHATSAPP_PAIR_FAILURE_WINDOW_MS",
  "WHATSAPP_PAIR_BLOCK_DURATION_MS",
  "VAULT_ENABLED",
  "VAULT_PATH",
  "VAULT_FOLDER_INBOX",
  "VAULT_FOLDER_DAILY",
  "VAULT_FOLDER_PROJECTS",
  "VAULT_FOLDER_AREAS",
  "VAULT_FOLDER_RESOURCES",
  "VAULT_FOLDER_ARCHIVE",
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
    delete Bun.env.TAILDRIVE_ENABLED
    delete Bun.env.TAILDRIVE_SHARE_NAME
    delete Bun.env.TAILDRIVE_AUTO_SHARE
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
    Bun.env.OPENCODE_CONFIG_DIR = ".data/vault/99-system/99-pocketbrain"
    Bun.env.DATA_DIR = ".data"
    Bun.env.ENABLE_WHATSAPP = "true"
    Bun.env.WHATSAPP_AUTH_DIR = ".data/whatsapp-auth"
    Bun.env.WHATSAPP_PAIR_MAX_FAILURES = "4"
    Bun.env.WHATSAPP_PAIR_FAILURE_WINDOW_MS = "120000"
    Bun.env.WHATSAPP_PAIR_BLOCK_DURATION_MS = "300000"
    Bun.env.WHATSAPP_WHITELIST_NUMBERS = "15551234567,+44 7700 900123,15551234567"
    Bun.env.WHATSAPP_WHITELIST_NUMBER = "12025550123"
    Bun.env.TAILDRIVE_ENABLED = "true"
    Bun.env.TAILDRIVE_SHARE_NAME = "vault"
    Bun.env.TAILDRIVE_AUTO_SHARE = "true"
    Bun.env.VAULT_ENABLED = "true"
    Bun.env.VAULT_PATH = ".data/vault"
    Bun.env.VAULT_FOLDER_INBOX = "00-inbox"
    Bun.env.VAULT_FOLDER_DAILY = "01-daily-journey"
    Bun.env.VAULT_FOLDER_PROJECTS = "02-projects"
    Bun.env.VAULT_FOLDER_AREAS = "03-areas"
    Bun.env.VAULT_FOLDER_RESOURCES = "04-resources"
    Bun.env.VAULT_FOLDER_ARCHIVE = "05-archive"

    const config = loadConfig()
    expect(config.logLevel).toBe("debug")
    expect(config.opencodePort).toBe(4096)
    expect(config.opencodeServerUrl).toBe("http://127.0.0.1:4096")
    expect(config.opencodeModel).toBe("openai/gpt-5")
    expect(config.opencodeConfigDir).toContain(".data/vault/99-system/99-pocketbrain")
    expect(config.whatsAppPairMaxFailures).toBe(4)
    expect(config.whatsAppPairFailureWindowMs).toBe(120000)
    expect(config.whatsAppPairBlockDurationMs).toBe(300000)
    expect(config.whatsAppWhitelistNumbers).toEqual(["15551234567", "447700900123", "12025550123"])
    expect(config.taildriveEnabled).toBe(true)
    expect(config.taildriveShareName).toBe("vault")
    expect(config.taildriveAutoShare).toBe(true)
    expect(config.vaultFolders.projects).toBe("02-projects")
    expect(config.vaultFolders.daily).toBe("01-daily-journey")
    expect(config.vaultFolders.journal).toBe("01-daily-journey")
  })

  test("uses generic vault folder defaults", () => {
    delete Bun.env.OPENCODE_CONFIG_DIR
    delete Bun.env.VAULT_FOLDER_INBOX
    delete Bun.env.VAULT_FOLDER_DAILY
    delete Bun.env.VAULT_FOLDER_PROJECTS
    delete Bun.env.VAULT_FOLDER_AREAS
    delete Bun.env.VAULT_FOLDER_RESOURCES
    delete Bun.env.VAULT_FOLDER_ARCHIVE

    const config = loadConfig()
    expect(config.vaultFolders).toEqual({
      inbox: "inbox",
      daily: "daily",
      journal: "daily",
      projects: "projects",
      areas: "areas",
      resources: "resources",
      archive: "archive",
    })
    expect(config.opencodeConfigDir).toContain(".data/vault/99-system/99-pocketbrain")
  })

  test("uses explicit OPENCODE_CONFIG_DIR when provided", () => {
    Bun.env.OPENCODE_CONFIG_DIR = "/tmp/pocketbrain-opencode"

    const config = loadConfig()
    expect(config.opencodeConfigDir).toBe("/tmp/pocketbrain-opencode")
  })

  test("throws when WHATSAPP_PAIR_MAX_FAILURES is invalid", () => {
    Bun.env.WHATSAPP_PAIR_MAX_FAILURES = "0"
    expect(() => loadConfig()).toThrow()
  })
})
