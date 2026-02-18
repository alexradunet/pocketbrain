/**
 * Session Repository Port
 * Defines the interface for session persistence operations.
 */

export interface SessionRepository {
  /**
   * Get a session ID by key
   */
  getSessionId(key: string): string | undefined

  /**
   * Save a session ID for a key
   */
  saveSessionId(key: string, sessionId: string): void

  /**
   * Delete a session
   */
  deleteSession(key: string): void
}
