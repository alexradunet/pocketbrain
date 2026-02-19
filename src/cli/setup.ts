/**
 * Setup CLI
 * Interactive setup for PocketBrain configuration.
 */

import { DisconnectReason, useMultiFileAuthState } from "@whiskeysockets/baileys"
import makeWASocket from "@whiskeysockets/baileys"
import qrcode from "qrcode-terminal"
import pino from "pino"
import { mkdirSync } from "node:fs"
import { join, resolve } from "node:path"
import { SQLiteChannelRepository } from "../adapters/persistence/repositories"

type EnvMap = Record<string, string>
type OpenCodeProfileMode = "isolated" | "system"

const REPO_ROOT = resolve(import.meta.dir, "..", "..")
const ENV_FILE = join(REPO_ROOT, ".env")

function ask(promptText: string): string {
  const value = prompt(promptText)
  return (value ?? "").trim()
}

function parseEnv(lines: string[]): EnvMap {
  const out: EnvMap = {}
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith("#")) continue
    const idx = trimmed.indexOf("=")
    if (idx <= 0) continue
    const key = trimmed.slice(0, idx).trim()
    const value = trimmed.slice(idx + 1)
    out[key] = value
  }
  return out
}

function updateEnvLines(lines: string[], updates: EnvMap): string[] {
  const out = [...lines]
  const seen = new Set<string>()
  for (let i = 0; i < out.length; i += 1) {
    const line = out[i]
    const idx = line.indexOf("=")
    if (idx <= 0) continue
    const key = line.slice(0, idx).trim()
    if (Object.prototype.hasOwnProperty.call(updates, key)) {
      out[i] = `${key}=${updates[key]}`
      seen.add(key)
    }
  }
  for (const [key, value] of Object.entries(updates)) {
    if (!seen.has(key)) out.push(`${key}=${value}`)
  }
  return out
}

async function loadEnvFile(): Promise<string[]> {
  try {
    return (await Bun.file(ENV_FILE).text()).split(/\r?\n/)
  } catch (error) {
    if (Bun.env.DEBUG) {
      console.debug("[setup] Could not load .env file:", error)
    }
    return []
  }
}

async function saveEnvFile(lines: string[]): Promise<void> {
  await Bun.write(ENV_FILE, lines.join("\n").trimEnd() + "\n")
}

type OpenCodeEnv = {
  OPENCODE_MODEL?: string
  OPENCODE_CONFIG_DIR?: string
  XDG_STATE_HOME?: string
  XDG_DATA_HOME?: string
  XDG_CONFIG_HOME?: string
}

function buildOpencodeEnv(overrides: OpenCodeEnv): NodeJS.ProcessEnv {
  return {
    ...process.env,
    ...overrides,
  }
}

function resolveOpencodeEnv(current: EnvMap, mode: OpenCodeProfileMode): OpenCodeEnv {
  if (mode === "system") {
    return {
      OPENCODE_MODEL: current.OPENCODE_MODEL || Bun.env.OPENCODE_MODEL,
    }
  }

  const dataDir = current.DATA_DIR || Bun.env.DATA_DIR || ".data"
  const xdgStateHome = current.XDG_STATE_HOME || `${dataDir}/opencode-state`
  const xdgDataHome = current.XDG_DATA_HOME || `${dataDir}/opencode-data`
  const xdgConfigHome = current.XDG_CONFIG_HOME || `${dataDir}/opencode-config-home`
  const opencodeConfigDir = current.OPENCODE_CONFIG_DIR || `${dataDir}/opencode-config`

  return {
    OPENCODE_MODEL: current.OPENCODE_MODEL || Bun.env.OPENCODE_MODEL,
    OPENCODE_CONFIG_DIR: resolve(process.cwd(), opencodeConfigDir),
    XDG_STATE_HOME: resolve(process.cwd(), xdgStateHome),
    XDG_DATA_HOME: resolve(process.cwd(), xdgDataHome),
    XDG_CONFIG_HOME: resolve(process.cwd(), xdgConfigHome),
  }
}

