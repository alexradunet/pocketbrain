/**
 * Application Configuration
 * Centralized configuration with schema validation.
 */

import { isAbsolute, join } from "node:path"
import { z } from "zod"
import type { VaultFolders } from "./vault/vault-service"

const LOG_LEVEL_VALUES = ["fatal", "error", "warn", "info", "debug", "trace", "silent"] as const

export interface AppConfig {
  appName: string
  logLevel: string
  dataDir: string
  opencodeModel: string | undefined
  opencodeConfigDir: string
  opencodeServerUrl: string | undefined
  opencodeHostname: string
  opencodePort: number
  heartbeatIntervalMinutes: number
  heartbeatBaseDelayMs: number
  heartbeatMaxDelayMs: number
  heartbeatNotifyAfterFailures: number
  enableWhatsApp: boolean
  whatsAppAuthDir: string
  messageMaxLength: number
  messageChunkDelayMs: number
  messageRateLimitMs: number
  outboxIntervalMs: number
  outboxMaxRetries: number
  outboxRetryBaseDelayMs: number
  connectionTimeoutMs: number
  connectionReconnectDelayMs: number
  whitelistPairToken: string | undefined
  whatsAppWhitelistNumbers: string[]
  syncthingEnabled: boolean
  syncthingBaseUrl: string
  syncthingApiKey: string | undefined
  syncthingTimeoutMs: number
  syncthingVaultFolderId: string | undefined
  syncthingAutoStart: boolean
  syncthingMutationToolsEnabled: boolean
  syncthingAllowedFolderIds: string[]
  vaultPath: string
  vaultEnabled: boolean
  vaultFolders: VaultFolders
}

const DEFAULTS = {
  appName: "pocketbrain",
  logLevel: "info",
  opencodeHostname: "127.0.0.1",
  opencodePort: 4096,
  heartbeatIntervalMinutes: 30,
  heartbeatBaseDelayMs: 60_000,
  heartbeatMaxDelayMs: 30 * 60 * 1000,
  heartbeatNotifyAfterFailures: 3,
  messageMaxLength: 3500,
  messageChunkDelayMs: 500,
  messageRateLimitMs: 1000,
  outboxIntervalMs: 60_000,
  outboxMaxRetries: 3,
  outboxRetryBaseDelayMs: 60_000,
  connectionTimeoutMs: 20_000,
  connectionReconnectDelayMs: 3000,
  syncthingBaseUrl: "http://127.0.0.1:8384",
  syncthingTimeoutMs: 5000,
  syncthingVaultFolderId: "vault",
  syncthingAutoStart: true,
  pocketBrainVaultHomeRelative: "99-system/99-pocketbrain",
  vaultFolders: {
    inbox: "inbox",
    daily: "daily",
    journal: "daily",
    projects: "projects",
    areas: "areas",
    resources: "resources",
    archive: "archive",
  },
} as const

const modelRefPattern = /^[^/]+\/.+$/

const AppConfigSchema = z
  .object({
    appName: z.string().min(1),
    logLevel: z.enum(LOG_LEVEL_VALUES),
    dataDir: z.string().min(1),
    opencodeModel: z.string().regex(modelRefPattern, "OPENCODE_MODEL must use provider/model format").optional(),
    opencodeConfigDir: z.string().min(1),
    opencodeServerUrl: z
      .string()
      .url("OPENCODE_SERVER_URL must be a valid URL")
      .refine((v) => v.startsWith("http://") || v.startsWith("https://"), {
        message: "OPENCODE_SERVER_URL must use http or https",
      })
      .optional(),
    opencodeHostname: z.string().min(1),
    opencodePort: z.number().int().min(1).max(65_535),
    heartbeatIntervalMinutes: z.number().int().min(1),
    heartbeatBaseDelayMs: z.number().positive(),
    heartbeatMaxDelayMs: z.number().positive(),
    heartbeatNotifyAfterFailures: z.number().int().positive(),
    enableWhatsApp: z.boolean(),
    whatsAppAuthDir: z.string().min(1),
    messageMaxLength: z.number().int().positive(),
    messageChunkDelayMs: z.number().int().nonnegative(),
    messageRateLimitMs: z.number().int().nonnegative(),
    outboxIntervalMs: z.number().int().positive(),
    outboxMaxRetries: z.number().int().positive(),
    outboxRetryBaseDelayMs: z.number().int().positive(),
    connectionTimeoutMs: z.number().positive(),
    connectionReconnectDelayMs: z.number().nonnegative(),
    whitelistPairToken: z.string().optional(),
    whatsAppWhitelistNumbers: z.array(z.string().regex(/^\d+$/)),
    syncthingEnabled: z.boolean(),
    syncthingBaseUrl: z.string().url("SYNCTHING_BASE_URL must be a valid URL"),
    syncthingApiKey: z.string().optional(),
    syncthingTimeoutMs: z.number().int().positive(),
    syncthingVaultFolderId: z.string().optional(),
    syncthingAutoStart: z.boolean(),
    syncthingMutationToolsEnabled: z.boolean(),
    syncthingAllowedFolderIds: z.array(z.string().min(1)),
    vaultPath: z.string().min(1),
    vaultEnabled: z.boolean(),
    vaultFolders: z.object({
      inbox: z.string().min(1),
      daily: z.string().min(1),
      journal: z.string().min(1),
      projects: z.string().min(1),
      areas: z.string().min(1),
      resources: z.string().min(1),
      archive: z.string().min(1),
    }),
  })
  .superRefine((cfg, ctx) => {
    if (cfg.enableWhatsApp && !cfg.whatsAppAuthDir.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["whatsAppAuthDir"],
        message: "WHATSAPP_AUTH_DIR cannot be empty when ENABLE_WHATSAPP=true",
      })
    }

    if (cfg.vaultEnabled && !cfg.vaultPath.trim()) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["vaultPath"],
        message: "VAULT_PATH cannot be empty when VAULT_ENABLED=true",
      })
    }

    if (cfg.syncthingEnabled) {
      if (!cfg.syncthingApiKey?.trim()) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["syncthingApiKey"],
          message: "SYNCTHING_API_KEY is required when SYNCTHING_ENABLED=true",
        })
      }

      if (cfg.syncthingMutationToolsEnabled && cfg.syncthingAllowedFolderIds.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["syncthingAllowedFolderIds"],
          message: "SYNCTHING_ALLOWED_FOLDER_IDS is required when SYNCTHING_MUTATION_TOOLS_ENABLED=true",
        })
      }
    }
  })

