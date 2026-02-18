/**
 * Assistant Core
 * Orchestrates the assistant functionality using injected dependencies.
 * 
 * This class now follows SRP by delegating to specialized services:
 * - RuntimeProvider: Manages OpenCode connection
 * - SessionManager: Manages session lifecycle
 * - PromptBuilder: Builds prompts
 * - MemoryRepository: Handles memory persistence
 */

import type { Part, TextPart, Message } from "@opencode-ai/sdk"
import type { Logger } from "pino"
import { randomUUID } from "node:crypto"
import type { RuntimeProvider } from "./runtime-provider"
import type { SessionManager } from "./session-manager"
import type { PromptBuilder } from "./prompt-builder"
import type { MemoryRepository } from "./ports/memory-repository"
import type { ChannelRepository } from "./ports/channel-repository"
import type { HeartbeatRepository } from "./ports/heartbeat-repository"
import { 
  isPromptResult, 
  isMessagesResult, 
  type SessionPromptRequest,
  type SessionMessagesRequest,
} from "../lib/types"

export type AssistantInput = {
  channel: string
  userID: string
  text: string
}

export interface AssistantCoreOptions {
  runtimeProvider: RuntimeProvider
  sessionManager: SessionManager
  promptBuilder: PromptBuilder
  memoryRepository: MemoryRepository
  channelRepository: ChannelRepository
  heartbeatRepository: HeartbeatRepository
  logger: Logger
}

export interface AssistantDeps {
  runtimeProvider: RuntimeProvider
  sessionManager: SessionManager
  promptBuilder: PromptBuilder
  memoryRepository: MemoryRepository
  channelRepository: ChannelRepository
  heartbeatRepository: HeartbeatRepository
  logger: Logger
}

export class AssistantCore {
  private readonly deps: AssistantDeps

  constructor(deps: AssistantCoreOptions) {
    this.deps = deps
  }

  /**
   * Initialize the assistant
   */
  async init(): Promise<void> {
    await this.deps.runtimeProvider.init()
  }

  /**
   * Close the assistant
   */
  async close(): Promise<void> {
    await this.deps.runtimeProvider.close()
  }

  /**
   * Process a user message and return a response
   */
  async ask(input: AssistantInput): Promise<string> {
    const operationID = this.createOperationID("ask")
    const startedAt = Date.now()
    const client = this.ensureClient()
    const sessionID = await this.deps.sessionManager.getOrCreateMainSession(client)

    if (input.channel === "whatsapp") {
      this.deps.channelRepository.saveLastChannel(input.channel, input.userID)
    }

    const memoryEntries = this.deps.memoryRepository.getAll()
    const systemPrompt = this.deps.promptBuilder.buildAgentSystemPrompt(memoryEntries)

    this.deps.logger.info(
      { 
        operationID,
        channel: input.channel, 
        userID: input.userID, 
        sessionID, 
        textLength: input.text.length, 
        memoryContextLength: memoryEntries.length
      },
      "assistant request started"
    )

    const promptRequest: SessionPromptRequest = {
      path: { id: sessionID },
      body: {
        noReply: false,
        system: systemPrompt,
        parts: [{ type: "text", text: input.text }],
        ...this.getModelConfig(),
      },
    }
    const result = await client.session.prompt(promptRequest)
    if (!isPromptResult(result)) {
      this.deps.logger.error({ operationID, sessionID }, "assistant returned invalid response format")
      return "I did not receive a valid model reply. Please check OpenCode provider auth/model setup."
    }

    const text = this.extractText(result.data?.parts ?? [])
    if (!text) {
      this.deps.logger.error({ operationID, sessionID }, "assistant returned empty response")
      return "I did not receive a model reply. Please check OpenCode provider auth/model setup."
    }

    this.deps.logger.info(
      { 
        operationID,
        channel: input.channel, 
        userID: input.userID, 
        sessionID, 
        durationMs: Date.now() - startedAt, 
        answerLength: text.length 
      },
      "assistant request completed"
    )
    
    return text
  }

  /**
   * Start a new main session
   */
  async startNewMainSession(reason = "manual"): Promise<string> {
    const client = this.ensureClient()
    return this.deps.sessionManager.startNewMainSession(client, reason)
  }

  /**
   * Save a memory fact
   */
  async remember(note: string, source: string): Promise<boolean> {
    return this.deps.memoryRepository.append(note.trim(), source)
  }

  /**
   * Delete a memory fact
   */
  async deleteMemory(id: number): Promise<boolean> {
    return this.deps.memoryRepository.delete(id)
  }

  /**
   * Update a memory fact
   */
  async updateMemory(id: number, fact: string): Promise<boolean> {
    return this.deps.memoryRepository.update(id, fact.trim())
  }

  /**
   * Get heartbeat task status
   */
  async heartbeatTaskStatus(): Promise<{ taskCount: number; empty: boolean }> {
    const taskCount = this.deps.heartbeatRepository.getTaskCount()
    return { taskCount, empty: taskCount === 0 }
  }

