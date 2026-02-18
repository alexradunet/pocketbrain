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
   * Add a new heartbeat task
   */
  addTask(task: string): void

  /**
   * Remove a heartbeat task by ID
   */
  removeTask(id: number): void

  /**
   * Get the number of tasks
   */
  getTaskCount(): number
}
