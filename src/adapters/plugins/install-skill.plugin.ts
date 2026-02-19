/**
 * Install Skill Plugin
 * Security-hardened version with proper input validation.
 */

import { tool } from "@opencode-ai/plugin"
import { join, basename, isAbsolute, posix } from "node:path"

const configuredOpenCodeDir = Bun.env.OPENCODE_CONFIG_DIR?.trim() || process.cwd()
const OPENCODE_CONFIG_DIR = isAbsolute(configuredOpenCodeDir)
  ? configuredOpenCodeDir
  : join(process.cwd(), configuredOpenCodeDir)
const SKILLS_DIR = join(OPENCODE_CONFIG_DIR, ".agents", "skills")
const configuredDataDir = Bun.env.DATA_DIR?.trim() || ".data"
const DATA_DIR = isAbsolute(configuredDataDir)
  ? configuredDataDir
  : join(process.cwd(), configuredDataDir)

// Valid GitHub repository name pattern
const GITHUB_REPO_PATTERN = /^[a-zA-Z0-9_.-]+$/
const GITHUB_REF_PATTERN = /^[a-zA-Z0-9_.\-/]+$/
const GITHUB_SUBPATH_PATTERN = /^[a-zA-Z0-9_.\-/]+$/

interface GitHubTreeUrl {
  repo: string
  ref: string
  subpath: string
}

async function exists(path: string): Promise<boolean> {
  return Bun.file(path).exists()
}

function decodePathSegment(value: string): string | null {
  try {
    return decodeURIComponent(value)
  } catch {
    return null
  }
}

function isSafeRepositorySubpath(value: string): boolean {
  if (!value || value.startsWith("/")) return false
  if (!GITHUB_SUBPATH_PATTERN.test(value)) return false

  const normalized = posix.normalize(value)
  if (normalized === "." || normalized.startsWith("../") || normalized.includes("/../")) {
    return false
  }

  const segments = normalized.split("/")
  return segments.every((segment) => segment.length > 0 && segment !== "." && segment !== "..")
}

export function parseGithubTreeUrl(input: string): GitHubTreeUrl | null {
  const source = input.trim()

  let parsedUrl: URL
  try {
    parsedUrl = new URL(source)
  } catch {
    return null
  }

  if (parsedUrl.protocol !== "https:" && parsedUrl.protocol !== "http:") {
    return null
  }
  if (parsedUrl.hostname !== "github.com") {
    return null
  }

  const segments = parsedUrl.pathname.split("/").filter(Boolean)
  if (segments.length < 5 || segments[2] !== "tree") {
    return null
  }

  const owner = segments[0]
  const repoName = segments[1]
  const refRaw = decodePathSegment(segments[3])
  const subpathParts = segments.slice(4).map(decodePathSegment)

  if (!owner || !repoName || !refRaw || subpathParts.some((part) => part === null)) {
    return null
  }

  if (!GITHUB_REPO_PATTERN.test(owner) || !GITHUB_REPO_PATTERN.test(repoName)) {
    return null
  }

  const ref = refRaw.trim()
  if (!ref || !GITHUB_REF_PATTERN.test(ref)) {
    return null
  }

  const subpath = subpathParts.join("/").trim()
  if (!isSafeRepositorySubpath(subpath)) {
    return null
  }

  return { repo: `${owner}/${repoName}`, ref, subpath: posix.normalize(subpath) }
}

function safeName(input: string): string {
  return input.replace(/[^a-zA-Z0-9._-]/g, "-")
}

interface PluginContext {
  $: (strings: TemplateStringsArray, ...values: unknown[]) => Promise<unknown>
}

interface InstallSkillArgs {
  source: string
  name?: string
}

export default async function createInstallSkillPlugin({ $ }: PluginContext) {
  return {
    tool: {
      install_skill: tool({
        description: "Install a skill into .agents/skills from a GitHub tree URL.",
        args: {
          source: tool.schema.string().describe("GitHub tree URL to the skill folder"),
          name: tool.schema.string().optional().describe("Optional target skill folder name"),
        },
        async execute(args: InstallSkillArgs) {
          await $`mkdir -p ${SKILLS_DIR}`

          const resolved = parseGithubTreeUrl(args.source)
          if (!resolved) {
            throw new Error("Unsupported source. Use a GitHub tree URL to the skill folder.")
          }

          const sourceName = basename(resolved.subpath)
          const targetName = safeName((args.name || sourceName).trim())
          const targetDir = join(SKILLS_DIR, targetName)

          if (await exists(targetDir)) {
            return `Skill '${targetName}' already exists at ${targetDir}`
          }

          // Validate target name
          if (!targetName || targetName === "." || targetName === "..") {
            throw new Error("Invalid skill name")
          }

          const tmpDir = join(
            DATA_DIR,
            `tmp-skill-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
          )
          
          try {
            await $`mkdir -p ${tmpDir}`
            
            // Sparse checkout keeps transfer focused on the requested skill folder
            await $`git -C ${tmpDir} init`
            await $`git -C ${tmpDir} remote add origin https://github.com/${resolved.repo}.git`
            await $`git -C ${tmpDir} config core.sparseCheckout true`
            await $`git -C ${tmpDir} sparse-checkout set ${resolved.subpath}`
            await $`git -C ${tmpDir} fetch --depth=1 origin ${resolved.ref}`
            await $`git -C ${tmpDir} checkout FETCH_HEAD`

            const srcDir = join(tmpDir, resolved.subpath)
            if (!(await exists(srcDir))) {
              throw new Error("Skill source folder not found after checkout.")
            }
            if (!(await exists(join(srcDir, "SKILL.md")))) {
              throw new Error("Missing SKILL.md in skill folder.")
            }

            await $`mkdir -p ${targetDir}`
            await $`cp -R ${srcDir}/. ${targetDir}`

            return `Installed skill '${targetName}' to ${targetDir}`
          } finally {
            await $`rm -rf ${tmpDir}`
          }
        },
      }),
    },
  }
}