function ensureOpencodeDirs(opencodeEnv: OpenCodeEnv): void {
  if (opencodeEnv.OPENCODE_CONFIG_DIR) mkdirSync(opencodeEnv.OPENCODE_CONFIG_DIR, { recursive: true })
  if (opencodeEnv.XDG_STATE_HOME) mkdirSync(opencodeEnv.XDG_STATE_HOME, { recursive: true })
  if (opencodeEnv.XDG_DATA_HOME) mkdirSync(opencodeEnv.XDG_DATA_HOME, { recursive: true })
  if (opencodeEnv.XDG_CONFIG_HOME) mkdirSync(opencodeEnv.XDG_CONFIG_HOME, { recursive: true })
}

async function findRecentModel(env: OpenCodeEnv): Promise<string | null> {
  const home = Bun.env.HOME ?? ""
  const stateHome = env.XDG_STATE_HOME ?? Bun.env.XDG_STATE_HOME ?? join(home, ".local", "state")
  const modelFile = join(stateHome, "opencode", "model.json")
  try {
    const parsed = (await Bun.file(modelFile).json()) as {
      recent?: Array<{ providerID?: string; modelID?: string }>
    }
    const first = parsed.recent?.[0]
    if (first?.providerID && first?.modelID) return `${first.providerID}/${first.modelID}`
  } catch (error) {
    // File missing or malformed – fall through.
    if (Bun.env.DEBUG) {
      console.debug("[setup] Could not load recent model:", error)
    }
  }
  return null
}

async function resolveModel(env: OpenCodeEnv): Promise<string> {
  const modelFromEnv = env.OPENCODE_MODEL?.trim() || Bun.env.OPENCODE_MODEL?.trim()
  if (modelFromEnv) return modelFromEnv
  return (await findRecentModel(env)) ?? ""
}

async function runOpencodeSetupWizard(opencodeEnv: OpenCodeEnv): Promise<void> {
  const before = await resolveModel(opencodeEnv)
  if (before) {
    console.log(`Current OpenCode model: ${before}`)
  } else {
    console.log("No OpenCode model configured yet for PocketBrain.")
  }

  const isolated = !!opencodeEnv.XDG_STATE_HOME
  console.log(`Launching ${isolated ? "isolated" : "system"} OpenCode setup for PocketBrain...`)
  console.log("In OpenCode, run /providers then /models, then quit to continue setup.")

  const env = buildOpencodeEnv(opencodeEnv)
  const proc = Bun.spawn(["opencode"], {
    stdin: "inherit",
    stdout: "inherit",
    stderr: "inherit",
    env,
  })
  await proc.exited

  const after = await resolveModel(opencodeEnv)
  if (!after) {
    console.log("Model still not selected. You can rerun: bun run setup")
  }
}

async function waitForWhatsAppOpen(
  sock: ReturnType<typeof makeWASocket>,
  showQr: boolean,
): Promise<"open" | "restart"> {
  return await new Promise((resolve, reject) => {
    type DisconnectError = {
      output?: { statusCode?: number }
      message?: string
    }

    const timer = setTimeout(() => reject(new Error("WhatsApp QR timeout")), 2 * 60_000)
    sock.ev.on("connection.update", (update) => {
      const { connection, lastDisconnect, qr } = update
      if (showQr && qr) {
        qrcode.generate(qr, { small: true })
      }
      if (connection === "open") {
        clearTimeout(timer)
        resolve("open")
      }
      if (connection === "close") {
        const disconnectError = lastDisconnect?.error as DisconnectError | undefined
        const statusCode = disconnectError?.output?.statusCode
        const message = disconnectError?.message
        const streamError = message?.includes("Stream Errored")
        if (streamError) {
          clearTimeout(timer)
          resolve("restart")
          return
        }
        if (statusCode === DisconnectReason.loggedOut) {
          clearTimeout(timer)
          reject(new Error("WhatsApp logged out"))
        }
      }
    })
  })
}

