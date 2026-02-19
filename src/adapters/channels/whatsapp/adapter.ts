/**
 * WhatsApp Adapter
 * Implements ChannelAdapter for WhatsApp using Baileys.
 * 
 * Refactored to use:
 * - ConnectionManager: handles connection lifecycle
 * - CommandHandler: handles command parsing
 * - MessageSender: handles chunked message delivery
 */

import type { WASocket } from "@whiskeysockets/baileys"
import type { Logger } from "pino"
import type { 
  ChannelAdapter, 
  MessageHandler 
} from "../../../core/ports/channel-adapter"
import type { WhitelistRepository } from "../../../core/ports/whitelist-repository"
import type { OutboxMessage, OutboxRepository } from "../../../core/ports/outbox-repository"
import type { MessageSender } from "../../../core/services/message-sender"
import { ConnectionManager } from "./connection-manager"
import { CommandHandler, type CommandResult } from "./command-handler"
import { OutboxProcessor } from "./outbox-processor"
import { createTaskQueue } from "../../../lib/task-queue"

export interface WhatsAppAdapterOptions {
  authDir: string
  logger: Logger
  whitelistRepository: WhitelistRepository
  outboxRepository: OutboxRepository
  messageSender: MessageSender
  pairToken: string | undefined
  pairMaxFailures?: number
  pairFailureWindowMs?: number
  pairBlockDurationMs?: number
  outboxIntervalMs?: number
  outboxRetryBaseDelayMs?: number
  connectingTimeoutMs?: number
  reconnectDelayMs?: number
}

// WhatsApp JID validation patterns
const DIRECT_JID_PATTERN = /^\d+@s\.whatsapp\.net$/
const DIRECT_LID_PATTERN = /^\d+@lid$/
const GROUP_JID_PATTERN = /^[\d-]+@g\.us$/
const BROADCAST_JID_PATTERN = /^\d+@broadcast$/

function isValidJid(jid: string): boolean {
  return DIRECT_JID_PATTERN.test(jid) ||
         DIRECT_LID_PATTERN.test(jid) ||
         GROUP_JID_PATTERN.test(jid) || 
         BROADCAST_JID_PATTERN.test(jid)
}

function isDirectJid(jid: string): boolean {
  return DIRECT_JID_PATTERN.test(jid) || DIRECT_LID_PATTERN.test(jid)
}

function extractText(message: unknown): string {
  if (typeof message !== "object" || message === null) return ""
  
  const msg = message as Record<string, unknown>
  return (
    (typeof msg.conversation === "string" ? msg.conversation : undefined) ??
    (typeof (msg.extendedTextMessage as Record<string, unknown>)?.text === "string" 
      ? (msg.extendedTextMessage as Record<string, unknown>).text as string 
      : undefined) ??
    (typeof (msg.imageMessage as Record<string, unknown>)?.caption === "string"
      ? (msg.imageMessage as Record<string, unknown>).caption as string
      : undefined) ??
    ""
  )
}

function normalizeJid(jid: string): string {
  // Validate input format first
  const sanitized = jid.trim()
  if (!sanitized) return ""
  
  // If already has @, validate it's a proper JID format
  if (sanitized.includes("@")) {
    return isValidJid(sanitized) ? sanitized : ""
  }
  
  // Add suffix for bare numbers - must be numeric only
  if (/^\d+$/.test(sanitized)) {
    return `${sanitized}@s.whatsapp.net`
  }
  
  return ""
}

export function expandDirectWhitelistIDs(jid: string): string[] {
  const normalized = normalizeJid(jid)
  if (!normalized || !isDirectJid(normalized)) {
    return []
  }

  const numericUserID = normalized.match(/^(\d+)@(?:s\.whatsapp\.net|lid)$/)?.[1]
  if (!numericUserID) {
    return [normalized]
  }

  return [`${numericUserID}@s.whatsapp.net`, `${numericUserID}@lid`]
}

interface IncomingMessage {
  message?: unknown
  key?: {
    id?: string
    fromMe?: boolean
    remoteJid?: string
    remoteJidAlt?: string
    participant?: string
    participantPn?: string
    senderPn?: string
  }
}

function collectCandidateUserIDs(msg: IncomingMessage): string[] {
  const candidates = [
    msg.key?.remoteJid,
    msg.key?.remoteJidAlt,
    msg.key?.participant,
    msg.key?.participantPn,
    msg.key?.senderPn,
  ]

  const unique = new Set<string>()
  for (const candidate of candidates) {
    if (!candidate) continue
    const normalized = normalizeJid(candidate)
    if (!normalized) continue
    if (!isDirectJid(normalized)) continue
    unique.add(normalized)
  }

  return [...unique]
}