function envBool(value: string | undefined, fallback = false): boolean {
  if (!value) return fallback
  const v = value.trim().toLowerCase()
  return v === "1" || v === "true" || v === "yes" || v === "on"
}

function envInt(value: string | undefined, fallback: number): number {
  if (!value || value.trim().length === 0) return fallback
  const n = Number.parseInt(value, 10)
  return Number.isFinite(n) ? n : fallback
}

function envString(value: string | undefined, fallback: string): string {
  return value?.trim() || fallback
}

function optionalTrimmed(value: string | undefined): string | undefined {
  const trimmed = value?.trim()
  return trimmed ? trimmed : undefined
}

function parsePhoneWhitelist(values: Array<string | undefined>): string[] {
  const unique = new Set<string>()

  for (const value of values) {
    if (!value) continue
    for (const item of value.split(",")) {
      const normalized = item.replace(/\D/g, "").trim()
      if (normalized) {
        unique.add(normalized)
      }
    }
  }

  return [...unique]
}

function parseCommaSeparatedList(value: string | undefined): string[] {
  if (!value) return []
  const unique = new Set<string>()
  for (const item of value.split(",")) {
    const normalized = item.trim()
    if (normalized) unique.add(normalized)
  }
  return [...unique]
}

function resolvePath(cwd: string, value: string): string {
  return isAbsolute(value) ? value : join(cwd, value)
}

function resolveVaultFoldersFromEnv(): VaultFolders {
  const daily = envString(Bun.env.VAULT_FOLDER_DAILY, DEFAULTS.vaultFolders.daily)

  return {
    inbox: envString(Bun.env.VAULT_FOLDER_INBOX, DEFAULTS.vaultFolders.inbox),
    daily,
    journal: daily,
    projects: envString(Bun.env.VAULT_FOLDER_PROJECTS, DEFAULTS.vaultFolders.projects),
    areas: envString(Bun.env.VAULT_FOLDER_AREAS, DEFAULTS.vaultFolders.areas),
    resources: envString(Bun.env.VAULT_FOLDER_RESOURCES, DEFAULTS.vaultFolders.resources),
    archive: envString(Bun.env.VAULT_FOLDER_ARCHIVE, DEFAULTS.vaultFolders.archive),
  }
}

