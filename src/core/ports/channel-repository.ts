/**
 * Channel Repository Port
 * Defines the interface for channel-related persistence operations.
 */

export type ChannelType = string

export interface LastChannel {
  channel: ChannelType
  userID: string
}

export interface ChannelRepository {
  /**
   * Save the last used channel
   */
  saveLastChannel(channel: ChannelType, userID: string): void

  /**
   * Get the last used channel
   */
  getLastChannel(): LastChannel | null
}
