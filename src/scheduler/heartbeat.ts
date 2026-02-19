/**
 * Heartbeat Scheduler
 * 
 * Manages periodic task execution with per-run retry backoff.
 */

import type { Logger } from "pino"
import { randomUUID } from "node:crypto"
import type { OutboxRepository } from "../core/ports/outbox-repository"
import type { ChannelRepository, LastChannel } from "../core/ports/channel-repository"
import type { HeartbeatRunner } from "../core/ports/heartbeat-runner"
import { retryWithBackoff } from "../lib/retry"

export interface HeartbeatOptions {
  intervalMinutes: number
  baseDelayMs: number
  maxDelayMs: number
  notifyAfterFailures: number
}

export interface HeartbeatDependencies {
  heartbeatRunner: HeartbeatRunner
  outboxRepository: OutboxRepository
  channelRepository: ChannelRepository
  logger: Logger
}

interface NotificationTarget {
  channel: string
  userID: string
}

export class HeartbeatScheduler {
  private readonly options: HeartbeatOptions
  private readonly deps: HeartbeatDependencies
  private nextRunTimeout: ReturnType<typeof setTimeout> | undefined
  private running = false
  private stopped = true
  private consecutiveFailures = 0
  private notifiedForCurrentFailureIncident = false

  constructor(options: HeartbeatOptions, deps: HeartbeatDependencies) {
    this.options = options
    this.deps = deps
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

    this.stopped = false
    this.scheduleNextRun(0)
    
    this.deps.logger.info({ intervalMinutes: this.options.intervalMinutes }, "heartbeat scheduler started")
  }

  /**
   * Stop the heartbeat scheduler
   */
  stop(): void {
    this.stopped = true
    this.clearNextRunTimeout()
  }

  private async run(): Promise<void> {
    if (this.stopped) {
      return
    }

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
      const result = await retryWithBackoff(
        async () => this.deps.heartbeatRunner.runHeartbeatTasks(),
        {
          retries: 2,
          minTimeoutMs: this.options.baseDelayMs,
          maxTimeoutMs: this.options.maxDelayMs,
          factor: 2,
        },
        {
          onFailedAttempt: (error) => {
            this.deps.logger.warn(
              {
                runID,
                attemptNumber: error.attemptNumber,
                retriesLeft: error.retriesLeft,
                message: error.message,
              },
              "heartbeat attempt failed, retrying",
            )
          },
        },
      )
      
      this.consecutiveFailures = 0
      this.notifiedForCurrentFailureIncident = false
      this.scheduleNextRun(Math.max(1, this.options.intervalMinutes) * 60_000)
      
      this.deps.logger.info({ runID, result, durationMs: Date.now() - startedAt }, "heartbeat run completed")
    } catch (error) {
      await this.handleFailure(error, startedAt, runID)
      this.scheduleNextRun(Math.max(1, this.options.intervalMinutes) * 60_000)
    } finally {
      this.running = false
    }
  }

  private async handleFailure(error: unknown, startedAt: number, runID: string): Promise<void> {
    this.consecutiveFailures += 1

    this.deps.logger.error({
      runID,
      error,
      durationMs: Date.now() - startedAt,
      consecutiveFailures: this.consecutiveFailures,
    }, "heartbeat run failed")

    if (
      this.consecutiveFailures >= this.options.notifyAfterFailures &&
      !this.notifiedForCurrentFailureIncident
    ) {
      const notificationSent = await this.notifyFailure()
      if (notificationSent) {
        this.notifiedForCurrentFailureIncident = true
      }
    }
  }

  private async notifyFailure(): Promise<boolean> {
    const target = this.resolveNotificationTarget(this.deps.channelRepository.getLastChannel())
    if (!target) {
      this.deps.logger.warn("heartbeat consecutive failures but no valid notification target")
      return false
    }

    try {
      this.deps.outboxRepository.enqueue(
        target.channel,
        target.userID,
        `Heartbeat has failed ${this.consecutiveFailures} times in a row. Check logs for details.`
      )

      this.deps.logger.warn(
        { failureCount: this.consecutiveFailures, channel: target.channel },
        "heartbeat notification sent",
      )
      return true
    } catch (error) {
      this.deps.logger.error({ error, failureCount: this.consecutiveFailures }, "heartbeat notification enqueue failed")
      return false
    }
  }

  private resolveNotificationTarget(lastChannel: LastChannel | null): NotificationTarget | null {
    if (!lastChannel) {
      return null
    }

    const channel = lastChannel.channel.trim()
    const userID = lastChannel.userID.trim()
    if (!channel || !userID) {
      return null
    }

    return { channel, userID }
  }

  private scheduleNextRun(delayMs: number): void {
    if (this.stopped) {
      return
    }

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
