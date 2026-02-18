/**
 * Session Manager
 * Manages OpenCode session lifecycle.
 * Follows Single Responsibility Principle.
 */

import type { Logger } from "pino"
import type { SessionRepository } from "./ports/session-repository"
import type { createOpencodeClient } from "@opencode-ai/sdk"
import { isCreateResult, type SessionCreateRequest } from "../lib/types"

export interface SessionManagerOptions {
  repository: SessionRepository
  logger: Logger
}

type Client = ReturnType<typeof createOpencodeClient>

export class SessionManager {
  private readonly repository: SessionRepository
  private readonly logger: Logger

  constructor(options: SessionManagerOptions) {
    this.repository = options.repository
    this.logger = options.logger
  }

  /**
   * Get or create a session by key
   */
  async getOrCreateSession(client: Client, key: string): Promise<string> {
    const sessionKey = `session:${key}`
    const existing = this.repository.getSessionId(sessionKey)
    if (existing) {
      return existing
    }

    const created = await this.createSession(client, key)
    this.repository.saveSessionId(sessionKey, created)
    return created
  }

  /**
   * Get or create the main session
   */
  async getOrCreateMainSession(client: Client): Promise<string> {
    return this.getOrCreateSession(client, "main")
  }

  /**
   * Get or create the heartbeat session
   */
  async getOrCreateHeartbeatSession(client: Client): Promise<string> {
    return this.getOrCreateSession(client, "heartbeat")
  }

  /**
   * Start a new main session
   */
  async startNewMainSession(client: Client, reason = "manual"): Promise<string> {
    const sessionID = await this.createSession(client, `main:${reason}`)
    this.repository.saveSessionId("session:main", sessionID)
    this.logger.info({ sessionID, reason }, "created new main session")
    return sessionID
  }

  /**
   * Create a new session
   */
  private async createSession(client: Client, key: string): Promise<string> {
    const createRequest: SessionCreateRequest = {
      body: { title: `chat:${key}` },
    }
    const result = await client.session.create(createRequest)
    if (!isCreateResult(result)) {
      throw new Error("Failed to create session: invalid response format")
    }

    const id = result.data?.id
    if (typeof id !== "string" || id.length === 0) {
      throw new Error("Failed to create session: missing id")
    }

    this.logger.info({ key, sessionID: id }, "created OpenCode session")
    return id
  }
}
