/**
 * PocketBrain Entry Point
 * 
 * Composition root that wires together all dependencies.
 * Follows Dependency Injection pattern.
 */

import pino from "pino"
import { mkdir, readdir, readFile, writeFile } from "node:fs/promises"
import { join } from "node:path"
import { loadConfig } from "./config"

// Core imports
import { AssistantCore, ChannelManager } from "./core"
import { RuntimeProvider } from "./core/runtime-provider"
import { SessionManager } from "./core/session-manager"
import { PromptBuilder } from "./core/prompt-builder"
import { MessageChunker, MessageSender } from "./core/services"

// Repository imports
import {
  SQLiteMemoryRepository,
  SQLiteChannelRepository,
  SQLiteSessionRepository,
  SQLiteWhitelistRepository,
  SQLiteOutboxRepository,
  SQLiteHeartbeatRepository,
} from "./adapters/persistence/repositories"

// Channel imports
import { RateLimiter } from "./adapters/channels/rate-limiter"
import { WhatsAppAdapter } from "./adapters/channels/whatsapp/adapter"
import { ensureSyncthingAvailable } from "./adapters/syncthing/bootstrap"

// Scheduler imports
import { HeartbeatScheduler } from "./scheduler/heartbeat"

// Vault imports
import { createVaultService, type ObsidianConfigSummary, VaultService } from "./vault/vault-service"
import { vaultProvider } from "./vault/vault-provider"

const OPENCODE_PLUGIN_RELATIVE_PATHS = [
  "./src/adapters/plugins/install-skill.plugin.ts",
  "./src/adapters/plugins/memory.plugin.ts",
  "./src/adapters/plugins/channel-message.plugin.ts",
  "./src/adapters/plugins/vault.plugin.ts",
  "./src/adapters/plugins/syncthing.plugin.ts",
]

