import type { Logger } from "pino"

export interface TaildriveBootstrapOptions {
  enabled: boolean
  shareName: string
  vaultPath: string
  autoShare: boolean
  logger: Logger
}

interface TaildriveShare {
  name: string
  path: string
}

async function listShares(logger: Logger): Promise<TaildriveShare[]> {
  try {
    const proc = Bun.spawn(["tailscale", "drive", "list"], {
      stdout: "pipe",
      stderr: "pipe",
    })
    const code = await proc.exited
    if (code !== 0) {
      const stderr = await new Response(proc.stderr).text()
      logger.warn({ code, stderr: stderr.trim() }, "tailscale drive list failed")
      return []
    }

    const stdout = await new Response(proc.stdout).text()
    const shares: TaildriveShare[] = []

    for (const line of stdout.trim().split("\n")) {
      if (!line.trim()) continue
      // Output format: "name  /path/to/dir"
      const match = line.match(/^(\S+)\s+(.+)$/)
      if (match) {
        shares.push({ name: match[1], path: match[2].trim() })
      }
    }

    return shares
  } catch (error) {
    logger.warn({ error }, "failed to run tailscale drive list")
    return []
  }
}

async function createShare(shareName: string, vaultPath: string, logger: Logger): Promise<boolean> {
  try {
    const proc = Bun.spawn(["tailscale", "drive", "share", shareName, vaultPath], {
      stdout: "pipe",
      stderr: "pipe",
    })
    const code = await proc.exited
    if (code !== 0) {
      const stderr = await new Response(proc.stderr).text()
      logger.warn({ code, stderr: stderr.trim() }, "tailscale drive share failed")
      return false
    }

    logger.info({ shareName, vaultPath }, "created taildrive share")
    return true
  } catch (error) {
    logger.warn({ error }, "failed to run tailscale drive share")
    return false
  }
}

export async function ensureTaildriveShare(options: TaildriveBootstrapOptions): Promise<void> {
  if (!options.enabled) return

  const shares = await listShares(options.logger)
  const existing = shares.find((s) => s.name === options.shareName)

  if (existing) {
    options.logger.info({ shareName: existing.name, path: existing.path }, "taildrive share exists")
    return
  }

  options.logger.warn({ shareName: options.shareName }, "taildrive share not found")

  if (!options.autoShare) {
    options.logger.warn("taildrive auto-share disabled; skipping share creation")
    return
  }

  const created = await createShare(options.shareName, options.vaultPath, options.logger)
  if (created) {
    options.logger.info("taildrive share created successfully")
  } else {
    options.logger.warn("failed to create taildrive share automatically")
  }
}