export class WhatsAppAdapter implements ChannelAdapter {
  readonly name = "whatsapp"

  private readonly options: WhatsAppAdapterOptions
  private connectionManager: ConnectionManager
  private commandHandler: CommandHandler
  private messageHandler?: MessageHandler
  private outboxInterval: ReturnType<typeof setInterval> | undefined
  private readonly outboxQueue = createTaskQueue({ concurrency: 1 })
  private readonly queuedOutboxIDs = new Set<number>()
  private readonly outboxProcessor: OutboxProcessor
  private stopping = false

  constructor(options: WhatsAppAdapterOptions) {
    this.options = {
      outboxIntervalMs: 60_000,
      outboxRetryBaseDelayMs: 60_000,
      ...options,
    }
    
    this.connectionManager = new ConnectionManager({
      authDir: this.options.authDir,
      logger: this.options.logger,
      connectingTimeoutMs: this.options.connectingTimeoutMs,
      reconnectDelayMs: this.options.reconnectDelayMs,
      onOpen: () => void this.handleConnectionOpen(),
    })

    this.commandHandler = new CommandHandler({
      pairToken: this.options.pairToken,
      logger: this.options.logger,
      pairMaxFailures: this.options.pairMaxFailures,
      pairFailureWindowMs: this.options.pairFailureWindowMs,
      pairBlockDurationMs: this.options.pairBlockDurationMs,
    })

    this.outboxProcessor = new OutboxProcessor({
      outboxRepository: this.options.outboxRepository,
      logger: this.options.logger,
      retryBaseDelayMs: this.options.outboxRetryBaseDelayMs,
      sendMessage: async (userID, text) => {
        await this.send(userID, text)
      },
    })
  }

  async start(handler: MessageHandler): Promise<void> {
    this.messageHandler = handler
    this.stopping = false
    this.outboxQueue.start()

    const socket = await this.connectionManager.connect()
    this.setupMessageHandler(socket)
    this.startOutboxFlush()
  }

  async stop(): Promise<void> {
    this.stopping = true
    this.stopOutboxFlush()
    this.outboxQueue.pause()
    this.outboxQueue.clear()
    this.queuedOutboxIDs.clear()
    this.connectionManager.stop()
  }

  async send(userID: string, text: string): Promise<void> {
    if (!this.connectionManager.isConnected()) {
      throw new Error("WhatsApp not connected")
    }

    const socket = this.connectionManager.getSocket()
    if (!socket) {
      throw new Error("WhatsApp socket not available")
    }

    const target = normalizeJid(userID)
    if (!target) {
      throw new Error("Invalid WhatsApp ID format")
    }
    if (!isDirectJid(target)) {
      throw new Error("Cannot send to group chats")
    }

    await this.options.messageSender.send(target, text, async (chunk) => {
      await socket.sendMessage(target, { text: chunk })
    })
  }

  private setupMessageHandler(socket: WASocket): void {
    socket.ev.on("messages.upsert", ({ messages, type }: { messages: unknown[]; type: string }) => {
      if (type !== "notify") return
      void this.handleMessages(messages)
    })
  }

  private async handleMessages(messages: unknown[]): Promise<void> {
    for (const rawMsg of messages) {
      try {
        await this.handleMessage(rawMsg)
      } catch (error) {
        this.options.logger.error({ error }, "whatsapp message handling failed")
      }
    }
  }

