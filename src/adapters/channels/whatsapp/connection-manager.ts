/**
 * WhatsApp Connection Manager
 * Handles Baileys connection lifecycle and events.
 * Follows Single Responsibility Principle.
 */

import {
  DisconnectReason,
  useMultiFileAuthState,
  type WASocket,
} from "@whiskeysockets/baileys"
import makeWASocket from "@whiskeysockets/baileys"
import type { Logger } from "pino"
import qrcode from "qrcode-terminal"

export interface ConnectionManagerOptions {
  authDir: string
  logger: Logger
  connectingTimeoutMs?: number
  reconnectDelayMs?: number
  onQr?: (qr: string) => void
  onOpen?: () => void
  onClose?: (shouldReconnect: boolean, reason: string) => void
}

export interface ConnectionState {
  connected: boolean
  socket?: WASocket
}

export class ConnectionManager {
  private readonly options: ConnectionManagerOptions
  private socket?: WASocket
  private connected = false
  private reconnectScheduled = false
  private connectTimeout?: ReturnType<typeof setTimeout>
  private reconnectTimeout?: ReturnType<typeof setTimeout>
  private stopping = false

  constructor(options: ConnectionManagerOptions) {
    this.options = options
  }

  /**
   * Initialize the connection
   */
  async connect(): Promise<WASocket> {
    if (this.socket && this.connected) {
      return this.socket
    }

    this.stopping = false
    const { state, saveCreds } = await useMultiFileAuthState(this.options.authDir)
    
    this.socket = makeWASocket({ auth: state })
    this.setupEventHandlers(saveCreds)
    
    return this.socket
  }

  /**
   * Stop the connection
   */
  stop(): void {
    this.stopping = true
    this.clearConnectTimeout()
    this.clearReconnectTimeout()
    this.reconnectScheduled = false
    this.socket?.end?.(new Error("stopping"))
    this.socket = undefined
    this.connected = false
  }

  /**
   * Check if connected
   */
  isConnected(): boolean {
    return this.connected
  }

  /**
   * Get the socket
   */
  getSocket(): WASocket | undefined {
    return this.socket
  }

  /**
   * Schedule a reconnection
   */
  scheduleReconnect(delayMs: number, reason: string): void {
    if (this.reconnectScheduled || this.stopping) return
    
    this.reconnectScheduled = true
    this.connected = false
    this.clearConnectTimeout()
    this.clearReconnectTimeout()
    
    this.options.logger.warn({ delayMs, reason }, "whatsapp reconnect scheduled")
    
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = undefined

      if (this.stopping) {
        return
      }

      this.reconnectScheduled = false
      void this.connect()
    }, delayMs)
  }

  private setupEventHandlers(saveCreds: (creds: unknown) => Promise<void>): void {
    if (!this.socket) return

    this.socket.ev.on("creds.update", saveCreds)

    this.socket.ev.on("connection.update", (update: ConnectionUpdate) => {
      void this.handleConnectionUpdate(update)
    })
  }

  private async handleConnectionUpdate(update: ConnectionUpdate): Promise<void> {
    const { connection, lastDisconnect, qr } = update

    if (qr) {
      qrcode.generate(qr, { small: true })
      this.options.onQr?.(qr)
      this.options.logger.info("whatsapp qr generated")
    }

    if (connection === "connecting" && !this.connectTimeout) {
      this.connectTimeout = setTimeout(() => {
        this.options.logger.warn("whatsapp connection stuck; restarting adapter")
        this.scheduleReconnect(0, "connecting timeout")
        this.socket?.end?.(new Error("restart"))
      }, this.options.connectingTimeoutMs ?? 20_000)
    }

    if (connection === "open") {
      this.handleOpen()
      return
    }

    if (connection === "close") {
      this.handleClose(lastDisconnect)
    }
  }

  private handleOpen(): void {
    this.connected = true
    this.reconnectScheduled = false
    this.clearConnectTimeout()
    this.clearReconnectTimeout()
    this.options.logger.info("whatsapp adapter connected")
    this.options.onOpen?.()
  }

  private handleClose(lastDisconnect?: { error?: ConnectionError }): void {
    this.connected = false
    this.clearConnectTimeout()

    const statusCode = lastDisconnect?.error?.output?.statusCode
    const message = lastDisconnect?.error?.message
    const streamError = message?.includes("Stream Errored") || 
                       message?.includes("Stream Errored (restart required)")
    const shouldReconnect = statusCode !== DisconnectReason.loggedOut

    this.options.logger.warn({ statusCode, shouldReconnect, streamError }, "whatsapp connection closed")

    if (shouldReconnect || streamError) {
      this.options.onClose?.(true, "connection close")
      this.scheduleReconnect(this.options.reconnectDelayMs ?? 3_000, "connection close")
    } else {
      this.options.onClose?.(false, "logged out")
    }
  }

  private clearConnectTimeout(): void {
    if (this.connectTimeout) {
      clearTimeout(this.connectTimeout)
      this.connectTimeout = undefined
    }
  }

  private clearReconnectTimeout(): void {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout)
      this.reconnectTimeout = undefined
    }
  }
}

// Type definitions for Baileys events
interface ConnectionUpdate {
  connection?: "connecting" | "open" | "close"
  lastDisconnect?: { error?: ConnectionError }
  qr?: string
}

interface ConnectionError {
  output?: { statusCode?: number }
  message?: string
}
