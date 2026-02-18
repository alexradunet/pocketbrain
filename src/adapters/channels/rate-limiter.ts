/**
 * Rate Limiter
 * Throttles message sending per user.
 */

export interface RateLimiterOptions {
  minIntervalMs: number
}

export class RateLimiter {
  private lastSendTime: Map<string, number> = new Map()
  private readonly options: RateLimiterOptions

  constructor(options: RateLimiterOptions) {
    this.options = options
  }

  /**
   * Throttle a user
   */
  async throttle(userID: string): Promise<void> {
    const now = Date.now()
    const lastTime = this.lastSendTime.get(userID) ?? 0
    const waitTime = Math.max(0, this.options.minIntervalMs - (now - lastTime))

    if (waitTime > 0) {
      await new Promise((resolve) => setTimeout(resolve, waitTime))
    }

    this.lastSendTime.set(userID, Date.now())
    this.cleanupOldEntries()
  }

  /**
   * Get last send time for a user
   */
  getLastSendTime(userID: string): number | undefined {
    return this.lastSendTime.get(userID)
  }

  /**
   * Reset a user's rate limit
   */
  reset(userID: string): void {
    this.lastSendTime.delete(userID)
  }

  /**
   * Reset all rate limits
   */
  resetAll(): void {
    this.lastSendTime.clear()
  }

  /**
   * Clean up entries older than 1 hour to prevent memory leaks
   */
  private cleanupOldEntries(): void {
    const oneHourAgo = Date.now() - 60 * 60 * 1000
    for (const [userID, time] of this.lastSendTime.entries()) {
      if (time < oneHourAgo) {
        this.lastSendTime.delete(userID)
      }
    }
  }
}
