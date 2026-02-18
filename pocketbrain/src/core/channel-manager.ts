/**
 * Channel Manager
 * 
 * Manages multiple channel adapters.
 * Simplified to focus on lifecycle management.
 */

import type { Logger } from "pino"
import type { ChannelAdapter, MessageHandler } from "./ports/channel-adapter"

export class ChannelManager {
  private adapters: Map<string, ChannelAdapter> = new Map()
  private messageHandler?: MessageHandler
  private readonly logger: Logger

  constructor(logger: Logger) {
    this.logger = logger
  }

  /**
   * Register a channel adapter
   */
  register(adapter: ChannelAdapter): void {
    this.adapters.set(adapter.name, adapter)
    this.logger.info({ channel: adapter.name }, "channel adapter registered")
  }

  /**
   * Start all registered adapters
   */
  async start(handler: MessageHandler): Promise<void> {
    this.messageHandler = handler
    const promises: Promise<void>[] = []
    
    for (const adapter of this.adapters.values()) {
      promises.push(adapter.start(handler))
    }
    
    await Promise.all(promises)
    this.logger.info({ channels: this.channels }, "all channels started")
  }

  /**
   * Stop all registered adapters
   */
  async stop(): Promise<void> {
    const promises: Promise<void>[] = []
    
    for (const adapter of this.adapters.values()) {
      promises.push(adapter.stop())
    }
    
    await Promise.all(promises)
    this.logger.info("all channels stopped")
  }

  /**
   * Send a message through a specific channel
   */
  async send(channel: string, userID: string, text: string): Promise<void> {
    const adapter = this.adapters.get(channel)
    if (!adapter) {
      throw new Error(`Unknown channel: ${channel}`)
    }
    await adapter.send(userID, text)
  }

  /**
   * Get a specific adapter
   */
  get(channel: string): ChannelAdapter | undefined {
    return this.adapters.get(channel)
  }

  /**
   * Get list of registered channel names
   */
  get channels(): string[] {
    return Array.from(this.adapters.keys())
  }
}