async function setupWhatsApp(authDir: string): Promise<string> {
  mkdirSync(authDir, { recursive: true })
  const { state, saveCreds } = await useMultiFileAuthState(authDir)

  let sock = makeWASocket({
    auth: state,
    logger: pino({ level: "error" }),
  })
  sock.ev.on("creds.update", saveCreds)
  let userID = ""

  try {
    const first = await waitForWhatsAppOpen(sock, true)
    userID = sock.user?.id ?? ""
    if (first === "restart") {
      sock.end?.(new Error("restart"))
      sock = makeWASocket({
        auth: state,
        logger: pino({ level: "error" }),
      })
      sock.ev.on("creds.update", saveCreds)
      await waitForWhatsAppOpen(sock, false)
      userID = sock.user?.id ?? userID
    }
  } finally {
    sock.end?.(new Error("setup complete"))
  }
  return userID
}

async function main(): Promise<void> {
  // Create repository for setup
  const channelRepository = new SQLiteChannelRepository()
  
  const lines = await loadEnvFile()
  const current = parseEnv(lines)
  const updates: EnvMap = {}

  const profileModeInput = ask("OpenCode profile mode: isolated for PocketBrain or system shared? (I/s): ")
  const profileMode: OpenCodeProfileMode = profileModeInput.toLowerCase().startsWith("s") ? "system" : "isolated"
  const opencodeEnv = resolveOpencodeEnv(current, profileMode)
  ensureOpencodeDirs(opencodeEnv)

  updates.OPENCODE_PROFILE = profileMode
  updates.OPENCODE_CONFIG_DIR = opencodeEnv.OPENCODE_CONFIG_DIR ?? current.OPENCODE_CONFIG_DIR ?? "."
  updates.XDG_STATE_HOME = opencodeEnv.XDG_STATE_HOME ?? ""
  updates.XDG_DATA_HOME = opencodeEnv.XDG_DATA_HOME ?? ""
  updates.XDG_CONFIG_HOME = opencodeEnv.XDG_CONFIG_HOME ?? ""

  const enableWhatsApp = ask("Enable WhatsApp? (y/N): ")
  if (enableWhatsApp.toLowerCase().startsWith("y")) {
    updates.ENABLE_WHATSAPP = "true"
    const dataDir = current.DATA_DIR || Bun.env.DATA_DIR || ".data"
    const authDir = current.WHATSAPP_AUTH_DIR || `${dataDir}/whatsapp-auth`
    updates.WHATSAPP_AUTH_DIR = authDir
    console.log("Scan the QR to connect WhatsApp...")
    const waUserID = await setupWhatsApp(resolve(process.cwd(), authDir))
    if (waUserID) {
      channelRepository.saveLastChannel("whatsapp", waUserID)
    }
    console.log("WhatsApp connected.")
  } else {
    updates.ENABLE_WHATSAPP = "false"
  }

  console.log("WHITELIST_PAIR_TOKEN allows users to self-pair via '/pair <token>' in chat.")
  const pairTokenPrompt = current.WHITELIST_PAIR_TOKEN
    ? "Whitelist pair token (leave blank to keep current): "
    : "Whitelist pair token (leave blank to disable): "
  const pairToken = ask(pairTokenPrompt)
  if (pairToken) updates.WHITELIST_PAIR_TOKEN = pairToken

  if (updates.ENABLE_WHATSAPP !== "true") {
    console.log("No channels enabled. Aborting setup.")
    process.exit(1)
  }

  const merged = updateEnvLines(lines, updates)
  await saveEnvFile(merged)

  await runOpencodeSetupWizard(opencodeEnv)

  console.log("Setup complete. Run: bun run dev")
  process.exit(0)
}

void main().catch((error) => {
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
})
