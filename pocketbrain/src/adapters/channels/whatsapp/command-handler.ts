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

  constructor(options: CommandHandlerOptions) {
    this.options = options
  }

  /**
   * Process a message and handle any commands
   */
  handle(context: CommandContext): CommandResult {
    const { text, jid, isWhitelisted } = context

    // Pair command
    if (text.startsWith("/pair")) {
      return this.handlePair(text, jid)
    }

    // Check whitelist for other commands
    if (!isWhitelisted) {
      return {
        handled: true,
        response: [
          "Access restricted.",
          `Your WhatsApp ID: ${jid}`,
          "Send /pair <token> to whitelist yourself.",
          "If you don't have a token, ask admin to add you to .data/state.db (whitelist table).",
        ].join("\n"),
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
    if (text.startsWith("/remember ")) {
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

  private handlePair(text: string, jid: string): CommandResult {
    const token = text.slice("/pair".length).trim()

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
      return {
        handled: true,
        response: "Invalid pairing token.",
      }
    }

    const isValid = timingSafeEqual(expectedToken, providedToken)
    
    if (!isValid) {
      return {
        handled: true,
        response: "Invalid pairing token.",
      }
    }

    this.options.logger.info({ jid }, "whatsapp pairing successful")
    return {
      handled: true,
      action: "pair",
      payload: jid,
    }
  }
}
