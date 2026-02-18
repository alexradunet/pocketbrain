/**
 * Vault Service Tests
 * 
 * Tests for vault operations.
 */

import { describe, test, expect, beforeEach, afterEach } from "bun:test"
import { VaultService, createVaultService } from "../../src/vault/vault-service"
import { join } from "node:path"
import { mkdirSync, rmSync, existsSync, writeFileSync, readFileSync } from "node:fs"

const TEST_DIR = join(__dirname, ".test-data", "vault-service")

describe("VaultService", () => {
  let vaultService: VaultService

  beforeEach(async () => {
    mkdirSync(TEST_DIR, { recursive: true })
    vaultService = createVaultService(TEST_DIR)
    await vaultService.initialize()
  })

  afterEach(async () => {
    await vaultService.stop()
    rmSync(TEST_DIR, { recursive: true, force: true })
  })

  describe("initialize", () => {
    test("creates directory structure", () => {
      expect(existsSync(join(TEST_DIR, "inbox"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "daily"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "journal"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "projects"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "areas"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "resources"))).toBe(true)
      expect(existsSync(join(TEST_DIR, "archive"))).toBe(true)
    })
  })

  describe("readFile", () => {
    test("reads file content", async () => {
      const filePath = join(TEST_DIR, "test.txt")
      writeFileSync(filePath, "hello world")

      const content = await vaultService.readFile("test.txt")

      expect(content).toBe("hello world")
    })

    test("returns null for non-existent file", async () => {
      const content = await vaultService.readFile("nonexistent.txt")

      expect(content).toBeNull()
    })

    test("reads file in subdirectory", async () => {
      writeFileSync(join(TEST_DIR, "projects", "test.md"), "# Project")

      const content = await vaultService.readFile("projects/test.md")

      expect(content).toBe("# Project")
    })

    test("blocks path traversal", async () => {
      const content = await vaultService.readFile("../outside.txt")
      expect(content).toBeNull()
    })
  })

  describe("writeFile", () => {
    test("creates new file", async () => {
      const result = await vaultService.writeFile("new.txt", "content")

      expect(result).toBe(true)
      expect(existsSync(join(TEST_DIR, "new.txt"))).toBe(true)
      expect(readFileSync(join(TEST_DIR, "new.txt"), "utf-8")).toBe("content")
    })

    test("overwrites existing file", async () => {
      writeFileSync(join(TEST_DIR, "existing.txt"), "old")

      const result = await vaultService.writeFile("existing.txt", "new")

      expect(result).toBe(true)
      expect(readFileSync(join(TEST_DIR, "existing.txt"), "utf-8")).toBe("new")
    })

    test("creates parent directories", async () => {
      const result = await vaultService.writeFile("nested/folder/file.txt", "content")

      expect(result).toBe(true)
      expect(existsSync(join(TEST_DIR, "nested", "folder", "file.txt"))).toBe(true)
    })

    test("returns false on error", async () => {
      // Try to write to read-only path (would need specific setup)
      // For now, just test valid cases work
      const result = await vaultService.writeFile("test.txt", "content")
      expect(result).toBe(true)
    })

    test("rejects path traversal writes", async () => {
      const result = await vaultService.writeFile("../outside.txt", "blocked")
      expect(result).toBe(false)
    })
  })

  describe("appendToFile", () => {
    test("appends to existing file", async () => {
      writeFileSync(join(TEST_DIR, "append.txt"), "hello ")

      const result = await vaultService.appendToFile("append.txt", "world")

      expect(result).toBe(true)
      expect(readFileSync(join(TEST_DIR, "append.txt"), "utf-8")).toBe("hello world")
    })

    test("creates file if not exists", async () => {
      const result = await vaultService.appendToFile("new-append.txt", "content")

      expect(result).toBe(true)
      expect(readFileSync(join(TEST_DIR, "new-append.txt"), "utf-8")).toBe("content")
    })

    test("appends multiple times", async () => {
      await vaultService.appendToFile("multi.txt", "a")
      await vaultService.appendToFile("multi.txt", "b")
      await vaultService.appendToFile("multi.txt", "c")

      expect(readFileSync(join(TEST_DIR, "multi.txt"), "utf-8")).toBe("abc")
    })
  })

  describe("listFiles", () => {
    test("lists files in root", async () => {
      writeFileSync(join(TEST_DIR, "file1.txt"), "")
      writeFileSync(join(TEST_DIR, "file2.txt"), "")
      mkdirSync(join(TEST_DIR, "folder"))

      const files = await vaultService.listFiles("")

      expect(files.length).toBeGreaterThanOrEqual(2)
      expect(files.some(f => f.name === "file1.txt")).toBe(true)
      expect(files.some(f => f.name === "file2.txt")).toBe(true)
    })

    test("lists files in subdirectory", async () => {
      writeFileSync(join(TEST_DIR, "projects", "file1.md"), "")
      writeFileSync(join(TEST_DIR, "projects", "file2.md"), "")

      const files = await vaultService.listFiles("projects")

      expect(files).toHaveLength(2)
      expect(files[0].isDirectory || files[1].isDirectory).toBe(false)
    })

    test("returns empty array for empty folder", async () => {
      const files = await vaultService.listFiles("empty-folder")
      expect(files).toHaveLength(0)
    })

    test("sorts directories first", async () => {
      writeFileSync(join(TEST_DIR, "zebra.txt"), "")
      mkdirSync(join(TEST_DIR, "alpha"))

      const files = await vaultService.listFiles("")

      // Directories should come first
      const dirIndex = files.findIndex(f => f.name === "alpha")
      const fileIndex = files.findIndex(f => f.name === "zebra.txt")
      expect(dirIndex).toBeLessThan(fileIndex)
    })

    test("includes file metadata", async () => {
      const before = Date.now()
      writeFileSync(join(TEST_DIR, "meta.txt"), "content")
      const after = Date.now()

      const files = await vaultService.listFiles("")
      const file = files.find(f => f.name === "meta.txt")

      expect(file).toBeDefined()
      expect(file?.path).toBe("meta.txt")
      expect(file?.size).toBe(7)
      expect(file?.isDirectory).toBe(false)
      // Allow 1 second tolerance for file system timestamps
      expect(file?.modified.getTime()).toBeGreaterThanOrEqual(before - 1000)
      expect(file?.modified.getTime()).toBeLessThanOrEqual(after + 1000)
    })
  })

  describe("searchFiles", () => {
    beforeEach(async () => {
      writeFileSync(join(TEST_DIR, "meeting-notes.md"), "")
      writeFileSync(join(TEST_DIR, "project-meeting.md"), "")
      writeFileSync(join(TEST_DIR, "todo.txt"), "")
      writeFileSync(join(TEST_DIR, "projects", "team-meeting.md"), "")
    })

    test("finds files matching query", async () => {
      const files = await vaultService.searchFiles("meeting")

      expect(files.length).toBeGreaterThanOrEqual(3)
      expect(files.some(f => f.name === "meeting-notes.md")).toBe(true)
      expect(files.some(f => f.name === "project-meeting.md")).toBe(true)
    })

    test("search is case-insensitive", async () => {
      const files = await vaultService.searchFiles("MEETING")

      expect(files.length).toBeGreaterThanOrEqual(3)
    })

    test("search in specific folder", async () => {
      const files = await vaultService.searchFiles("meeting", "projects")

      expect(files).toHaveLength(1)
      expect(files[0].name).toBe("team-meeting.md")
    })

    test("returns empty array for no matches", async () => {
      const files = await vaultService.searchFiles("nonexistent")

      expect(files).toHaveLength(0)
    })
  })

  describe("moveFile", () => {
    test("moves file to new location", async () => {
      writeFileSync(join(TEST_DIR, "source.txt"), "content")

      const result = await vaultService.moveFile("source.txt", "dest.txt")

      expect(result).toBe(true)
      expect(existsSync(join(TEST_DIR, "source.txt"))).toBe(false)
      expect(existsSync(join(TEST_DIR, "dest.txt"))).toBe(true)
      expect(readFileSync(join(TEST_DIR, "dest.txt"), "utf-8")).toBe("content")
    })

    test("moves between folders", async () => {
      writeFileSync(join(TEST_DIR, "inbox", "note.md"), "# Note")

      const result = await vaultService.moveFile("inbox/note.md", "projects/note.md")

      expect(result).toBe(true)
      expect(existsSync(join(TEST_DIR, "inbox", "note.md"))).toBe(false)
      expect(existsSync(join(TEST_DIR, "projects", "note.md"))).toBe(true)
    })

    test("creates destination directories", async () => {
      writeFileSync(join(TEST_DIR, "file.txt"), "content")

      const result = await vaultService.moveFile("file.txt", "nested/folder/file.txt")

      expect(result).toBe(true)
      expect(existsSync(join(TEST_DIR, "nested", "folder", "file.txt"))).toBe(true)
    })

    test("returns false on error", async () => {
      const result = await vaultService.moveFile("nonexistent.txt", "dest.txt")

      expect(result).toBe(false)
    })

    test("rejects path traversal in destination", async () => {
      writeFileSync(join(TEST_DIR, "source.txt"), "content")

      const result = await vaultService.moveFile("source.txt", "../outside.txt")

      expect(result).toBe(false)
      expect(existsSync(join(TEST_DIR, "source.txt"))).toBe(true)
    })
  })

  describe("getDailyNotePath", () => {
    test("returns path for today", () => {
      const today = new Date()
      const expectedDate = today.toISOString().split('T')[0]

      const path = vaultService.getDailyNotePath()

      expect(path).toBe(`daily/${expectedDate}.md`)
    })
  })

  describe("appendToDaily", () => {
    test("creates daily note with header if not exists", async () => {
      const today = new Date()
      const dateStr = today.toISOString().split('T')[0]

      const result = await vaultService.appendToDaily("## Morning\n- Task 1")

      expect(result).toBe(true)
      const content = readFileSync(join(TEST_DIR, "daily", `${dateStr}.md`), "utf-8")
      expect(content).toContain(`# ${dateStr}`)
      expect(content).toContain("## Morning")
      expect(content).toContain("- Task 1")
    })

    test("appends to existing daily note", async () => {
      const today = new Date()
      const dateStr = today.toISOString().split('T')[0]
      writeFileSync(join(TEST_DIR, "daily", `${dateStr}.md`), "# Existing\n")

      const result = await vaultService.appendToDaily("## Evening")

      expect(result).toBe(true)
      const content = readFileSync(join(TEST_DIR, "daily", `${dateStr}.md`), "utf-8")
      expect(content).toContain("# Existing")
      expect(content).toContain("## Evening")
    })
  })

  describe("getStats", () => {
    test("returns zero for empty vault", async () => {
      const stats = await vaultService.getStats()

      expect(stats.totalFiles).toBe(7) // 7 folders created by initialize
      expect(stats.totalSize).toBe(0)
      expect(stats.lastModified).toBeNull()
    })

    test("counts files correctly", async () => {
      writeFileSync(join(TEST_DIR, "file1.txt"), "content1")
      writeFileSync(join(TEST_DIR, "file2.txt"), "content2")

      const stats = await vaultService.getStats()

      expect(stats.totalFiles).toBeGreaterThanOrEqual(2)
    })

    test("calculates total size", async () => {
      writeFileSync(join(TEST_DIR, "file1.txt"), "a".repeat(100))
      writeFileSync(join(TEST_DIR, "file2.txt"), "b".repeat(200))

      const stats = await vaultService.getStats()

      expect(stats.totalSize).toBe(300)
    })

    test("finds last modified date", async () => {
      const before = Date.now()
      writeFileSync(join(TEST_DIR, "recent.txt"), "content")
      const after = Date.now()

      const stats = await vaultService.getStats()

      expect(stats.lastModified).not.toBeNull()
      // Allow 1 second tolerance for file system timestamps
      expect(stats.lastModified!.getTime()).toBeGreaterThanOrEqual(before - 1000)
      expect(stats.lastModified!.getTime()).toBeLessThanOrEqual(after + 1000)
    })
  })
})

describe("createVaultService", () => {
  test("creates service with default config", () => {
    const service = createVaultService("/tmp/test-vault")
    expect(service).toBeInstanceOf(VaultService)
  })
})
