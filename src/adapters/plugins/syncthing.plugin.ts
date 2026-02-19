/**
 * Syncthing Plugin
 *
 * Read-only diagnostics plus guarded folder scan mutation.
 */

import { tool } from "@opencode-ai/plugin"
import { SyncthingClient } from "../syncthing/client"

interface FolderArgs {
  folderID?: string
}

function envBool(value: string | undefined, fallback = false): boolean {
  if (!value) return fallback
  const normalized = value.trim().toLowerCase()
  return normalized === "1" || normalized === "true" || normalized === "yes" || normalized === "on"
}

function parseAllowedFolderIDs(value: string | undefined): Set<string> {
  if (!value) return new Set<string>()
  const out = new Set<string>()
  for (const part of value.split(",")) {
    const id = part.trim()
    if (id) out.add(id)
  }
  return out
}

function normalizeFolderID(argsFolderID: string | undefined, defaultFolderID: string | undefined): string {
  return (argsFolderID?.trim() || defaultFolderID?.trim() || "")
}

export default async function createSyncthingPlugin() {
  const enabled = envBool(Bun.env.SYNCTHING_ENABLED, false)
  if (!enabled) {
    return { tool: {} }
  }

  const baseUrl = Bun.env.SYNCTHING_BASE_URL?.trim() || "http://127.0.0.1:8384"
  const apiKey = Bun.env.SYNCTHING_API_KEY?.trim()
  const timeoutMs = Number.parseInt(Bun.env.SYNCTHING_TIMEOUT_MS?.trim() || "5000", 10)
  const defaultFolderID = Bun.env.SYNCTHING_VAULT_FOLDER_ID?.trim() || "vault"
  const mutationEnabled = envBool(Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED, false)
  const allowedFolderIDs = parseAllowedFolderIDs(Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS)

  if (!apiKey) {
    return {
      tool: {
        syncthing_health: tool({
          description: "Check Syncthing API availability.",
          args: {},
          async execute() {
            return "Syncthing disabled: missing SYNCTHING_API_KEY."
          },
        }),
      },
    }
  }

  const client = new SyncthingClient({
    baseUrl,
    apiKey,
    timeoutMs: Number.isFinite(timeoutMs) && timeoutMs > 0 ? timeoutMs : 5000,
  })

  return {
    tool: {
      syncthing_health: tool({
        description: "Check Syncthing health via /rest/system/ping.",
        args: {},
        async execute() {
          try {
            const result = await client.ping()
            return result.ok ? "Syncthing API reachable." : "Syncthing API reachable but returned unexpected ping response."
          } catch (error) {
            return `Syncthing health check failed: ${error instanceof Error ? error.message : String(error)}`
          }
        },
      }),

      syncthing_status: tool({
        description: "Get Syncthing system status.",
        args: {},
        async execute() {
          try {
            const status = await client.systemStatus()
            const uptime = typeof status.uptime === "number" ? `${status.uptime}s` : "unknown"
            const id = typeof status.myID === "string" ? status.myID : "unknown"
            const gui = typeof status.guiAddressUsed === "string" ? status.guiAddressUsed : "unknown"
            return `Syncthing status: myID=${id}, uptime=${uptime}, guiAddress=${gui}`
          } catch (error) {
            return `Failed to fetch Syncthing status: ${error instanceof Error ? error.message : String(error)}`
          }
        },
      }),

      syncthing_folder_status: tool({
        description: "Get Syncthing folder status by folder ID.",
        args: {
          folderID: tool.schema.string().optional().describe("Syncthing folder ID (defaults to SYNCTHING_VAULT_FOLDER_ID)"),
        },
        async execute(args: FolderArgs) {
          const folderID = normalizeFolderID(args.folderID, defaultFolderID)
          if (!folderID) {
            return "Missing folder ID. Provide folderID or set SYNCTHING_VAULT_FOLDER_ID."
          }

          try {
            const status = await client.folderStatus(folderID)
            const state = typeof status.state === "string" ? status.state : "unknown"
            const needFiles = typeof status.needFiles === "number" ? status.needFiles : 0
            const needBytes = typeof status.needBytes === "number" ? status.needBytes : 0
            return `Folder ${folderID}: state=${state}, needFiles=${needFiles}, needBytes=${needBytes}`
          } catch (error) {
            return `Failed to fetch folder status for ${folderID}: ${error instanceof Error ? error.message : String(error)}`
          }
        },
      }),

      syncthing_folder_errors: tool({
        description: "List Syncthing folder errors (optionally scoped to one folder).",
        args: {
          folderID: tool.schema.string().optional().describe("Optional Syncthing folder ID filter"),
        },
        async execute(args: FolderArgs) {
          const folderID = args.folderID?.trim()
          try {
            const result = await client.folderErrors(folderID)
            const errors = result.errors ?? []
            if (errors.length === 0) {
              return folderID ? `No Syncthing folder errors for ${folderID}.` : "No Syncthing folder errors."
            }
            const lines = errors.slice(0, 20).map((item) => {
              const path = item.path ? ` path=${item.path}` : ""
              return `- folder=${item.folder}${path} error=${item.error}`
            })
            return `Syncthing folder errors (${errors.length}):\n${lines.join("\n")}`
          } catch (error) {
            return `Failed to fetch folder errors: ${error instanceof Error ? error.message : String(error)}`
          }
        },
      }),

      syncthing_scan_folder: tool({
        description: "Trigger a Syncthing folder rescan for an allowed folder ID.",
        args: {
          folderID: tool.schema.string().optional().describe("Folder ID to scan (defaults to SYNCTHING_VAULT_FOLDER_ID)"),
        },
        async execute(args: FolderArgs) {
          if (!mutationEnabled) {
            return "Blocked by policy: SYNCTHING_MUTATION_TOOLS_ENABLED is false."
          }

          const folderID = normalizeFolderID(args.folderID, defaultFolderID)
          if (!folderID) {
            return "Missing folder ID. Provide folderID or set SYNCTHING_VAULT_FOLDER_ID."
          }

          if (allowedFolderIDs.size === 0) {
            return "Blocked by policy: SYNCTHING_ALLOWED_FOLDER_IDS is empty."
          }

          if (!allowedFolderIDs.has(folderID)) {
            return `Blocked by policy: folder '${folderID}' is not in SYNCTHING_ALLOWED_FOLDER_IDS.`
          }

          try {
            await client.scanFolder(folderID)
            return `Triggered Syncthing scan for folder '${folderID}'.`
          } catch (error) {
            return `Failed to trigger scan for ${folderID}: ${error instanceof Error ? error.message : String(error)}`
          }
        },
      }),
    },
  }
}
