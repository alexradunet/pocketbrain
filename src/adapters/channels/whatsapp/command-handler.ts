/**
 * WhatsApp Command Handler
 * Handles incoming WhatsApp commands like /pair, /remember, /new
 * Follows Single Responsibility Principle.
 */

import type { Logger } from "pino"
import { timingSafeEqual } from "node:crypto"

export interface CommandHandlerOptions {
  pairToken: string | undefined
  logger: Logger
  pairMaxFailures?: number
  pairFailureWindowMs?: number
  pairBlockDurationMs?: number
}

export interface CommandContext {
  jid: string
  text: string
  isWhitelisted: boolean
}

export interface CommandResult {
  handled: boolean
  response?: string
  action?: "remember" | "new_session" | "pair"
  payload?: string
}

export class CommandHandler {
  private readonly options: CommandHandlerOptions
  private readonly pairMaxFailures: number
  private readonly pairFailureWindowMs: number
  private readonly pairBlockDurationMs: number
  private readonly pairFailures = new Map<string, { count: number; windowStartedAt: number; blockedUntil: number }>()

  constructor(options: CommandHandlerOptions) {
    this.options = options
    this.pairMaxFailures = options.pairMaxFailures ?? 5
    this.pairFailureWindowMs = options.pairFailureWindowMs ?? 5 * 60 * 1000
    this.pairBlockDurationMs = options.pairBlockDurationMs ?? 15 * 60 * 1000
  }

  /**
   * Process a message and handle any commands
   */
  handle(context: CommandContext): CommandResult {
    const { text, jid, isWhitelisted } = context

    // Pair command
    const pairToken = this.extractPairToken(text)
    if (pairToken !== null) {
      return this.handlePair(pairToken, jid)
    }

    // Check whitelist for other commands
    if (!isWhitelisted) {
      return {
        handled: true,
        response: this.buildAccessRestrictedResponse(jid),
      }
    }

    // New session command
    if (text === "/new") {
      return {
        handled: true,
        response: "Starting a new conversation session...",
        action: "new_session",
      }
    }

    // Remember command
    if (text === "/remember" || text.startsWith("/remember ")) {
      const note = text.slice(10).trim()
      if (note.length > 0) {
        return {
          handled: true,
          response: "Saved to long-term memory.",
          action: "remember",
          payload: note,
        }
      }
      return { handled: true, response: "Usage: /remember <text>" }
    }

    // Not a command - pass through
    return { handled: false }
  }

  private extractPairToken(text: string): string | null {
    const match = text.match(/^\/pair(?:\s+(.+))?$/)
    if (!match) {
      return null
    }

    return match[1]?.trim() ?? ""
  }

  private buildAccessRestrictedResponse(jid: string): string {
    const lines = ["Access restricted.", `Your WhatsApp ID: ${jid}`]

    if (this.options.pairToken) {
      lines.push("Send /pair <token> to whitelist yourself.")
    } else {
      lines.push("Pairing is disabled by admin.")
    }

    lines.push("If you need access, ask admin to add you to the WhatsApp whitelist.")
    return lines.join("\n")
  }

  private handlePair(token: string, jid: string): CommandResult {
    if (this.isPairBlocked(jid)) {
      return {
        handled: true,
        response: "Too many failed pairing attempts. Please wait before trying again.",
      }
    }

    if (!this.options.pairToken) {
      return {
        handled: true,
        response: "Pairing is disabled by admin. Ask admin to whitelist your account.",
      }
    }

    if (!token) {
      return {
        handled: true,
        response: "Usage: /pair <token>",
      }
    }

    // Timing-safe comparison to prevent timing attacks
    const expectedToken = Buffer.from(this.options.pairToken)
    const providedToken = Buffer.from(token)
    
    if (expectedToken.length !== providedToken.length) {
      if (this.registerPairFailure(jid)) {
        return {
          handled: true,
          response: "Too many failed pairing attempts. Please wait before trying again.",
        }
      }
      return {
        handled: true,
        response: "Invalid pairing token.",
      }
    }

    const isValid = timingSafeEqual(expectedToken, providedToken)
    
    if (!isValid) {
      if (this.registerPairFailure(jid)) {
        return {
          handled: true,
          response: "Too many failed pairing attempts. Please wait before trying again.",
        }
      }
      return {
        handled: true,
        response: "Invalid pairing token.",
      }
    }

    this.pairFailures.delete(jid)

    this.options.logger.info({ jid }, "whatsapp pairing successful")
    return {
      handled: true,
      action: "pair",
      payload: jid,
    }
  }

  private isPairBlocked(jid: string): boolean {
    const state = this.pairFailures.get(jid)
    if (!state) {
      return false
    }

    const now = Date.now()
    if (state.blockedUntil > now) {
      return true
    }

    if (state.blockedUntil > 0 && state.blockedUntil <= now) {
      this.pairFailures.delete(jid)
    }

    return false
  }

  private registerPairFailure(jid: string): boolean {
    const now = Date.now()
    const existing = this.pairFailures.get(jid)

    if (!existing || now - existing.windowStartedAt > this.pairFailureWindowMs) {
      const blocked = this.pairMaxFailures <= 1
      this.pairFailures.set(jid, {
        count: 1,
        windowStartedAt: now,
        blockedUntil: blocked ? now + this.pairBlockDurationMs : 0,
      })
      return blocked
    }

    const nextCount = existing.count + 1
    const blocked = nextCount >= this.pairMaxFailures
    this.pairFailures.set(jid, {
      count: nextCount,
      windowStartedAt: existing.windowStartedAt,
      blockedUntil: blocked ? now + this.pairBlockDurationMs : 0,
    })

    return blocked
  }
}
