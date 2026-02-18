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
import type { OutboxRepository } from "../../../core/ports/outbox-repository"
import type { MessageSender } from "../../../core/services/message-sender"
import { ConnectionManager } from "./connection-manager"
import { CommandHandler, type CommandResult } from "./command-handler"
import { createTaskQueue } from "../../../lib/task-queue"
import { retryWithBackoff } from "../../../lib/retry"

export interface WhatsAppAdapterOptions {
  authDir: string
  logger: Logger
  whitelistRepository: WhitelistRepository
  outboxRepository: OutboxRepository
  messageSender: MessageSender
  pairToken: string | undefined
  outboxIntervalMs?: number
  outboxRetryBaseDelayMs?: number
  connectingTimeoutMs?: number
  reconnectDelayMs?: number
}

// WhatsApp JID validation patterns
const DIRECT_JID_PATTERN = /^\d+@s\.whatsapp\.net$/
const GROUP_JID_PATTERN = /^[\d-]+@g\.us$/
const BROADCAST_JID_PATTERN = /^\d+@broadcast$/

function isValidJid(jid: string): boolean {
  return DIRECT_JID_PATTERN.test(jid) || 
         GROUP_JID_PATTERN.test(jid) || 
         BROADCAST_JID_PATTERN.test(jid)
}

function isDirectJid(jid: string): boolean {
  return DIRECT_JID_PATTERN.test(jid)
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

interface IncomingMessage {
  message?: unknown
  key?: {
    fromMe?: boolean
    remoteJid?: string
  }
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

    const isWhitelisted = this.options.whitelistRepository.isWhitelisted("whatsapp", jid)
    const commandResult = this.commandHandler.handle({ jid, text, isWhitelisted })

    if (commandResult.handled) {
      await this.handleCommandResult(jid, commandResult)
      return
    }

    // Not a command - process through handler
    if (this.messageHandler) {
      await this.processUserMessage(jid, text)
    }
  }

  private async handleCommandResult(jid: string, result: CommandResult): Promise<void> {
    const socket = this.connectionManager.getSocket()
    if (!socket) return

    switch (result.action) {
      case "pair": {
        const created = this.options.whitelistRepository.addToWhitelist("whatsapp", jid)
        await socket.sendMessage(jid, {
          text: created ? "Pairing successful. You are now whitelisted." : "You are already whitelisted.",
        })
        return
      }

      case "remember": {
        if (result.payload && this.messageHandler) {
          await this.messageHandler(jid, `/remember ${result.payload}`)
        }
        if (result.response) {
          await socket.sendMessage(jid, { text: result.response })
        }
        return
      }

      case "new_session": {
        if (!this.messageHandler) {
          if (result.response) {
            await socket.sendMessage(jid, { text: result.response })
          }
          return
        }

        const response = await this.messageHandler(jid, "/new")
        await socket.sendMessage(jid, { text: response })
        return
      }

      default: {
        if (result.response) {
          await socket.sendMessage(jid, { text: result.response })
        }
      }
    }
  }

  private async processUserMessage(jid: string, text: string): Promise<void> {
    if (!this.messageHandler) return

    const socket = this.connectionManager.getSocket()
    if (!socket) return

    try {
      const answer = await this.messageHandler(jid, text)
      await this.send(jid, answer)
      this.options.logger.info({ jid, answerLength: answer.length }, "whatsapp reply sent")
    } catch (error) {
      this.options.logger.error({ error, jid }, "whatsapp message processing failed")
      await socket.sendMessage(jid, { 
        text: "I hit an internal error while processing that. Please try again." 
      })
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

  private enqueueOutboxItem(item: { id: number; userID: string; text: string; retryCount: number; maxRetries: number }): void {
    if (this.queuedOutboxIDs.has(item.id)) {
      return
    }
    this.queuedOutboxIDs.add(item.id)

    void this.outboxQueue.add(async () => {
      try {
        await this.processOutboxItem(item)
      } finally {
        this.queuedOutboxIDs.delete(item.id)
      }
    })
  }

  private async processOutboxItem(
    item: { id: number; userID: string; text: string; retryCount: number; maxRetries: number }
  ): Promise<void> {
    const target = normalizeJid(item.userID)
    
    if (!target) {
      this.options.logger.warn({ jid: item.userID }, "dropping proactive message with invalid JID")
      this.options.outboxRepository.acknowledge(item.id)
      return
    }
    
    if (!isDirectJid(target)) {
      this.options.logger.warn({ jid: target }, "dropping non-1:1 proactive message target")
      this.options.outboxRepository.acknowledge(item.id)
      return
    }

    try {
      await retryWithBackoff(
        async () => {
          const socket = this.connectionManager.getSocket()
          if (!socket || !this.connectionManager.isConnected()) {
            throw new Error("whatsapp socket unavailable")
          }

          await this.options.messageSender.send(target, item.text, async (chunk) => {
            await socket.sendMessage(target, { text: chunk })
          })
        },
        {
          retries: 2,
          minTimeoutMs: this.options.outboxRetryBaseDelayMs ?? 60_000,
          maxTimeoutMs: 10 * 60_000,
          factor: 2,
        },
      )
      
      this.options.outboxRepository.acknowledge(item.id)
      this.options.logger.info({ jid: target }, "whatsapp proactive message sent")
    } catch (error) {
      const newRetryCount = item.retryCount + 1
      
      if (newRetryCount >= item.maxRetries) {
        this.options.logger.error({ 
          jid: item.userID, 
          retries: newRetryCount 
        }, "whatsapp outbox max retries exceeded")
        this.options.outboxRepository.acknowledge(item.id)
      } else {
        const delayMs = (this.options.outboxRetryBaseDelayMs ?? 60_000) * Math.pow(2, item.retryCount)
        const nextRetry = new Date(Date.now() + delayMs).toISOString()
        
        this.options.outboxRepository.markRetry(item.id, newRetryCount, nextRetry)
        this.options.logger.warn({ 
          jid: item.userID, 
          retry: newRetryCount, 
          nextRetry 
        }, "whatsapp outbox send failed, scheduling retry")
      }
    }
  }
}