async function main(): Promise<void> {
  // Load configuration
  const cfg = loadConfig()
  process.env.OPENCODE_CONFIG_DIR = cfg.opencodeConfigDir

  await bootstrapVaultPocketBrainHome(cfg.opencodeConfigDir)

  const logger = pino({ level: cfg.logLevel })

  await ensureSyncthingAvailable({
    enabled: cfg.syncthingEnabled,
    baseUrl: cfg.syncthingBaseUrl,
    apiKey: cfg.syncthingApiKey,
    timeoutMs: cfg.syncthingTimeoutMs,
    autoStart: cfg.syncthingAutoStart,
    logger,
  })

  logger.info(
    {
      version: process.env.APP_VERSION ?? "dev",
      gitSha: process.env.GIT_SHA ?? "unknown",
    },
    "starting PocketBrain",
  )

  // Create repositories (adapters layer)
  const memoryRepository = new SQLiteMemoryRepository()
  const channelRepository = new SQLiteChannelRepository()
  const sessionRepository = new SQLiteSessionRepository()
  const whitelistRepository = new SQLiteWhitelistRepository()
  const outboxRepository = new SQLiteOutboxRepository(cfg.outboxMaxRetries)
  const heartbeatRepository = new SQLiteHeartbeatRepository()

  if (cfg.whatsAppWhitelistNumbers.length > 0) {
    let addedCount = 0
    for (const phoneNumber of cfg.whatsAppWhitelistNumbers) {
      const directJid = `${phoneNumber}@s.whatsapp.net`
      const lidJid = `${phoneNumber}@lid`
      if (whitelistRepository.addToWhitelist("whatsapp", directJid)) {
        addedCount += 1
      }
      if (whitelistRepository.addToWhitelist("whatsapp", lidJid)) {
        addedCount += 1
      }
    }
    logger.info(
      {
        configuredCount: cfg.whatsAppWhitelistNumbers.length,
        addedCount,
      },
      "applied WhatsApp whitelist from environment",
    )
  }

  // Create runtime provider
  const runtimeProvider = new RuntimeProvider({
    model: cfg.opencodeModel,
    serverUrl: cfg.opencodeServerUrl,
    hostname: cfg.opencodeHostname,
    port: cfg.opencodePort,
    logger,
  })

  // Create session manager
  const sessionManager = new SessionManager({
    repository: sessionRepository,
    logger,
  })

  // Initialize vault if enabled before assistant init so vault tools are available.
  let vaultService: VaultService | undefined
  let vaultProfile = ""

  if (cfg.vaultEnabled) {
    logger.info({ vaultPath: cfg.vaultPath, folders: cfg.vaultFolders }, "initializing vault")
    vaultService = createVaultService(cfg.vaultPath, logger, cfg.vaultFolders)
    await vaultService.initialize()
    const vaultConfig = await vaultService.getObsidianConfigSummary()
    vaultProfile = formatVaultProfile(vaultConfig)
    logger.info({ obsidianConfigFound: vaultConfig.obsidianConfigFound }, "vault profile detected")
    vaultProvider.setVaultService(vaultService)
    logger.info("vault initialized (sync via Syncthing)")
  } else {
    logger.info("vault disabled")
  }

  // Create prompt builder
  const promptBuilder = new PromptBuilder({
    heartbeatIntervalMinutes: cfg.heartbeatIntervalMinutes,
    vaultEnabled: cfg.vaultEnabled,
    vaultPath: cfg.vaultPath,
    vaultFolders: cfg.vaultFolders,
    vaultProfile,
  })

  // Create assistant core with injected dependencies
  const assistant = new AssistantCore({
    runtimeProvider,
    sessionManager,
    promptBuilder,
    memoryRepository,
    channelRepository,
    heartbeatRepository,
    logger,
  })

  // Initialize assistant
  await assistant.init()

  // Create heartbeat scheduler
  let heartbeatScheduler: HeartbeatScheduler | undefined
  const taskCount = heartbeatRepository.getTaskCount()
  
  if (taskCount === 0) {
    logger.warn(
      "No heartbeat tasks configured. Add tasks via SQL: INSERT INTO heartbeat_tasks (task) VALUES ('your task')",
    )
  } else {
    heartbeatScheduler = new HeartbeatScheduler(
      {
        intervalMinutes: cfg.heartbeatIntervalMinutes,
        baseDelayMs: cfg.heartbeatBaseDelayMs,
        maxDelayMs: cfg.heartbeatMaxDelayMs,
        notifyAfterFailures: cfg.heartbeatNotifyAfterFailures,
      },
      {
        assistant,
        outboxRepository,
        channelRepository,
        logger,
      }
    )
    heartbeatScheduler.start()
  }

  // Create channel manager
  const channelManager = new ChannelManager(logger)

  // Setup graceful shutdown
  let shuttingDown = false
  const shutdown = async (code: number) => {
    if (shuttingDown) return
    shuttingDown = true
    logger.info("shutting down...")
    
    // Stop vault service
    if (vaultService) {
      logger.info("stopping vault service...")
      await vaultService.stop()
      vaultProvider.clear()
    }
    
    await channelManager.stop()
    heartbeatScheduler?.stop()
    await assistant.close()
    
    // Finalize repository statements to release SQLite resources
    logger.debug("finalizing repositories...")
    memoryRepository.close()
    channelRepository.close()
    sessionRepository.close()
    whitelistRepository.close()
    outboxRepository.close()
    heartbeatRepository.close()
    logger.info("shutdown complete")
    
    process.exit(code)
  }

  process.on("SIGINT", () => shutdown(0))
  process.on("SIGTERM", () => shutdown(0))
  process.on("SIGHUP", () => shutdown(0))
  process.on("SIGQUIT", () => shutdown(0))
  process.on("uncaughtException", (error) => {
    logger.error({ error }, "uncaught exception")
    shutdown(1)
  })
  process.on("unhandledRejection", (reason) => {
    logger.error({ reason }, "unhandled rejection")
    shutdown(1)
  })

  // Setup WhatsApp if enabled
  if (cfg.enableWhatsApp) {
    // Create shared services
    const rateLimiter = new RateLimiter({ minIntervalMs: cfg.messageRateLimitMs })
    const messageChunker = new MessageChunker({ maxLength: cfg.messageMaxLength })
    const messageSender = new MessageSender({
      chunker: messageChunker,
      rateLimiter,
      chunkDelayMs: cfg.messageChunkDelayMs,
      logger,
    })

    // Create WhatsApp adapter
    const whatsappAdapter = new WhatsAppAdapter({
      authDir: cfg.whatsAppAuthDir,
      logger,
      whitelistRepository,
      outboxRepository,
      messageSender,
      pairToken: cfg.whitelistPairToken,
      outboxIntervalMs: cfg.outboxIntervalMs,
      outboxRetryBaseDelayMs: cfg.outboxRetryBaseDelayMs,
      connectingTimeoutMs: cfg.connectionTimeoutMs,
      reconnectDelayMs: cfg.connectionReconnectDelayMs,
    })

    channelManager.register(whatsappAdapter)

    // Start channel manager with message handler
    await channelManager.start(async (userID: string, text: string) => {
      if (text.trim() === "/new") {
        await assistant.startNewMainSession("whatsapp command")
        return "Started a new conversation session."
      }

      return assistant.ask({ channel: "whatsapp", userID, text })
    })

    logger.info("WhatsApp channel started")
  } else {
    logger.warn("No channel enabled. Set ENABLE_WHATSAPP=true.")
  }
}

