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
  "SYNCTHING_ENABLED",
  "SYNCTHING_BASE_URL",
  "SYNCTHING_API_KEY",
  "SYNCTHING_TIMEOUT_MS",
  "SYNCTHING_VAULT_FOLDER_ID",
  "SYNCTHING_AUTO_START",
  "SYNCTHING_MUTATION_TOOLS_ENABLED",
  "SYNCTHING_ALLOWED_FOLDER_IDS",
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
    Bun.env.SYNCTHING_ENABLED = "false"
    delete Bun.env.SYNCTHING_API_KEY
    delete Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED
    delete Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS
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
    Bun.env.WHATSAPP_WHITELIST_NUMBERS = "15551234567,+44 7700 900123,15551234567"
    Bun.env.WHATSAPP_WHITELIST_NUMBER = "12025550123"
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_BASE_URL = "http://127.0.0.1:8384"
    Bun.env.SYNCTHING_API_KEY = "test-api-key"
    Bun.env.SYNCTHING_TIMEOUT_MS = "7000"
    Bun.env.SYNCTHING_VAULT_FOLDER_ID = "vault"
    Bun.env.SYNCTHING_AUTO_START = "true"
    Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED = "true"
    Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS = "vault,notes"
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
    expect(config.whatsAppWhitelistNumbers).toEqual(["15551234567", "447700900123", "12025550123"])
    expect(config.syncthingEnabled).toBe(true)
    expect(config.syncthingBaseUrl).toBe("http://127.0.0.1:8384")
    expect(config.syncthingTimeoutMs).toBe(7000)
    expect(config.syncthingVaultFolderId).toBe("vault")
    expect(config.syncthingAutoStart).toBe(true)
    expect(config.syncthingMutationToolsEnabled).toBe(true)
    expect(config.syncthingAllowedFolderIds).toEqual(["vault", "notes"])
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

  test("defaults Syncthing vault folder ID to vault when enabled", () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = "test-api-key"
    Bun.env.SYNCTHING_VAULT_FOLDER_ID = ""

    const config = loadConfig()
    expect(config.syncthingVaultFolderId).toBe("vault")
  })

  test("throws when Syncthing is enabled without API key", () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = ""
    expect(() => loadConfig()).toThrow()
  })

  test("throws when Syncthing mutation tools enabled without allowlist", () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = "test-api-key"
    Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED = "true"
    Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS = ""
    expect(() => loadConfig()).toThrow()
  })
})
