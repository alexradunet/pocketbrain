/**
 * Outbox Repository Port
 * Defines the interface for outbox (proactive messaging) persistence operations.
 */

export interface OutboxMessage {
  id: number
  channel: string
  userID: string
  text: string
  retryCount: number
  maxRetries: number
  nextRetryAt: string | null
}

export interface OutboxRepository {
  /**
   * Queue a message for sending
   */
  enqueue(channel: string, userID: string, text: string, maxRetries?: number): void

  /**
   * List pending messages ready to be sent
   */
  listPending(channel: string): OutboxMessage[]

  /**
   * Acknowledge (delete) a sent message
   */
  acknowledge(id: number): void

  /**
   * Mark a message for retry
   */
  markRetry(id: number, retryCount: number, nextRetryAt: string): void
}
