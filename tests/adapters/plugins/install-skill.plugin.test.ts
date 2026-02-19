import { describe, expect, test } from "bun:test"
import { parseGithubTreeUrl } from "../../../src/adapters/plugins/install-skill.plugin"

describe("install-skill plugin URL parsing", () => {
  test("parses a valid github tree url", () => {
    const parsed = parseGithubTreeUrl("https://github.com/org/repo/tree/main/skills/my-skill")

    expect(parsed).toEqual({
      repo: "org/repo",
      ref: "main",
      subpath: "skills/my-skill",
    })
  })

  test("decodes encoded branch refs with slashes", () => {
    const parsed = parseGithubTreeUrl("https://github.com/org/repo/tree/feature%2Falpha/skills/my-skill")

    expect(parsed).toEqual({
      repo: "org/repo",
      ref: "feature/alpha",
      subpath: "skills/my-skill",
    })
  })

  test("rejects traversal in skill subpath", () => {
    const parsed = parseGithubTreeUrl("https://github.com/org/repo/tree/main/skills/%2E%2E/%2E%2E/etc")

    expect(parsed).toBeNull()
  })

  test("rejects non github hosts", () => {
    const parsed = parseGithubTreeUrl("https://example.com/org/repo/tree/main/skills/my-skill")

    expect(parsed).toBeNull()
  })
})
