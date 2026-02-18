/**
 * Heartbeat Repository Port
 * Defines the interface for heartbeat task persistence operations.
 */

export interface HeartbeatRepository {
  /**
   * Get all enabled heartbeat tasks
   */
  getTasks(): string[]

  /**
   * Get the number of tasks
   */
  getTaskCount(): number
}
