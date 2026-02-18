/**
 * Application Configuration
 * Centralized configuration with validation and defaults.
 */

import { join } from "node:path"

// Configuration types
export interface AppConfig {
  appName: string
  logLevel: string
  
  // Server settings
  opencodeModel: string | undefined
  opencodeServerUrl: string | undefined
  opencodeHostname: string
  opencodePort: number
  
  // Heartbeat settings
  heartbeatIntervalMinutes: number
  heartbeatBaseDelayMs: number
  heartbeatMaxDelayMs: number
  heartbeatNotifyAfterFailures: number
  
  // WhatsApp settings
  enableWhatsApp: boolean
  whatsAppAuthDir: string
  
  // Message settings
  messageMaxLength: number
  messageChunkDelayMs: number
  messageRateLimitMs: number
  
  // Outbox settings
  outboxIntervalMs: number
  outboxMaxRetries: number
  outboxRetryBaseDelayMs: number
  
  // Connection settings
  connectionTimeoutMs: number
  connectionReconnectDelayMs: number
  
  // Security settings
  whitelistPairToken: string | undefined
  
  // Vault settings
  vaultPath: string
  vaultEnabled: boolean
}

// Default configuration values
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
} as const

// Environment variable parsers
function envBool(value: string | undefined, fallback = false): boolean {
  if (!value) return fallback
  const v = value.trim().toLowerCase()
  return v === "1" || v === "true" || v === "yes" || v === "on"
}

function envInt(value: string | undefined, fallback: number): number {
  if (!value) return fallback
  const n = Number.parseInt(value, 10)
  return Number.isFinite(n) ? n : fallback
}

function envString(value: string | undefined, fallback: string): string {
  return value?.trim() || fallback
}

/**
 * Load configuration from environment variables
 */
export function loadConfig(): AppConfig {
  const cwd = process.cwd()
  
  return {
    appName: envString(Bun.env.APP_NAME, DEFAULTS.appName),
    logLevel: envString(Bun.env.LOG_LEVEL, DEFAULTS.logLevel),
    
    opencodeModel: Bun.env.OPENCODE_MODEL?.trim() || undefined,
    opencodeServerUrl: Bun.env.OPENCODE_SERVER_URL?.trim() || undefined,
    opencodeHostname: envString(Bun.env.OPENCODE_HOSTNAME, DEFAULTS.opencodeHostname),
    opencodePort: envInt(Bun.env.OPENCODE_PORT, DEFAULTS.opencodePort),
    
    heartbeatIntervalMinutes: envInt(
      Bun.env.HEARTBEAT_INTERVAL_MINUTES, 
      DEFAULTS.heartbeatIntervalMinutes
    ),
    heartbeatBaseDelayMs: DEFAULTS.heartbeatBaseDelayMs,
    heartbeatMaxDelayMs: DEFAULTS.heartbeatMaxDelayMs,
    heartbeatNotifyAfterFailures: DEFAULTS.heartbeatNotifyAfterFailures,
    
    enableWhatsApp: envBool(Bun.env.ENABLE_WHATSAPP, false),
    whatsAppAuthDir: join(
      cwd, 
      envString(Bun.env.WHATSAPP_AUTH_DIR, ".data/whatsapp-auth")
    ),
    
    messageMaxLength: DEFAULTS.messageMaxLength,
    messageChunkDelayMs: DEFAULTS.messageChunkDelayMs,
    messageRateLimitMs: DEFAULTS.messageRateLimitMs,
    
    outboxIntervalMs: DEFAULTS.outboxIntervalMs,
    outboxMaxRetries: DEFAULTS.outboxMaxRetries,
    outboxRetryBaseDelayMs: DEFAULTS.outboxRetryBaseDelayMs,
    
    connectionTimeoutMs: DEFAULTS.connectionTimeoutMs,
    connectionReconnectDelayMs: DEFAULTS.connectionReconnectDelayMs,
    
    whitelistPairToken: Bun.env.WHITELIST_PAIR_TOKEN?.trim() || undefined,
    
    // Vault configuration
    vaultPath: envString(Bun.env.VAULT_PATH, "/data/vault"),
    vaultEnabled: envBool(Bun.env.VAULT_ENABLED, true),
  }
}

/**
 * Find recent model from OpenCode state
 */
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
  } catch (error) {
    // File missing or malformed – fall through
    if (Bun.env.DEBUG) {
      console.debug("[config] Could not load recent model:", error)
    }
  }
  return null
}

/**
 * Resolve model from environment or state
 */
export async function resolveModel(): Promise<string | null> {
  const modelFromEnv = Bun.env.OPENCODE_MODEL?.trim()
  if (modelFromEnv) return modelFromEnv
  return findRecentModel()
}