export function loadConfig(): AppConfig {
  const cwd = process.cwd()
  const dataDir = resolvePath(cwd, envString(Bun.env.DATA_DIR, ".data"))
  const whatsAppAuthDir = optionalTrimmed(Bun.env.WHATSAPP_AUTH_DIR)
  const vaultPath = optionalTrimmed(Bun.env.VAULT_PATH)
  const opencodeConfigDirRaw = optionalTrimmed(Bun.env.OPENCODE_CONFIG_DIR)
  const resolvedVaultPath = vaultPath ? resolvePath(cwd, vaultPath) : join(dataDir, "vault")
  const vaultEnabled = envBool(Bun.env.VAULT_ENABLED, true)
  const opencodeConfigDir = opencodeConfigDirRaw
    ? resolvePath(cwd, opencodeConfigDirRaw)
    : vaultEnabled
      ? join(resolvedVaultPath, DEFAULTS.pocketBrainVaultHomeRelative)
      : cwd

  const candidate: AppConfig = {
    appName: envString(Bun.env.APP_NAME, DEFAULTS.appName),
    logLevel: envString(Bun.env.LOG_LEVEL, DEFAULTS.logLevel),
    dataDir,
    opencodeModel: optionalTrimmed(Bun.env.OPENCODE_MODEL),
    opencodeConfigDir,
    opencodeServerUrl: optionalTrimmed(Bun.env.OPENCODE_SERVER_URL),
    opencodeHostname: envString(Bun.env.OPENCODE_HOSTNAME, DEFAULTS.opencodeHostname),
    opencodePort: envInt(Bun.env.OPENCODE_PORT, DEFAULTS.opencodePort),
    heartbeatIntervalMinutes: envInt(Bun.env.HEARTBEAT_INTERVAL_MINUTES, DEFAULTS.heartbeatIntervalMinutes),
    heartbeatBaseDelayMs: DEFAULTS.heartbeatBaseDelayMs,
    heartbeatMaxDelayMs: DEFAULTS.heartbeatMaxDelayMs,
    heartbeatNotifyAfterFailures: DEFAULTS.heartbeatNotifyAfterFailures,
    enableWhatsApp: envBool(Bun.env.ENABLE_WHATSAPP, false),
    whatsAppAuthDir: whatsAppAuthDir ? resolvePath(cwd, whatsAppAuthDir) : join(dataDir, "whatsapp-auth"),
    messageMaxLength: DEFAULTS.messageMaxLength,
    messageChunkDelayMs: DEFAULTS.messageChunkDelayMs,
    messageRateLimitMs: DEFAULTS.messageRateLimitMs,
    outboxIntervalMs: DEFAULTS.outboxIntervalMs,
    outboxMaxRetries: DEFAULTS.outboxMaxRetries,
    outboxRetryBaseDelayMs: DEFAULTS.outboxRetryBaseDelayMs,
    connectionTimeoutMs: DEFAULTS.connectionTimeoutMs,
    connectionReconnectDelayMs: DEFAULTS.connectionReconnectDelayMs,
    whitelistPairToken: optionalTrimmed(Bun.env.WHITELIST_PAIR_TOKEN),
    whatsAppWhitelistNumbers: parsePhoneWhitelist([
      Bun.env.WHATSAPP_WHITELIST_NUMBERS,
      Bun.env.WHATSAPP_WHITELIST_NUMBER,
    ]),
    syncthingEnabled: envBool(Bun.env.SYNCTHING_ENABLED, false),
    syncthingBaseUrl: envString(Bun.env.SYNCTHING_BASE_URL, DEFAULTS.syncthingBaseUrl),
    syncthingApiKey: optionalTrimmed(Bun.env.SYNCTHING_API_KEY),
    syncthingTimeoutMs: envInt(Bun.env.SYNCTHING_TIMEOUT_MS, DEFAULTS.syncthingTimeoutMs),
    syncthingVaultFolderId:
      optionalTrimmed(Bun.env.SYNCTHING_VAULT_FOLDER_ID) ||
      (envBool(Bun.env.SYNCTHING_ENABLED, false) ? DEFAULTS.syncthingVaultFolderId : undefined),
    syncthingAutoStart: envBool(Bun.env.SYNCTHING_AUTO_START, DEFAULTS.syncthingAutoStart),
    syncthingMutationToolsEnabled: envBool(Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED, false),
    syncthingAllowedFolderIds: parseCommaSeparatedList(Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS),
    vaultPath: resolvedVaultPath,
    vaultEnabled,
    vaultFolders: resolveVaultFoldersFromEnv(),
  }

  const parsed = AppConfigSchema.parse(candidate)
  return {
    ...parsed,
    opencodeModel: parsed.opencodeModel,
    opencodeServerUrl: parsed.opencodeServerUrl,
    whitelistPairToken: parsed.whitelistPairToken,
    whatsAppWhitelistNumbers: parsed.whatsAppWhitelistNumbers,
    syncthingApiKey: parsed.syncthingApiKey,
    syncthingVaultFolderId: parsed.syncthingVaultFolderId,
  }
}

export async function findRecentModel(): Promise<string | null> {
  const home = Bun.env.HOME ?? ""
  const stateHome = Bun.env.XDG_STATE_HOME ?? join(home, ".local", "state")
  const modelFile = join(stateHome, "opencode", "model.json")

  try {
    const parsed = (await Bun.file(modelFile).json()) as {
      recent?: Array<{ providerID?: string; modelID?: string }>
    }
    const first = parsed.recent?.[0]
    if (first?.providerID && first?.modelID) {
      return `${first.providerID}/${first.modelID}`
    }
  } catch {
  }
  return null
}

export async function resolveModel(): Promise<string | null> {
  const modelFromEnv = optionalTrimmed(Bun.env.OPENCODE_MODEL)
  if (modelFromEnv) return modelFromEnv
  return findRecentModel()
}
