/**
 * Syncthing REST client
 *
 * Minimal typed wrapper over selected Syncthing endpoints used by PocketBrain tools.
 */

export interface SyncthingClientOptions {
  baseUrl: string
  apiKey: string
  timeoutMs?: number
}

export interface SyncthingFolderError {
  folder: string
  error: string
  path?: string
}

export interface SyncthingSystemStatus {
  myID?: string
  guiAddressUsed?: string
  uptime?: number
  [key: string]: unknown
}

export interface SyncthingFolderStatus {
  folder?: string
  state?: string
  needBytes?: number
  needFiles?: number
  inSyncBytes?: number
  [key: string]: unknown
}

export class SyncthingClient {
  private readonly baseUrl: string
  private readonly apiKey: string
  private readonly timeoutMs: number

  constructor(options: SyncthingClientOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, "")
    this.apiKey = options.apiKey
    this.timeoutMs = options.timeoutMs ?? 5000
  }

  async ping(): Promise<{ ok: boolean; ping?: string }> {
    return this.getJson<{ ok: boolean; ping?: string }>("/rest/system/ping")
  }

  async systemStatus(): Promise<SyncthingSystemStatus> {
    return this.getJson<SyncthingSystemStatus>("/rest/system/status")
  }

  async folderStatus(folderID: string): Promise<SyncthingFolderStatus> {
    const params = new URLSearchParams({ folder: folderID })
    return this.getJson<SyncthingFolderStatus>(`/rest/db/status?${params.toString()}`)
  }

  async folderErrors(folderID?: string): Promise<{ errors: SyncthingFolderError[] }> {
    const query = folderID ? `?${new URLSearchParams({ folder: folderID }).toString()}` : ""
    return this.getJson<{ errors: SyncthingFolderError[] }>(`/rest/folder/errors${query}`)
  }

  async scanFolder(folderID: string): Promise<{ queued: boolean }> {
    const params = new URLSearchParams({ folder: folderID })
    return this.postJson<{ queued: boolean }>(`/rest/db/scan?${params.toString()}`)
  }

  private async getJson<T>(path: string): Promise<T> {
    return this.requestJson<T>(path, "GET")
  }

  private async postJson<T>(path: string): Promise<T> {
    return this.requestJson<T>(path, "POST")
  }

  private async requestJson<T>(path: string, method: "GET" | "POST"): Promise<T> {
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), this.timeoutMs)

    try {
      const response = await fetch(`${this.baseUrl}${path}`, {
        method,
        signal: controller.signal,
        headers: {
          "X-API-Key": this.apiKey,
        },
      })

      if (!response.ok) {
        const body = await response.text().catch(() => "")
        const details = body ? `: ${body.slice(0, 200)}` : ""
        throw new Error(`Syncthing API ${method} ${path} failed (${response.status})${details}`)
      }

      return (await response.json()) as T
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        throw new Error(`Syncthing API ${method} ${path} timed out after ${this.timeoutMs}ms`)
      }
      throw error
    } finally {
      clearTimeout(timeout)
    }
  }
}
