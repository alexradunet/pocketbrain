import { test, expect } from "bun:test"
import { createOpencode } from "@opencode-ai/sdk"
import { findRecentModel } from "../src/config"

type SessionMessage = {
  info?: { id?: string; role?: string }
  parts?: Array<{ type?: string; text?: string }>
}

type SdkEnvelope<T> = {
  data?: T
  error?: unknown
}

function getData<T>(label: string, value: unknown): T {
  const wrapped = value as SdkEnvelope<T>
  if (wrapped && typeof wrapped === "object" && "error" in wrapped && wrapped.error) {
    throw new Error(`${label} failed: ${JSON.stringify(wrapped.error)}`)
  }
  if (!wrapped || typeof wrapped !== "object" || !("data" in wrapped)) {
    return value as T
  }
  return wrapped.data as T
}

function extractAssistantText(messages: SessionMessage[]): string | null {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const msg = messages[i]
    if (msg?.info?.role !== "assistant") continue
    const text = (msg.parts ?? [])
      .map((part) => (part?.type === "text" && typeof part.text === "string" ? part.text : ""))
      .filter(Boolean)
      .join("\n")
      .trim()
    if (text.length > 0) return text
  }
  return null
}

async function getFreePort(): Promise<number> {
  const server = Bun.serve({
    hostname: "127.0.0.1",
    port: 0,
    fetch() {
      return new Response("ok")
    },
  })
  const port = server.port
  server.stop()
  return port
}

const E2E_TIMEOUT_MS = Math.min(Math.max(Number(Bun.env.E2E_TIMEOUT_MS ?? 10_000), 1_000), 60_000)
const E2E_POLL_MS = Number(Bun.env.E2E_POLL_MS ?? 700)

test(
  "opencode e2e: assistant responds to prompt",
  async () => {
    const modelFromEnv = Bun.env.OPENCODE_MODEL?.trim()
    const model = modelFromEnv || (await findRecentModel())
    if (!model) {
      throw new Error(
        "No model available for E2E. Set OPENCODE_MODEL or set a recent model in ~/.local/state/opencode/model.json.",
      )
    }

    const port = await getFreePort()
    console.log(`[e2e] starting opencode server on 127.0.0.1:${port}, model=${model}`)

    const runtime = await createOpencode({ hostname: "127.0.0.1", port, config: { model } })

    try {
      const client = runtime.client

      const config = getData<Record<string, unknown>>("config.get", await client.config.get({} as never))
      const activeModel = typeof config.model === "string" ? config.model : null
      expect(activeModel).toBeTruthy()
      console.log(`[e2e] active model: ${activeModel}`)

      const session = getData<{ id: string }>(
        "session.create",
        await client.session.create({ body: { title: "opencode-e2e" } } as never),
      )
      expect(session.id).toBeTruthy()
      const sessionID = session.id
      console.log(`[e2e] session: ${sessionID}`)

      await client.session.prompt({
        path: { id: sessionID },
        body: {
          noReply: false,
          parts: [{ type: "text", text: "Reply with exactly: E2E_OK" }],
        },
      } as never)
      console.log("[e2e] prompt sent; polling for reply")

      const endAt = Date.now() + E2E_TIMEOUT_MS
      let answer: string | null = null
      let loops = 0

      while (Date.now() < endAt) {
        loops += 1
        const messages = getData<SessionMessage[]>(
          "session.messages",
          await client.session.messages({ path: { id: sessionID } } as never),
        )
        answer = extractAssistantText(messages)
        if (answer) break
        if (loops % 5 === 0) {
          console.log(`[e2e] waiting... messages=${messages.length}`)
        }
        await Bun.sleep(E2E_POLL_MS)
      }

      console.log(`[e2e] assistant reply: ${JSON.stringify(answer)}`)
      expect(answer).toContain("E2E_OK")
    } finally {
      runtime.server.close()
      console.log("[e2e] server closed")
    }
  },
  E2E_TIMEOUT_MS + 10_000,
)
