/**
 * Message Sender Service
 * Handles chunked message delivery with rate limiting.
 * Eliminates duplicate code from channel adapters.
 */

import type { Logger } from "pino"
import { MessageChunker } from "./message-chunker"
import type { ThrottlePort } from "../ports/throttle"

export interface MessageSenderOptions {
  chunker: MessageChunker
  rateLimiter: ThrottlePort
  chunkDelayMs: number
  logger: Logger
}

export interface SendFunction {
  (text: string): Promise<void>
}

export class MessageSender {
  private readonly chunker: MessageChunker
  private readonly rateLimiter: ThrottlePort
  private readonly chunkDelayMs: number
  private readonly logger: Logger

  constructor(options: MessageSenderOptions) {
    this.chunker = options.chunker
    this.rateLimiter = options.rateLimiter
    this.chunkDelayMs = options.chunkDelayMs
    this.logger = options.logger
  }

  /**
   * Send a message, handling chunking and rate limiting
   */
  async send(userID: string, text: string, sendFn: SendFunction): Promise<void> {
    await this.rateLimiter.throttle(userID)

    const chunks = this.chunker.split(text)
    if (chunks.length === 0) {
      this.logger.warn({ userID }, "attempted to send empty message")
      return
    }

    for (let i = 0; i < chunks.length; i++) {
      await sendFn(chunks[i])
      
      // Delay between chunks (but not after the last one)
      if (i < chunks.length - 1) {
        await this.delay(this.chunkDelayMs)
      }
    }

    this.logger.info(
      { userID, chunkCount: chunks.length, totalLength: text.length },
      "message sent"
    )
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms))
  }
}