async function bootstrapVaultPocketBrainHome(opencodeConfigDir: string): Promise<void> {
  await mkdir(opencodeConfigDir, { recursive: true })

  const requiredFolders = [
    ".agents/skills",
    "skills",
    "processes",
    "knowledge",
    "runbooks",
    "config",
  ]

  for (const folder of requiredFolders) {
    await mkdir(join(opencodeConfigDir, folder), { recursive: true })
  }

  await writeVaultOpencodeConfig(opencodeConfigDir)
  await seedBundledSkills(opencodeConfigDir)
}

async function writeVaultOpencodeConfig(opencodeConfigDir: string): Promise<void> {
  const configPath = join(opencodeConfigDir, "opencode.json")
  if (await Bun.file(configPath).exists()) {
    return
  }

  const plugin = OPENCODE_PLUGIN_RELATIVE_PATHS.map((relativePath) => join(process.cwd(), relativePath.slice(2)))
  const config = {
    $schema: "https://opencode.ai/config.json",
    permission: {
      skill: {
        "pocketbrain-*": "allow",
        "*": "ask",
      },
    },
    plugin,
  }

  await writeFile(configPath, `${JSON.stringify(config, null, 2)}\n`, "utf-8")
}

async function seedBundledSkills(opencodeConfigDir: string): Promise<void> {
  const sourceSkillsDir = join(process.cwd(), ".agents", "skills")
  const targetSkillsDir = join(opencodeConfigDir, ".agents", "skills")

  let sourceSkillDirs: string[] = []
  try {
    const entries = await readdir(sourceSkillsDir, { withFileTypes: true })
    sourceSkillDirs = entries.filter((entry) => entry.isDirectory()).map((entry) => entry.name)
  } catch {
    return
  }

  for (const skillDirName of sourceSkillDirs) {
    const sourceFile = join(sourceSkillsDir, skillDirName, "SKILL.md")
    const targetDir = join(targetSkillsDir, skillDirName)
    const targetFile = join(targetDir, "SKILL.md")

    await mkdir(targetDir, { recursive: true })

    const targetExists = await Bun.file(targetFile).exists()
    if (targetExists) continue

    try {
      const content = await readFile(sourceFile, "utf-8")
      await writeFile(targetFile, content, "utf-8")
    } catch {
      continue
    }
  }
}

function formatVaultProfile(summary: ObsidianConfigSummary): string {
  if (!summary.obsidianConfigFound) {
    return "No .obsidian config detected yet. Confirm user conventions before writing many files."
  }

  const lines = [
    `Daily notes folder: ${summary.dailyNotes.folder}`,
    `Daily format: ${summary.dailyNotes.format}`,
    `Daily template: ${summary.dailyNotes.templateFile}`,
    `Default new-note location: ${summary.newNotes.location}`,
    `Default new-note folder: ${summary.newNotes.folder}`,
    `Attachment folder: ${summary.attachments.folder}`,
    `Link style: ${summary.links.style}`,
    `Templates folder: ${summary.templates.folder}`,
  ]

  if (summary.warnings.length > 0) {
    lines.push(`Warnings: ${summary.warnings.join("; ")}`)
  }

  return lines.join("\n")
}

// Run main
void main().catch((error) => {
  pino({ level: "error" }).fatal({ error }, "fatal startup error")
  process.exit(1)
})
