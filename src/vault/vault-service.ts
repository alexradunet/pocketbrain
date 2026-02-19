/**
 * Vault Service
 * 
 * High-level vault operations for PocketBrain integration.
 * Manages file operations, daily notes.
 * 
 * File synchronization is handled externally by Taildrive.
 * This service only handles file operations, not networking.
 */

import { join, dirname, resolve, relative, sep } from "node:path"
import { mkdir, readdir, stat, lstat, rename, realpath } from "node:fs/promises"
import type { Logger } from "pino"
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
  logger?: Logger
  folders: VaultFolders
}

export interface VaultFolders {
  inbox: string
  daily: string
  journal: string
  projects: string
  areas: string
  resources: string
  archive: string
}

export type VaultSearchMode = "name" | "content" | "both"

export interface ObsidianConfigSummary {
  obsidianConfigFound: boolean
  dailyNotes: {
    folder: string
    format: string
    templateFile: string
    pluginEnabled: boolean
  }
  newNotes: {
    location: "current" | "folder" | "root" | "unknown"
    folder: string
  }
  attachments: {
    folder: string
  }
  links: {
    style: "wikilink" | "markdown"
  }
  templates: {
    folder: string
  }
  warnings: string[]
}

export interface ObsidianConfigState {
  summary: ObsidianConfigSummary
  fingerprint: string
  cacheHit: boolean
}

interface DailyNoteSettings {
  folder: string
  format: string
  templateFile: string | null
}

export const DEFAULT_VAULT_FOLDERS: VaultFolders = {
  inbox: "inbox",
  daily: "daily",
  journal: "journal",
  projects: "projects",
  areas: "areas",
  resources: "resources",
  archive: "archive",
}

export class VaultService {
  private obsidianConfigCache: {
    fingerprint: string
    summary: ObsidianConfigSummary
  } | null = null

  constructor(private options: VaultOptions) {}

  private get vaultRootPath(): string {
    return resolve(this.options.vaultPath)
  }

  /**
   * Initialize vault directory structure
   */
  async initialize(): Promise<void> {
    await mkdir(this.options.vaultPath, { recursive: true })

    this.options.logger?.info("vault initialized")
  }

