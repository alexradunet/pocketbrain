import type { Logger } from "pino"
import type { OutboxMessage, OutboxRepository } from "../../../core/ports/outbox-repository"

export interface OutboxProcessorOptions {
  outboxRepository: OutboxRepository
  logger: Logger
  sendMessage: (userID: string, text: string) => Promise<void>
  retryBaseDelayMs?: number
}

export class OutboxProcessor {
  private readonly options: OutboxProcessorOptions

  constructor(options: OutboxProcessorOptions) {
    this.options = options
  }

  async process(item: OutboxMessage): Promise<void> {
    try {
      await this.options.sendMessage(item.userID, item.text)

      this.options.outboxRepository.acknowledge(item.id)
      this.options.logger.info({ userID: item.userID }, "whatsapp proactive message sent")
    } catch (error) {
      if (this.isInvalidTargetError(error)) {
        this.options.logger.warn(
          { userID: item.userID, error: error instanceof Error ? error.message : String(error) },
          "dropping proactive message with invalid recipient",
        )
        this.options.outboxRepository.acknowledge(item.id)
        return
      }

      const newRetryCount = item.retryCount + 1
      if (newRetryCount >= item.maxRetries) {
        this.options.logger.error(
          { userID: item.userID, retries: newRetryCount, error },
          "whatsapp outbox max retries exceeded",
        )
        this.options.outboxRepository.acknowledge(item.id)
        return
      }

      const baseDelay = this.options.retryBaseDelayMs ?? 60_000
      const delayMs = baseDelay * Math.pow(2, item.retryCount)
      const nextRetry = new Date(Date.now() + delayMs).toISOString()
      this.options.outboxRepository.markRetry(item.id, newRetryCount, nextRetry)

      this.options.logger.warn(
        { userID: item.userID, retry: newRetryCount, nextRetry, error },
        "whatsapp outbox send failed, scheduling retry",
      )
    }
  }

  private isInvalidTargetError(error: unknown): boolean {
    if (!(error instanceof Error)) {
      return false
    }

    return (
      error.message === "Invalid WhatsApp ID format" ||
      error.message === "Cannot send to group chats"
    )
  }
}
