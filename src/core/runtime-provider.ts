/**
 * Runtime Provider
 * Manages OpenCode runtime connection.
 * Follows Single Responsibility Principle.
 */

import { createOpencode, createOpencodeClient } from "@opencode-ai/sdk"
import type { Logger } from "pino"

export interface RuntimeProviderOptions {
  model: string | undefined
  serverUrl: string | undefined
  hostname: string
  port: number
  logger: Logger
}

export interface OpencodeRuntime {
  client: ReturnType<typeof createOpencodeClient>
  close?: () => Promise<void> | void
}

export interface ModelConfig {
  providerID: string
  modelID: string
}

export class RuntimeProvider {
  private readonly options: RuntimeProviderOptions
  private runtime: OpencodeRuntime | undefined

  constructor(options: RuntimeProviderOptions) {
    this.options = options
  }

  /**
   * Initialize the runtime
   */
  async init(): Promise<OpencodeRuntime> {
    if (this.runtime) {
      return this.runtime
    }

    this.runtime = await this.createRuntime()
    return this.runtime
  }

  /**
   * Get the client
   */
  getClient(): ReturnType<typeof createOpencodeClient> | undefined {
    return this.runtime?.client
  }

  /**
   * Close the runtime
   */
  async close(): Promise<void> {
    if (typeof this.runtime?.close === "function") {
      await this.runtime.close()
    }
    this.runtime = undefined
  }

  /**
   * Build model configuration from model string
   */
  buildModelConfig(): ModelConfig | undefined {
    if (!this.options.model) return undefined
    const [providerID, ...rest] = this.options.model.split("/")
    if (!providerID || rest.length === 0) return undefined
    return { providerID, modelID: rest.join("/") }
  }

  private async createRuntime(): Promise<OpencodeRuntime> {
    if (this.options.serverUrl) {
      return { client: createOpencodeClient({ baseUrl: this.options.serverUrl }) }
    }

    const fallbackUrl = `http://${this.options.hostname}:${this.options.port}`
    
    try {
      const runtime = await createOpencode({
        hostname: this.options.hostname,
        port: this.options.port,
        ...(this.options.model ? { config: { model: this.options.model } } : {}),
      })
      
      return {
        client: runtime.client,
        close: () => runtime.server.close(),
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      if (message.includes("port") || message.includes("EADDRINUSE")) {
        this.options.logger.warn({ fallbackUrl }, "port in use, connecting to existing server")
        return { client: createOpencodeClient({ baseUrl: fallbackUrl }) }
      }
      throw error
    }
  }
}
