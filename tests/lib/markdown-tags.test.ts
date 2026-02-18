import { describe, expect, test } from "bun:test"
import { buildTagIndex, extractMarkdownTags } from "../../src/lib/markdown-tags"

describe("markdown-tags", () => {
  test("extracts simple and nested tags", () => {
    const tags = extractMarkdownTags("Work on #pkm and #life/os this week")

    expect(tags).toEqual(["#life/os", "#pkm"])
  })

  test("normalizes tags to lowercase", () => {
    const tags = extractMarkdownTags("#PKM #Life/OS")

    expect(tags).toEqual(["#life/os", "#pkm"])
  })

  test("does not treat headings as tags", () => {
    const tags = extractMarkdownTags("# Daily Note\nContent without tags")

    expect(tags).toEqual([])
  })

  test("builds tag index from multiple notes", () => {
    const index = buildTagIndex([
      { path: "daily/2026-02-18.md", content: "Tasks for #pkm" },
      { path: "projects/life-os.md", content: "Roadmap #pkm #life/os" },
    ])

    expect(index.get("#pkm")).toEqual(["daily/2026-02-18.md", "projects/life-os.md"])
    expect(index.get("#life/os")).toEqual(["projects/life-os.md"])
  })
})
