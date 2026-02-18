/**
 * Vault Service
 * 
 * High-level vault operations for PocketBrain integration.
 * Manages file operations, daily notes.
 * 
 * File synchronization is handled externally by Syncthing.
 * This service only handles file operations, not networking.
 */

import { join, dirname, resolve, relative, sep } from "node:path"
import { mkdir, readdir, stat, rename } from "node:fs/promises"
import { normalizeWikiLinkTarget, parseWikiLinks } from "../lib/markdown-links"
import { extractMarkdownTags } from "../lib/markdown-tags"

export interface VaultFile {
  path: string
  name: string
  size: number
  modified: Date
  isDirectory: boolean
}

export interface VaultOptions {
  vaultPath: string
  dailyNoteFormat: string
  folders: {
    inbox: string
    daily: string
    journal: string
    projects: string
    areas: string
    resources: string
    archive: string
  }
}

export type VaultSearchMode = "name" | "content" | "both"

export class VaultService {
  constructor(private options: VaultOptions) {}

  private get vaultRootPath(): string {
    return resolve(this.options.vaultPath)
  }

  /**
   * Initialize vault directory structure
   */
  async initialize(): Promise<void> {
    const folders = [
      this.options.folders.inbox,
      this.options.folders.daily,
      this.options.folders.journal,
      this.options.folders.projects,
      this.options.folders.areas,
      this.options.folders.resources,
      this.options.folders.archive,
    ]

    for (const folder of folders) {
      const folderPath = join(this.options.vaultPath, folder)
      await mkdir(folderPath, { recursive: true })
    }

    console.log("[Vault] Directory structure initialized")
  }

  /**
   * Read file content
   */
  async readFile(relativePath: string): Promise<string | null> {
    try {
      const filePath = this.resolvePathWithinVault(relativePath)
      if (!filePath) return null
      const file = Bun.file(filePath)
      return await file.text()
    } catch {
      return null
    }
  }

  /**
   * Write file content
   */
  async writeFile(relativePath: string, content: string): Promise<boolean> {
    try {
      const filePath = this.resolvePathWithinVault(relativePath)
      if (!filePath) return false
      await mkdir(dirname(filePath), { recursive: true })
      await Bun.write(filePath, content)
      return true
    } catch (error) {
      console.error("[Vault] Write error:", error)
      return false
    }
  }

  /**
   * Append content to file (for daily notes)
   */
  async appendToFile(relativePath: string, content: string): Promise<boolean> {
    try {
      // Check if file exists
      const existing = await this.readFile(relativePath)
      
      if (existing !== null) {
        await this.writeFile(relativePath, existing + content)
      } else {
        await this.writeFile(relativePath, content)
      }

      return true
    } catch (error) {
      console.error("[Vault] Append error:", error)
      return false
    }
  }

  /**
   * List files in a folder
   */
  async listFiles(folderPath: string = ""): Promise<VaultFile[]> {
    try {
      const fullPath = this.resolvePathWithinVault(folderPath, true)
      if (!fullPath) return []
      const entries = await readdir(fullPath, { withFileTypes: true })

      const files: VaultFile[] = []

      for (const entry of entries) {
        if (entry.name.startsWith(".")) continue

        const entryPath = join(fullPath, entry.name)
        const stats = await stat(entryPath)

        files.push({
          path: join(folderPath, entry.name),
          name: entry.name,
          size: stats.size,
          modified: stats.mtime,
          isDirectory: entry.isDirectory(),
        })
      }

      return files.sort((a, b) => {
        // Directories first, then by name
        if (a.isDirectory !== b.isDirectory) {
          return a.isDirectory ? -1 : 1
        }
        return a.name.localeCompare(b.name)
      })
    } catch {
      return []
    }
  }

  async searchFiles(
    query: string,
    folder: string = "",
    mode: VaultSearchMode = "name",
  ): Promise<VaultFile[]> {
    const allFiles = await this.listFilesRecursive(folder)
    const lowerQuery = query.toLowerCase()
    const searchMode = this.normalizeSearchMode(mode)

    if (searchMode === "name") {
      return allFiles.filter((file) => file.name.toLowerCase().includes(lowerQuery))
    }

    const matched: VaultFile[] = []

    for (const file of allFiles) {
      if (file.isDirectory) {
        continue
      }

      const nameMatch = file.name.toLowerCase().includes(lowerQuery)
      if (searchMode === "both" && nameMatch) {
        matched.push(file)
        continue
      }

      const content = await this.readFile(file.path)
      if (content && content.toLowerCase().includes(lowerQuery)) {
        matched.push(file)
      }
    }

    return matched
  }

