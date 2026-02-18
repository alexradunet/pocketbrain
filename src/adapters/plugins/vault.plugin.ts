/**
 * Vault Plugin
 * 
 * Provides tools for the agent to read/write/search the vault.
 */

import { tool } from "@opencode-ai/plugin"
import { vaultProvider } from "../../vault/vault-provider"

interface VaultReadArgs {
  path: string
}

interface VaultWriteArgs {
  path: string
  content: string
}

interface VaultAppendArgs {
  path: string
  content: string
}

interface VaultListArgs {
  folder?: string
}

interface VaultSearchArgs {
  query: string
  folder?: string
}

interface VaultMoveArgs {
  from: string
  to: string
}

interface VaultDailyArgs {
  content?: string
}

export default async function createVaultPlugin() {
  const vaultService = vaultProvider.getVaultService()
  
  // If vault is not enabled, return empty tool set
  if (!vaultService) {
    return { tool: {} }
  }

  return {
    tool: {
      vault_read: tool({
        description: "Read the contents of a file from the vault. Path is relative to vault root (e.g., 'daily/2026-02-18.md' or 'projects/my-project.md')",
        args: {
          path: tool.schema.string().describe("Path to the file, relative to vault root"),
        },
        async execute(args: VaultReadArgs) {
          const content = await vaultService.readFile(args.path)
          if (content === null) {
            return `Error: File not found: ${args.path}`
          }
          return content
        },
      }),

      vault_write: tool({
        description: "Write content to a file in the vault. Creates the file if it doesn't exist, overwrites if it does. Use for creating new notes or replacing existing ones.",
        args: {
          path: tool.schema.string().describe("Path to the file, relative to vault root"),
          content: tool.schema.string().describe("Content to write to the file"),
        },
        async execute(args: VaultWriteArgs) {
          const success = await vaultService.writeFile(args.path, args.content)
          if (success) {
            return `Successfully wrote to ${args.path}`
          }
          return `Error: Failed to write to ${args.path}`
        },
      }),

      vault_append: tool({
        description: "Append content to a file in the vault. Creates the file if it doesn't exist. Useful for adding entries to daily notes or logs.",
        args: {
          path: tool.schema.string().describe("Path to the file, relative to vault root"),
          content: tool.schema.string().describe("Content to append to the file"),
        },
        async execute(args: VaultAppendArgs) {
          const success = await vaultService.appendToFile(args.path, args.content)
          if (success) {
            return `Successfully appended to ${args.path}`
          }
          return `Error: Failed to append to ${args.path}`
        },
      }),

      vault_list: tool({
        description: "List files and folders in a vault directory. Returns file names, sizes, and modification dates.",
        args: {
          folder: tool.schema.string().optional().describe("Folder path relative to vault root (default: root)"),
        },
        async execute(args: VaultListArgs) {
          const files = await vaultService.listFiles(args.folder || '')
          
          if (files.length === 0) {
            return `Folder is empty: ${args.folder || 'root'}`
          }
          
          const lines = files.map(f => {
            const type = f.isDirectory ? '📁' : '📄'
            const size = f.isDirectory ? '' : ` (${formatBytes(f.size)})`
            return `${type} ${f.name}${size}`
          })
          
          return `Contents of ${args.folder || 'vault root'}:\n${lines.join('\n')}`
        },
      }),

      vault_search: tool({
        description: "Search for files by name in the vault. Returns matching file paths.",
        args: {
          query: tool.schema.string().describe("Search query (matched against file names)"),
          folder: tool.schema.string().optional().describe("Folder to search in (default: entire vault)"),
        },
        async execute(args: VaultSearchArgs) {
          const files = await vaultService.searchFiles(args.query, args.folder || '')
          
          if (files.length === 0) {
            return `No files found matching "${args.query}"`
          }
          
          const lines = files.map(f => `- ${f.path}`)
          return `Found ${files.length} file(s) matching "${args.query}":\n${lines.join('\n')}`
        },
      }),

      vault_move: tool({
        description: "Move or rename a file in the vault. Can move between folders.",
        args: {
          from: tool.schema.string().describe("Source path relative to vault root"),
          to: tool.schema.string().describe("Destination path relative to vault root"),
        },
        async execute(args: VaultMoveArgs) {
          const success = await vaultService.moveFile(args.from, args.to)
          if (success) {
            return `Successfully moved ${args.from} to ${args.to}`
          }
          return `Error: Failed to move ${args.from}`
        },
      }),

      vault_daily: tool({
        description: "Get today's daily note path or append to it. Creates the daily note if it doesn't exist. Daily notes are stored in daily/YYYY-MM-DD.md",
        args: {
          content: tool.schema.string().optional().describe("Content to append to today's daily note (if not provided, returns the path)"),
        },
        async execute(args: VaultDailyArgs) {
          const dailyPath = vaultService.getDailyNotePath()
          
          if (!args.content) {
            // Just return the path
            const exists = await vaultService.readFile(dailyPath)
            if (exists === null) {
              return `Today's daily note: ${dailyPath} (doesn't exist yet)`
            }
            return `Today's daily note: ${dailyPath}`
          }
          
          // Append to daily note
          const success = await vaultService.appendToDaily(args.content)
          if (success) {
            return `Successfully added to today's daily note (${dailyPath})`
          }
          return `Error: Failed to update daily note`
        },
      }),

      vault_stats: tool({
        description: "Get statistics about the vault: total files, total size, last modified date",
        args: {},
        async execute() {
          const stats = await vaultService.getStats()
          return `Vault Statistics:
- Total files: ${stats.totalFiles}
- Total size: ${formatBytes(stats.totalSize)}
- Last modified: ${stats.lastModified?.toISOString() || 'N/A'}`
        },
      }),
    },
  }
}

/**
 * Format bytes to human-readable string
 */
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
