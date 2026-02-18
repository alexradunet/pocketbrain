/**
 * Channel Adapter Port
 * 
 * Defines the contract for channel implementations.
 * Following Interface Segregation Principle.
 */

import type { Logger } from "pino"

/**
 * Message handler function type
 */
export type MessageHandler = (userID: string, text: string) => Promise<string>

/**
 * Core channel adapter interface
 */
export interface ChannelAdapter {
  /** Channel name/identifier */
  readonly name: string

  /** Start the channel adapter */
  start(handler: MessageHandler): Promise<void>

  /** Stop the channel adapter */
  stop(): Promise<void>

  /** Send a message to a user */
  send(userID: string, text: string): Promise<void>
}

/**
 * Options for creating channel adapters
 */
export interface ChannelAuthOptions {
  authDir: string
  logger: Logger
}

/**
 * Dependencies for WhatsApp adapter
 */
export interface WhatsAppDependencies {
  logger: Logger
  isWhitelisted: (jid: string) => boolean
  addToWhitelist: (jid: string) => boolean
  pairToken?: string
}