  private async handleMessage(rawMsg: unknown): Promise<void> {
    const msg = rawMsg as IncomingMessage

    if (!msg.message || msg.key?.fromMe) return

    const jid = msg.key?.remoteJid
    if (!jid) return

    if (!isDirectJid(jid)) {
      this.options.logger.info({ jid }, "ignoring non-1:1 whatsapp chat")
      return
    }

    const text = extractText(msg.message).trim()
    if (!text) return

    this.options.logger.info({ jid, textLength: text.length }, "whatsapp message received")

    const candidateUserIDs = collectCandidateUserIDs(msg)
    const isWhitelisted = candidateUserIDs.some((candidateID) =>
      this.options.whitelistRepository.isWhitelisted("whatsapp", candidateID),
    )
    if (!isWhitelisted) {
      this.options.logger.warn(
        {
          jid,
          messageKey: msg.key,
          suggestedWhitelistIDs: candidateUserIDs,
        },
        "whatsapp sender not whitelisted",
      )
    }
    const commandResult = this.commandHandler.handle({ jid, text, isWhitelisted })

    if (commandResult.handled) {
      // Mark as read for commands too
      const socket = this.connectionManager.getSocket()
      if (socket && msg.key?.id) {
        try {
          await socket.readMessages([{ remoteJid: jid, id: msg.key.id, fromMe: false }])
        } catch { /* best-effort */ }
      }
      await this.handleCommandResult(jid, commandResult)
      return
    }

    // Not a command - process through handler
    if (this.messageHandler) {
      const messageKey = {
        remoteJid: msg.key?.remoteJid ?? jid,
        id: msg.key?.id ?? '',
        fromMe: false,
      }
      await this.processUserMessage(jid, text, messageKey)
    }
  }

  private async handleCommandResult(jid: string, result: CommandResult): Promise<void> {
    switch (result.action) {
      case "pair": {
        const whitelistIDs = expandDirectWhitelistIDs(jid)
        const targetIDs = whitelistIDs.length > 0 ? whitelistIDs : [jid]
        let created = false

        for (const targetID of targetIDs) {
          if (this.options.whitelistRepository.addToWhitelist("whatsapp", targetID)) {
            created = true
          }
        }

        await this.send(
          jid,
          created ? "Pairing successful. You are now whitelisted." : "You are already whitelisted.",
        )
        return
      }

      case "remember": {
        if (result.payload && this.messageHandler) {
          await this.messageHandler(jid, `/remember ${result.payload}`)
        }
        if (result.response) {
          await this.send(jid, result.response)
        }
        return
      }

      case "new_session": {
        if (!this.messageHandler) {
          if (result.response) {
            await this.send(jid, result.response)
          }
          return
        }

        const response = await this.messageHandler(jid, "/new")
        await this.send(jid, response)
        return
      }

      default: {
        if (result.response) {
          await this.send(jid, result.response)
        }
      }
    }
  }

  private async processUserMessage(
    jid: string,
    text: string,
    messageKey: { remoteJid: string; id: string; fromMe?: boolean },
  ): Promise<void> {
    if (!this.messageHandler) return

    const socket = this.connectionManager.getSocket()

    // Best-effort UX: read receipt + typing indicator
    if (socket) {
      try {
        await socket.readMessages([messageKey])
      } catch { /* best-effort */ }
      try {
        await socket.sendPresenceUpdate('composing', jid)
      } catch { /* best-effort */ }
    }

    try {
      const answer = await this.messageHandler(jid, text)
      await this.send(jid, answer)
      this.options.logger.info({ jid, answerLength: answer.length }, "whatsapp reply sent")
    } catch (error) {
      this.options.logger.error({ error, jid }, "whatsapp message processing failed")
      try {
        await this.send(jid, "I hit an internal error while processing that. Please try again.")
      } catch (sendError) {
        this.options.logger.error({ error: sendError, jid }, "failed to send whatsapp fallback error message")
      }
    } finally {
      if (socket) {
        try {
          await socket.sendPresenceUpdate('paused', jid)
        } catch { /* best-effort */ }
      }
    }
  }

  private handleConnectionOpen(): void {
    void this.flushOutbox()
  }

  private startOutboxFlush(): void {
    this.outboxInterval = setInterval(() => {
      void this.flushOutbox()
    }, this.options.outboxIntervalMs)
  }

  private stopOutboxFlush(): void {
    if (this.outboxInterval) {
      clearInterval(this.outboxInterval)
      this.outboxInterval = undefined
    }
  }

  private async flushOutbox(): Promise<void> {
    if (!this.connectionManager.isConnected()) return

    try {
      const pending = this.options.outboxRepository.listPending("whatsapp")
      if (pending.length === 0) return

      for (const item of pending) {
        this.enqueueOutboxItem(item)
      }
    } catch (error) {
      this.options.logger.warn({ error }, "whatsapp outbox flush failed")
    }
  }

  private enqueueOutboxItem(item: OutboxMessage): void {
    if (this.queuedOutboxIDs.has(item.id)) {
      return
    }
    this.queuedOutboxIDs.add(item.id)

    void this.outboxQueue.add(async () => {
      try {
        await this.outboxProcessor.process(item)
      } finally {
        this.queuedOutboxIDs.delete(item.id)
      }
    })
  }
}