  /**
   * Run heartbeat tasks
   */
  async runHeartbeatTasks(): Promise<string> {
    const operationID = this.createOperationID("heartbeat")
    const startedAt = Date.now()
    const tasks = this.deps.heartbeatRepository.getTasks()
    
    if (tasks.length === 0) {
      return "Heartbeat skipped: no tasks found in heartbeat_tasks table."
    }

    const client = this.ensureClient()
    const heartbeatSessionID = await this.deps.sessionManager.getOrCreateHeartbeatSession(client)
    const mainSessionID = await this.deps.sessionManager.getOrCreateMainSession(client)
    
    this.deps.logger.info(
      { operationID, heartbeatSessionID, mainSessionID, taskCount: tasks.length }, 
      "heartbeat sessions ready"
    )

    const memoryEntries = this.deps.memoryRepository.getAll()
    const systemPrompt = this.deps.promptBuilder.buildAgentSystemPrompt(memoryEntries)
    const recentContext = await this.loadRecentContext(client, mainSessionID)
    const prompt = this.deps.promptBuilder.buildHeartbeatPrompt(tasks, recentContext)

    const heartbeatRequest: SessionPromptRequest = {
      path: { id: heartbeatSessionID },
      body: {
        noReply: false,
        system: systemPrompt,
        parts: [{ type: "text", text: prompt }],
        ...this.getModelConfig(),
      },
    }
    const response = await client.session.prompt(heartbeatRequest)
    if (!isPromptResult(response)) {
      this.deps.logger.error({ operationID, heartbeatSessionID }, "heartbeat invalid response format")
      return "Heartbeat failed: invalid response format from model."
    }

    const summary = this.extractText(response.data?.parts ?? [])
    if (!summary) {
      this.deps.logger.error({ operationID, heartbeatSessionID }, "heartbeat empty summary")
      return "Heartbeat failed: no summary reply from model."
    }

    // Inject summary into main session
    const summaryRequest: SessionPromptRequest = {
      path: { id: mainSessionID },
      body: {
        noReply: true,
        parts: [{ type: "text", text: `[Heartbeat summary]\n${summary}` }],
        ...this.getModelConfig(),
      },
    }
    await client.session.prompt(summaryRequest)

    // Trigger proactive notification decision
    const notificationRequest: SessionPromptRequest = {
      path: { id: mainSessionID },
      body: {
        noReply: false,
        parts: [{ type: "text", text: this.deps.promptBuilder.buildProactiveNotificationPrompt() }],
        ...this.getModelConfig(),
      },
    }
    await client.session.prompt(notificationRequest)

    this.deps.logger.info(
      { 
        operationID,
        heartbeatSessionID, 
        mainSessionID, 
        taskCount: tasks.length, 
        durationMs: Date.now() - startedAt 
      }, 
      "heartbeat task run complete"
    )
    
    return `Heartbeat completed with ${tasks.length} tasks.`
  }

  private createOperationID(prefix: string): string {
    return `${prefix}-${randomUUID()}`
  }

  /**
   * Cleanup old sessions (placeholder)
   */
  async cleanupSessions(_maxAgeDays = 30): Promise<{ deleted: string[]; errors: string[] }> {
    const deleted: string[] = []
    const errors: string[] = []
    this.deps.logger.warn("session cleanup requires SDK support for session deletion")
    return { deleted, errors }
  }

  private ensureClient(): NonNullable<ReturnType<RuntimeProvider["getClient"]>> {
    const client = this.deps.runtimeProvider.getClient()
    if (!client) {
      throw new Error("AssistantCore is not initialized. Call init() before use.")
    }
    return client
  }

  private getModelConfig(): { model: { providerID: string; modelID: string } } | Record<string, never> {
    const config = this.deps.runtimeProvider.buildModelConfig()
    return config ? { model: config } : {}
  }

  private extractText(parts: Part[]): string {
    return parts
      .filter((p): p is TextPart => p.type === "text")
      .map((p) => p.text)
      .join("\n")
      .trim()
  }

  private async loadRecentContext(
    client: NonNullable<ReturnType<RuntimeProvider["getClient"]>>, 
    sessionID: string
  ): Promise<string> {
    try {
      const messagesRequest: SessionMessagesRequest = { path: { id: sessionID } }
      const msgs = await client.session.messages(messagesRequest)
      if (!isMessagesResult(msgs)) {
        return ""
      }
      return this.buildRecentContext(msgs.data ?? [])
    } catch (error) {
      this.deps.logger.warn({ error, sessionID }, "failed to load recent context")
      return ""
    }
  }

  private buildRecentContext(
    messages: Array<{ info: Message; parts: Part[] }>, 
    limit = 6, 
    maxChars = 2000
  ): string {
    const out: string[] = []
    let remaining = maxChars
    
    for (let i = messages.length - 1; i >= 0 && out.length < limit; i -= 1) {
      const message = messages[i]
      if (!message) continue
      
      const { info, parts } = message
      const text = this.extractText(parts)
      if (!text) continue
      
      const snippet = `${info.role.toUpperCase()}: ${text}`.trim()
      if (snippet.length > remaining) continue
      
      out.push(snippet)
      remaining -= snippet.length + 1
    }
    
    return out.reverse().join("\n")
  }
}
