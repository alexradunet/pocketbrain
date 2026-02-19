import { afterEach, beforeEach, describe, expect, test } from "bun:test"
import createSyncthingPlugin from "../../../src/adapters/plugins/syncthing.plugin"

const ENV_KEYS = [
  "SYNCTHING_ENABLED",
  "SYNCTHING_BASE_URL",
  "SYNCTHING_API_KEY",
  "SYNCTHING_TIMEOUT_MS",
  "SYNCTHING_VAULT_FOLDER_ID",
  "SYNCTHING_MUTATION_TOOLS_ENABLED",
  "SYNCTHING_ALLOWED_FOLDER_IDS",
] as const

type EnvKey = (typeof ENV_KEYS)[number]
type Snapshot = Record<EnvKey, string | undefined>

function captureSnapshot(): Snapshot {
  const snapshot = {} as Snapshot
  for (const key of ENV_KEYS) {
    snapshot[key] = Bun.env[key]
  }
  return snapshot
}

function restoreSnapshot(snapshot: Snapshot): void {
  for (const key of ENV_KEYS) {
    const value = snapshot[key]
    if (value === undefined) {
      delete Bun.env[key]
      continue
    }
    Bun.env[key] = value
  }
}

describe("syncthing plugin", () => {
  let envSnapshot: Snapshot
  let fetchSnapshot: typeof fetch

  beforeEach(() => {
    envSnapshot = captureSnapshot()
    fetchSnapshot = globalThis.fetch
  })

  afterEach(() => {
    restoreSnapshot(envSnapshot)
    globalThis.fetch = fetchSnapshot
  })

  test("returns empty tool set when disabled", async () => {
    Bun.env.SYNCTHING_ENABLED = "false"
    const plugin = await createSyncthingPlugin()
    expect(Object.keys(plugin.tool)).toEqual([])
  })

  test("returns health message when API key is missing", async () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = ""

    const plugin = await createSyncthingPlugin()
    expect(plugin.tool.syncthing_health).toBeDefined()
    const result = await plugin.tool.syncthing_health!.execute({}, {} as never)
    expect(result).toContain("missing SYNCTHING_API_KEY")
  })

  test("blocks scan mutation when mutation tools disabled", async () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = "token"
    Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED = "false"
    Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS = "vault"

    const plugin = await createSyncthingPlugin()
    expect(plugin.tool.syncthing_scan_folder).toBeDefined()
    const result = await plugin.tool.syncthing_scan_folder!.execute({ folderID: "vault" }, {} as never)
    expect(result).toContain("Blocked by policy")
  })

  test("blocks scan mutation for disallowed folder", async () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_API_KEY = "token"
    Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED = "true"
    Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS = "vault"

    const plugin = await createSyncthingPlugin()
    expect(plugin.tool.syncthing_scan_folder).toBeDefined()
    const result = await plugin.tool.syncthing_scan_folder!.execute({ folderID: "other" }, {} as never)
    expect(result).toContain("not in SYNCTHING_ALLOWED_FOLDER_IDS")
  })

  test("triggers scan for allowed folder", async () => {
    Bun.env.SYNCTHING_ENABLED = "true"
    Bun.env.SYNCTHING_BASE_URL = "http://127.0.0.1:8384"
    Bun.env.SYNCTHING_API_KEY = "token"
    Bun.env.SYNCTHING_MUTATION_TOOLS_ENABLED = "true"
    Bun.env.SYNCTHING_ALLOWED_FOLDER_IDS = "vault"

    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
      const url = typeof input === "string" ? input : input.toString()
      if (url.includes("/rest/db/scan?folder=vault")) {
        expect(init?.method).toBe("POST")
        return new Response(JSON.stringify({ queued: true }), {
          status: 200,
          headers: { "content-type": "application/json" },
        })
      }

      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "content-type": "application/json" },
      })
    }) as typeof fetch

    const plugin = await createSyncthingPlugin()
    expect(plugin.tool.syncthing_scan_folder).toBeDefined()
    const result = await plugin.tool.syncthing_scan_folder!.execute({ folderID: "vault" }, {} as never)
    expect(result).toContain("Triggered Syncthing scan")
  })
})
