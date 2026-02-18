/**
 * Heartbeat Scheduler
 * 
 * Manages periodic task execution with exponential backoff.
 */

import type { Logger } from "pino"
import { randomUUID } from "node:crypto"
import type { AssistantCore } from "../core/assistant"
import type { OutboxRepository } from "../core/ports/outbox-repository"
import type { ChannelRepository } from "../core/ports/channel-repository"

export interface HeartbeatOptions {
  intervalMinutes: number
  baseDelayMs: number
  maxDelayMs: number
  notifyAfterFailures: number
}

export interface HeartbeatDependencies {
  assistant: AssistantCore
  outboxRepository: OutboxRepository
  channelRepository: ChannelRepository
  logger: Logger
}

export class HeartbeatScheduler {
  private readonly options: HeartbeatOptions
  private readonly deps: HeartbeatDependencies
  private nextRunTimeout: ReturnType<typeof setTimeout> | undefined
  private running = false
  private consecutiveFailures = 0
  private currentBackoffMs: number

  constructor(options: HeartbeatOptions, deps: HeartbeatDependencies) {
    this.options = options
    this.deps = deps
    this.currentBackoffMs = options.intervalMinutes * 60_000
  }

  /**
   * Start the heartbeat scheduler
   */
  start(): void {
    const ms = Math.max(1, this.options.intervalMinutes) * 60_000
    
    if (!Number.isFinite(ms) || ms <= 0) {
      this.deps.logger.warn({ intervalMinutes: this.options.intervalMinutes }, "invalid heartbeat interval")
      return
    }

    this.scheduleNextRun(0)
    
    this.deps.logger.info({ intervalMinutes: this.options.intervalMinutes }, "heartbeat scheduler started")
  }

  /**
   * Stop the heartbeat scheduler
   */
  stop(): void {
    this.clearNextRunTimeout()
  }

  private async run(): Promise<void> {
    const runID = `scheduler-${randomUUID()}`
    if (this.running) {
      this.deps.logger.warn({ runID }, "heartbeat run skipped: previous run still active")
      this.scheduleNextRun(Math.max(1, this.options.intervalMinutes) * 60_000)
      return
    }

    this.running = true
    const startedAt = Date.now()

    try {
      this.deps.logger.debug({ runID }, "heartbeat run started")
      const result = await this.deps.assistant.runHeartbeatTasks()
      
      // Reset on success
      this.consecutiveFailures = 0
      this.currentBackoffMs = this.options.intervalMinutes * 60_000
      this.scheduleNextRun(this.currentBackoffMs)
      
      this.deps.logger.info({ runID, result, durationMs: Date.now() - startedAt }, "heartbeat run completed")
    } catch (error) {
      await this.handleFailure(error, startedAt, runID)
      this.scheduleNextRun(this.currentBackoffMs)
    } finally {
      this.running = false
    }
  }

  private async handleFailure(error: unknown, startedAt: number, runID: string): Promise<void> {
    this.consecutiveFailures += 1
    
    const delay = Math.min(
      this.options.baseDelayMs * Math.pow(2, this.consecutiveFailures),
      this.options.maxDelayMs
    )
    this.currentBackoffMs = Math.max(this.options.intervalMinutes * 60_000, delay)

    this.deps.logger.error({
      runID,
      error,
      durationMs: Date.now() - startedAt,
      consecutiveFailures: this.consecutiveFailures,
      nextBackoffMs: this.currentBackoffMs,
    }, "heartbeat run failed")

    if (this.consecutiveFailures >= this.options.notifyAfterFailures) {
      await this.notifyFailure()
    }
  }

  private async notifyFailure(): Promise<void> {
    const lastChannel = this.deps.channelRepository.getLastChannel()
    if (!lastChannel) {
      this.deps.logger.warn("heartbeat consecutive failures but no last channel to notify")
      return
    }

    this.deps.outboxRepository.enqueue(
      lastChannel.channel,
      lastChannel.userID,
      `Heartbeat has failed ${this.consecutiveFailures} times in a row. Check logs for details.`
    )
    
    this.deps.logger.warn({ failureCount: this.consecutiveFailures }, "heartbeat notification sent to user")
  }

  private scheduleNextRun(delayMs: number): void {
    this.clearNextRunTimeout()
    this.nextRunTimeout = setTimeout(() => {
      void this.run()
    }, delayMs)
  }

  private clearNextRunTimeout(): void {
    if (this.nextRunTimeout) {
      clearTimeout(this.nextRunTimeout)
      this.nextRunTimeout = undefined
    }
  }
}
