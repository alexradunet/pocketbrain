import { describe, expect, test } from "bun:test"
import { normalizeWikiLinkTarget, parseWikiLinks } from "../../src/lib/markdown-links"

describe("markdown-links", () => {
  test("parses plain wiki links", () => {
    const links = parseWikiLinks("Read [[Project Plan]] and [[Inbox]].")

    expect(links).toHaveLength(2)
    expect(links[0]?.target).toBe("Project Plan")
    expect(links[0]?.alias).toBeUndefined()
    expect(links[1]?.target).toBe("Inbox")
  })

  test("parses aliased wiki links", () => {
    const links = parseWikiLinks("See [[Project Plan|Plan]] now")

    expect(links).toHaveLength(1)
    expect(links[0]?.target).toBe("Project Plan")
    expect(links[0]?.alias).toBe("Plan")
  })

  test("keeps duplicate links", () => {
    const links = parseWikiLinks("[[Daily]] then [[Daily]]")

    expect(links).toHaveLength(2)
    expect(links[0]?.normalizedTarget).toBe("daily")
    expect(links[1]?.normalizedTarget).toBe("daily")
  })

  test("ignores malformed links", () => {
    const links = parseWikiLinks("broken [[open and []] and [[ ]] and [[]]")

    expect(links).toHaveLength(0)
  })

  test("normalizes targets for lookup", () => {
    expect(normalizeWikiLinkTarget("  My Note  ")).toBe("my note")
  })
})