  /**
   * Read file content
   */
  async readFile(relativePath: string): Promise<string | null> {
    try {
      const filePath = await this.resolveExistingPathWithinVault(relativePath)
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
      const filePath = await this.resolveWritablePathWithinVault(relativePath)
      if (!filePath) return false
      await mkdir(dirname(filePath), { recursive: true })

      // Re-check in case a symlink appeared between validation and mkdir/write.
      if (!(await this.isWritablePathSafe(filePath))) {
        return false
      }

      await Bun.write(filePath, content)
      return true
    } catch (error) {
      this.options.logger?.error({ error }, "vault write failed")
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
        return await this.writeFile(relativePath, existing + content)
      }

      return await this.writeFile(relativePath, content)
    } catch (error) {
      this.options.logger?.error({ error }, "vault append failed")
      return false
    }
  }

  /**
   * List files in a folder
   */
  async listFiles(folderPath: string = ""): Promise<VaultFile[]> {
    try {
      const fullPath = await this.resolveExistingPathWithinVault(folderPath, true)
      if (!fullPath) return []
      const entries = await readdir(fullPath, { withFileTypes: true })

      const files: VaultFile[] = []

      for (const entry of entries) {
        if (entry.name.startsWith(".")) continue

        const entryPath = join(fullPath, entry.name)
        const stats = await lstat(entryPath)
        if (stats.isSymbolicLink()) {
          continue
        }

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
   * Resolve today's daily note path based on Obsidian config.
   */
  async getTodayDailyNotePath(): Promise<string> {
    const settings = await this.resolveDailyNoteSettings()
    const dateStr = formatObsidianDate(new Date(), settings.format)
    return join(settings.folder, `${dateStr}.md`)
  }

  /**
   * Append to today's daily note
   */
  async appendToDaily(content: string): Promise<boolean> {
    const trimmedContent = content.trim()
    if (trimmedContent.length === 0) return false

    const now = new Date()
    const settings = await this.resolveDailyNoteSettings()
    const dailyPath = join(settings.folder, `${formatObsidianDate(now, settings.format)}.md`)

    let noteContent = await this.readFile(dailyPath)
    if (noteContent === null) {
      noteContent = await this.buildNewDailyNote(settings, now)
    }

    noteContent = ensureHeadingSection(noteContent, "## Timeline")
    noteContent = ensureHeadingSection(noteContent, "## Tracking")
    const line = `- ${formatHourMinute(now)} ${trimmedContent}`
    noteContent = appendLineToSection(noteContent, "## Timeline", line)

    return this.writeFile(dailyPath, noteContent)
  }

  /**
   * Upsert a daily tracking key/value pair in today's daily note.
   */
  async upsertDailyTracking(metric: string, value: string): Promise<boolean> {
    const normalizedMetric = metric.trim().replace(/:+$/g, "")
    const normalizedValue = value.trim()
    if (!normalizedMetric || !normalizedValue) return false

    const now = new Date()
    const settings = await this.resolveDailyNoteSettings()
    const dailyPath = join(settings.folder, `${formatObsidianDate(now, settings.format)}.md`)

    let noteContent = await this.readFile(dailyPath)
    if (noteContent === null) {
      noteContent = await this.buildNewDailyNote(settings, now)
    }

    noteContent = ensureHeadingSection(noteContent, "## Tracking")
    noteContent = upsertTrackingLine(noteContent, "## Tracking", normalizedMetric, normalizedValue)

    return this.writeFile(dailyPath, noteContent)
  }

  /**
   * Read Obsidian configuration and summarize authoring locations.
   */
  async getObsidianConfigSummary(forceRefresh = false): Promise<ObsidianConfigSummary> {
    const state = await this.getObsidianConfigState(forceRefresh)
    return state.summary
  }

  /**
   * Read Obsidian configuration with cache state details.
   */
  async getObsidianConfigState(forceRefresh = false): Promise<ObsidianConfigState> {
    const fingerprint = await this.computeVaultFingerprint()
    const cacheHit = !forceRefresh && this.obsidianConfigCache?.fingerprint === fingerprint

    if (cacheHit && this.obsidianConfigCache) {
      return {
        summary: this.obsidianConfigCache.summary,
        fingerprint,
        cacheHit: true,
      }
    }

    const summary = await this.buildObsidianConfigSummary()
    this.obsidianConfigCache = { fingerprint, summary }

    return {
      summary,
      fingerprint,
      cacheHit: false,
    }
  }

  private async buildObsidianConfigSummary(): Promise<ObsidianConfigSummary> {
    const app = await this.readObsidianJson<Record<string, unknown>>("app.json")
    const daily = await this.readObsidianJson<Record<string, unknown>>("daily-notes.json")
    const templates = await this.readObsidianJson<Record<string, unknown>>("templates.json")
    const corePlugins = await this.readObsidianJson<unknown>("core-plugins.json")

    const dailyFolder = asTrimmedString(daily?.folder) || this.options.folders.daily
    const dailyFormat = asTrimmedString(daily?.format) || "YYYY-MM-DD"
    const dailyTemplate = asTrimmedString(daily?.template)
    const pluginEnabled = Array.isArray(corePlugins) && corePlugins.includes("daily-notes")

    const newFileLocationRaw = asTrimmedString(app?.newFileLocation)
    const newFileLocation = normalizeNewFileLocation(newFileLocationRaw)
    const newFileFolder = asTrimmedString(app?.newFileFolderPath)

    const attachmentFolderRaw = asTrimmedString(app?.attachmentFolderPath)
    const useMarkdownLinks = !!app?.useMarkdownLinks
    const templatesFolder = asTrimmedString(templates?.folder)

    const warnings: string[] = []

    if (!pluginEnabled) {
      warnings.push("daily-notes core plugin is not enabled in core-plugins.json")
    }

    if (newFileLocation === "folder" && !newFileFolder) {
      warnings.push("newFileLocation is set to folder but newFileFolderPath is empty")
    }

    if (attachmentFolderRaw === "/") {
      warnings.push("attachments are configured to save at the vault root")
    }

    const obsidianConfigFound = !!(app || daily || templates || corePlugins)

    return {
      obsidianConfigFound,
      dailyNotes: {
        folder: dailyFolder,
        format: dailyFormat,
        templateFile: dailyTemplate || "(none)",
        pluginEnabled,
      },
      newNotes: {
        location: newFileLocation,
        folder: newFileFolder || "(not set)",
      },
      attachments: {
        folder: attachmentFolderRaw || "(current note folder)",
      },
      links: {
        style: useMarkdownLinks ? "markdown" : "wikilink",
      },
      templates: {
        folder: templatesFolder || "(not set)",
      },
      warnings,
    }
  }

  /**
   * Move file between folders
   */
  async moveFile(fromPath: string, toPath: string): Promise<boolean> {
    try {
      const source = await this.resolveExistingPathWithinVault(fromPath)
      const dest = await this.resolveWritablePathWithinVault(toPath)
      if (!source || !dest) return false

      await mkdir(dirname(dest), { recursive: true })

      if (!(await this.isWritablePathSafe(dest))) {
        return false
      }

      await rename(source, dest)

      return true
    } catch (error) {
      this.options.logger?.error({ error }, "vault move failed")
      return false
    }
  }

  /**
   * Cleanup (nothing to do - Taildrive handles sync)
   */
  async stop(): Promise<void> {
    // No-op: Taildrive handles sync externally
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

  private async resolveExistingPathWithinVault(inputPath: string, allowRoot = false): Promise<string | null> {
    const resolvedPath = this.resolvePathWithinVault(inputPath, allowRoot)
    if (!resolvedPath) {
      return null
    }

    if (!(await this.isExistingPathSafe(resolvedPath))) {
      return null
    }

    return resolvedPath
  }

  private async resolveWritablePathWithinVault(inputPath: string): Promise<string | null> {
    const resolvedPath = this.resolvePathWithinVault(inputPath)
    if (!resolvedPath) {
      return null
    }

    if (!(await this.isWritablePathSafe(resolvedPath))) {
      return null
    }

    return resolvedPath
  }

  private async isExistingPathSafe(targetPath: string): Promise<boolean> {
    if (!(await this.hasNoSymlinkSegments(targetPath))) {
      return false
    }

    try {
      const [rootReal, targetReal] = await Promise.all([
        realpath(this.vaultRootPath),
        realpath(targetPath),
      ])
      return isWithinRoot(rootReal, targetReal)
    } catch {
      return false
    }
  }

  private async isWritablePathSafe(targetPath: string): Promise<boolean> {
    if (!(await this.hasNoSymlinkSegments(targetPath))) {
      return false
    }

    const nearestExistingAncestor = await this.findNearestExistingAncestor(targetPath)
    if (!nearestExistingAncestor) {
      return false
    }

    try {
      const [rootReal, ancestorReal] = await Promise.all([
        realpath(this.vaultRootPath),
        realpath(nearestExistingAncestor),
      ])
      if (!isWithinRoot(rootReal, ancestorReal)) {
        return false
      }

      const targetStat = await lstat(targetPath).catch(() => null)
      if (targetStat?.isSymbolicLink()) {
        return false
      }

      if (targetStat) {
        const targetReal = await realpath(targetPath)
        return isWithinRoot(rootReal, targetReal)
      }

      return true
    } catch {
      return false
    }
  }

  private async hasNoSymlinkSegments(targetPath: string): Promise<boolean> {
    const relToRoot = relative(this.vaultRootPath, targetPath)
    if (relToRoot.length === 0) {
      return true
    }

    const segments = relToRoot.split(sep).filter(Boolean)
    let current = this.vaultRootPath

    for (const segment of segments) {
      current = join(current, segment)

      try {
        const stats = await lstat(current)
        if (stats.isSymbolicLink()) {
          return false
        }
      } catch (error) {
        if (isNotFoundError(error)) {
          continue
        }
        return false
      }
    }

    return true
  }

  private async findNearestExistingAncestor(targetPath: string): Promise<string | null> {
    let current = targetPath

    while (true) {
      try {
        await lstat(current)
        return current
      } catch (error) {
        if (!isNotFoundError(error)) {
          return null
        }

        const parent = dirname(current)
        if (parent === current) {
          return null
        }
        current = parent
      }
    }
  }

  private normalizeSearchMode(mode: VaultSearchMode | string): VaultSearchMode {
    return mode === "content" || mode === "both" || mode === "name" ? mode : "name"
  }

  private async computeVaultFingerprint(): Promise<string> {
    const topLevelDirs = await this.getTopLevelDirectories()
    const obsidianFileSignatures = await Promise.all(
      ["app.json", "daily-notes.json", "templates.json", "core-plugins.json"].map((fileName) =>
        this.getFileSignature(join(".obsidian", fileName)),
      ),
    )

    return [
      `dirs=${topLevelDirs.join(",")}`,
      `obsidian=${obsidianFileSignatures.join("|")}`,
    ].join(";")
  }

  private async getTopLevelDirectories(): Promise<string[]> {
    try {
      const entries = await readdir(this.vaultRootPath, { withFileTypes: true })
      return entries
        .filter((entry) => entry.isDirectory() && !entry.name.startsWith("."))
        .map((entry) => entry.name)
        .sort((a, b) => a.localeCompare(b))
    } catch {
      return []
    }
  }

  private async getFileSignature(relativePath: string): Promise<string> {
    const absolutePath = await this.resolveExistingPathWithinVault(relativePath)
    if (!absolutePath) {
      return `${relativePath}:missing`
    }

    try {
      const fileStat = await stat(absolutePath)
      return `${relativePath}:${fileStat.size}:${Math.floor(fileStat.mtimeMs)}`
    } catch {
      return `${relativePath}:missing`
    }
  }

  private async resolveDailyNoteSettings(): Promise<DailyNoteSettings> {
    const daily = await this.readObsidianJson<Record<string, unknown>>("daily-notes.json")

    return {
      folder: asTrimmedString(daily?.folder) || this.options.folders.daily,
      format: asTrimmedString(daily?.format) || this.options.dailyNoteFormat,
      templateFile: asTrimmedString(daily?.template),
    }
  }

  private async buildNewDailyNote(settings: DailyNoteSettings, now: Date): Promise<string> {
    const title = formatObsidianDate(now, settings.format)
    const templateContent = settings.templateFile ? await this.readFile(settings.templateFile) : null

    if (templateContent && templateContent.trim().length > 0) {
      let content = templateContent
      content = ensureHeadingSection(content, "## Timeline")
      content = ensureHeadingSection(content, "## Tracking")
      return content
    }

    return [
      `# ${title}`,
      "",
      "## Timeline",
      "",
      "## Tracking",
      "- Mood:",
      "- Energy:",
      "- Focus:",
      "- Sleep:",
      "",
    ].join("\n")
  }

  private async readObsidianJson<T>(fileName: string): Promise<T | null> {
    try {
      const configPath = await this.resolveExistingPathWithinVault(join(".obsidian", fileName))
      if (!configPath) {
        return null
      }
      const file = Bun.file(configPath)
      if (!(await file.exists())) {
        return null
      }
      return (await file.json()) as T
    } catch {
      return null
    }
  }
}

function normalizeTag(tag: string): string {
  const trimmed = tag.trim().toLowerCase()
  if (trimmed.length === 0) return "#"
  return trimmed.startsWith("#") ? trimmed : `#${trimmed}`
}

function formatHourMinute(value: Date): string {
  return `${pad2(value.getHours())}:${pad2(value.getMinutes())}`
}

function formatObsidianDate(value: Date, pattern: string): string {
  const month = value.getMonth()
  const day = value.getDay()
  const fullYear = value.getFullYear()

  const monthNamesFull = [
    "January",
    "February",
    "March",
    "April",
    "May",
    "June",
    "July",
    "August",
    "September",
    "October",
    "November",
    "December",
  ]
  const monthNamesShort = monthNamesFull.map((name) => name.slice(0, 3))
  const dayNamesFull = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"]
  const dayNamesShort = dayNamesFull.map((name) => name.slice(0, 3))

  const tokenMap: Record<string, string> = {
    YYYY: String(fullYear),
    YY: String(fullYear).slice(-2),
    MMMM: monthNamesFull[month],
    MMM: monthNamesShort[month],
    MM: pad2(month + 1),
    M: String(month + 1),
    DD: pad2(value.getDate()),
    D: String(value.getDate()),
    dddd: dayNamesFull[day],
    ddd: dayNamesShort[day],
    HH: pad2(value.getHours()),
    H: String(value.getHours()),
    mm: pad2(value.getMinutes()),
    m: String(value.getMinutes()),
  }

  const tokenPattern = /(YYYY|MMMM|dddd|MMM|ddd|MM|DD|HH|mm|YY|M|D|H|m)/g
  return pattern.replace(tokenPattern, (token) => tokenMap[token] ?? token)
}

function pad2(value: number): string {
  return value < 10 ? `0${value}` : String(value)
}

function normalizeLineEndings(value: string): string {
  return value.replace(/\r\n/g, "\n")
}

function ensureHeadingSection(content: string, heading: string): string {
  const normalized = normalizeLineEndings(content)
  const lines = normalized.split("\n")
  if (lines.some((line) => line.trim() === heading)) {
    return normalized
  }

  const trimmed = normalized.trimEnd()
  if (!trimmed) {
    return `${heading}\n`
  }
  return `${trimmed}\n\n${heading}\n`
}

function sectionBounds(lines: string[], heading: string): { start: number; end: number } | null {
  const start = lines.findIndex((line) => line.trim() === heading)
  if (start === -1) return null

  let end = lines.length
  for (let i = start + 1; i < lines.length; i += 1) {
    if (lines[i].startsWith("## ")) {
      end = i
      break
    }
  }

  return { start, end }
}

function appendLineToSection(content: string, heading: string, lineToAdd: string): string {
  const normalized = normalizeLineEndings(ensureHeadingSection(content, heading))
  const lines = normalized.split("\n")
  const bounds = sectionBounds(lines, heading)
  if (!bounds) return normalized

  lines.splice(bounds.end, 0, lineToAdd)
  return lines.join("\n")
}

function upsertTrackingLine(content: string, heading: string, metric: string, value: string): string {
  const normalized = normalizeLineEndings(ensureHeadingSection(content, heading))
  const lines = normalized.split("\n")
  const bounds = sectionBounds(lines, heading)
  if (!bounds) return normalized

  const desired = `- ${metric}: ${value}`
  const target = metric.toLowerCase()

  for (let i = bounds.start + 1; i < bounds.end; i += 1) {
    const match = lines[i].match(/^\s*-\s*([^:]+):/)
    if (match && match[1].trim().toLowerCase() === target) {
      lines[i] = desired
      return lines.join("\n")
    }
  }

  lines.splice(bounds.end, 0, desired)
  return lines.join("\n")
}

function asTrimmedString(value: unknown): string | null {
  if (typeof value !== "string") return null
  const trimmed = value.trim()
  return trimmed.length > 0 ? trimmed : null
}

function normalizeNewFileLocation(value: string | null): "current" | "folder" | "root" | "unknown" {
  if (!value) return "current"
  if (value === "current" || value === "folder" || value === "root") {
    return value
  }
  return "unknown"
}

function isWithinRoot(rootPath: string, candidatePath: string): boolean {
  const rel = relative(rootPath, candidatePath)
  return rel !== ".." && !rel.startsWith(`..${sep}`)
}

function isNotFoundError(error: unknown): boolean {
  return error instanceof Error && "code" in error && (error as { code?: string }).code === "ENOENT"
}

/**
 * Create vault service with default configuration
 */
export function createVaultService(
  vaultPath: string,
  logger?: Logger,
  folders: VaultFolders = DEFAULT_VAULT_FOLDERS,
): VaultService {
  return new VaultService({
    vaultPath,
    dailyNoteFormat: "YYYY-MM-DD",
    logger,
    folders,
  })
}
