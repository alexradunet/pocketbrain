/**
 * Whitelist Repository Port
 * Defines the interface for whitelist persistence operations.
 */

export interface WhitelistRepository {
  /**
   * Check if a user is whitelisted for a channel
   */
  isWhitelisted(channel: string, userID: string): boolean

  /**
   * Add a user to the whitelist
   * @returns true if added, false if already exists
   */
  addToWhitelist(channel: string, userID: string): boolean

  /**
   * Remove a user from the whitelist
   */
  removeFromWhitelist(channel: string, userID: string): boolean
}
