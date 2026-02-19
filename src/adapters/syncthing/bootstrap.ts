import type { Logger } from "pino"
import { createInterface } from "node:readline/promises"
import { stdin, stdout } from "node:process"
import { SyncthingClient } from "./client"

export interface SyncthingBootstrapOptions {
  enabled: boolean
  baseUrl: string
  apiKey: string | undefined
  timeoutMs: number
  autoStart: boolean
  logger: Logger
}

async function isReachable(client: SyncthingClient): Promise<boolean> {
  try {
    const result = await client.ping()
    return result.ok === true
  } catch {
    return false
  }
}

async function maybeAskForStart(): Promise<boolean> {
  if (!stdin.isTTY || !stdout.isTTY) return false

  const rl = createInterface({ input: stdin, output: stdout })
  try {
    const answer = await rl.question("Syncthing is not reachable. Start Syncthing service now? [Y/n] ")
    const normalized = answer.trim().toLowerCase()
    return normalized === "" || normalized === "y" || normalized === "yes"
  } finally {
    rl.close()
  }
}

async function tryStartSyncthingService(logger: Logger): Promise<boolean> {
  const commands = [
    ["systemctl", "--user", "start", "syncthing"],
    ["systemctl", "start", "syncthing"],
  ]

  for (const command of commands) {
    try {
      const proc = Bun.spawn(command, { stdout: "ignore", stderr: "ignore" })
      const code = await proc.exited
      if (code === 0) {
        logger.info({ command: command.join(" ") }, "started syncthing service")
        return true
      }
    } catch {
      // ignore and try the next strategy
    }
  }

  return false
}

export async function ensureSyncthingAvailable(options: SyncthingBootstrapOptions): Promise<void> {
  if (!options.enabled) return

  if (!options.apiKey) {
    options.logger.warn("syncthing enabled but API key is missing")
    return
  }

  const client = new SyncthingClient({
    baseUrl: options.baseUrl,
    apiKey: options.apiKey,
    timeoutMs: options.timeoutMs,
  })

  if (await isReachable(client)) {
    options.logger.info("syncthing API reachable")
    return
  }

  options.logger.warn("syncthing API not reachable")

  let shouldStart = options.autoStart
  if (!shouldStart) {
    shouldStart = await maybeAskForStart()
  }

  if (!shouldStart) {
    options.logger.warn("syncthing auto-start disabled; skipping start attempt")
    return
  }

  const started = await tryStartSyncthingService(options.logger)
  if (!started) {
    options.logger.warn("failed to start syncthing service automatically")
    return
  }

  await Bun.sleep(1000)
  if (await isReachable(client)) {
    options.logger.info("syncthing API reachable after start attempt")
  } else {
    options.logger.warn("syncthing service started but API still unreachable")
  }
}