  async findBacklinks(target: string, folder: string = ""): Promise<VaultFile[]> {
    const normalizedTarget = normalizeWikiLinkTarget(target)
    const allFiles = await this.listFilesRecursive(folder)
    const matches: VaultFile[] = []

    for (const file of allFiles) {
      if (file.isDirectory) {
        continue
      }

      const content = await this.readFile(file.path)
      if (!content) {
        continue
      }

      const links = parseWikiLinks(content)
      if (links.some((link) => link.normalizedTarget === normalizedTarget)) {
        matches.push(file)
      }
    }

    return matches
  }

  async searchByTag(tag: string, folder: string = ""): Promise<VaultFile[]> {
    const normalizedTag = normalizeTag(tag)
    const allFiles = await this.listFilesRecursive(folder)
    const matches: VaultFile[] = []

    for (const file of allFiles) {
      if (file.isDirectory) {
        continue
      }

      const content = await this.readFile(file.path)
      if (!content) {
        continue
      }

      const tags = extractMarkdownTags(content)
      if (tags.includes(normalizedTag)) {
        matches.push(file)
      }
    }

    return matches
  }

  /**
   * Get today's daily note path
   */
  getDailyNotePath(): string {
    const today = new Date()
    const dateStr = today.toISOString().split("T")[0] // YYYY-MM-DD
    return join(this.options.folders.daily, `${dateStr}.md`)
  }

  /**
   * Append to today's daily note
   */
  async appendToDaily(content: string): Promise<boolean> {
    const dailyPath = this.getDailyNotePath()

    // Add timestamp header if new file
    const exists = await this.readFile(dailyPath)
    if (exists === null) {
      const dateStr = new Date().toISOString().split("T")[0]
      const header = `# ${dateStr}\n\n`
      await this.writeFile(dailyPath, header)
    }

    return this.appendToFile(dailyPath, content)
  }

  /**
   * Move file between folders
   */
  async moveFile(fromPath: string, toPath: string): Promise<boolean> {
    try {
      const source = this.resolvePathWithinVault(fromPath)
      const dest = this.resolvePathWithinVault(toPath)
      if (!source || !dest) return false

      await mkdir(dirname(dest), { recursive: true })
      await rename(source, dest)

      return true
    } catch (error) {
      console.error("[Vault] Move error:", error)
      return false
    }
  }

  /**
   * Cleanup (nothing to do - Syncthing handles sync)
   */
  async stop(): Promise<void> {
    // No-op: Syncthing runs in separate container
  }

  /**
   * Get vault statistics
   */
  async getStats(): Promise<{
    totalFiles: number
    totalSize: number
    lastModified: Date | null
  }> {
    const files = await this.listFilesRecursive("")

    let totalSize = 0
    let lastModified: Date | null = null

    for (const file of files) {
      if (!file.isDirectory) {
        totalSize += file.size
        if (!lastModified || file.modified > lastModified) {
          lastModified = file.modified
        }
      }
    }

    return {
      totalFiles: files.length,
      totalSize,
      lastModified,
    }
  }

  /**
   * List all files recursively
   */
  private async listFilesRecursive(folder: string): Promise<VaultFile[]> {
    const files: VaultFile[] = []
    const items = await this.listFiles(folder)

    for (const item of items) {
      files.push(item)

      if (item.isDirectory) {
        const children = await this.listFilesRecursive(item.path)
        files.push(...children)
      }
    }

    return files
  }

  private resolvePathWithinVault(inputPath: string, allowRoot = false): string | null {
    const trimmed = inputPath.trim()
    if (!allowRoot && trimmed.length === 0) {
      return null
    }

    const resolvedPath = resolve(this.vaultRootPath, trimmed)
    const rel = relative(this.vaultRootPath, resolvedPath)
    if (rel === ".." || rel.startsWith(`..${sep}`)) {
      return null
    }

    return resolvedPath
  }

  private normalizeSearchMode(mode: VaultSearchMode | string): VaultSearchMode {
    return mode === "content" || mode === "both" || mode === "name" ? mode : "name"
  }
}

function normalizeTag(tag: string): string {
  const trimmed = tag.trim().toLowerCase()
  if (trimmed.length === 0) return "#"
  return trimmed.startsWith("#") ? trimmed : `#${trimmed}`
}

/**
 * Create vault service with default configuration
 */
export function createVaultService(vaultPath: string): VaultService {
  return new VaultService({
    vaultPath,
    dailyNoteFormat: "YYYY-MM-DD",
    folders: {
      inbox: "inbox",
      daily: "daily",
      journal: "journal",
      projects: "projects",
      areas: "areas",
      resources: "resources",
      archive: "archive",
    },
  })
}
