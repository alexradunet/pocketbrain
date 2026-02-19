/**
 * PocketBrain Entry Point
 * 
 * Composition root that wires together all dependencies.
 * Follows Dependency Injection pattern.
 */

import pino from "pino"
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

// Scheduler imports
import { HeartbeatScheduler } from "./scheduler/heartbeat"

// Vault imports
import { createVaultService, VaultService } from "./vault/vault-service"
import { vaultProvider } from "./vault/vault-provider"

// Set OpenCode config directory
process.env.OPENCODE_CONFIG_DIR ??= process.cwd()

async function main(): Promise<void> {
  // Load configuration
  const cfg = loadConfig()
  const logger = pino({ level: cfg.logLevel })

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

  // Create prompt builder
  const promptBuilder = new PromptBuilder({
    heartbeatIntervalMinutes: cfg.heartbeatIntervalMinutes,
    vaultEnabled: cfg.vaultEnabled,
    vaultPath: cfg.vaultPath,
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
  
  // Initialize vault if enabled
  let vaultService: VaultService | undefined
  
  if (cfg.vaultEnabled) {
    logger.info({ vaultPath: cfg.vaultPath }, "initializing vault")
    vaultService = createVaultService(cfg.vaultPath, logger)
    await vaultService.initialize()
    vaultProvider.setVaultService(vaultService)
    logger.info("vault initialized (sync via Syncthing)")
  } else {
    logger.info("vault disabled")
  }

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

// Run main
void main().catch((error) => {
  pino({ level: "error" }).fatal({ error }, "fatal startup error")
  process.exit(1)
})
