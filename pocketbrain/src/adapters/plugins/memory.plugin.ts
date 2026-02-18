/**
 * Memory Plugin
 * Uses repository pattern for memory operations.
 */

import { tool } from "@opencode-ai/plugin"
import type { MemoryRepository } from "../../core/ports/memory-repository"

export interface MemoryPluginOptions {
  memoryRepository: MemoryRepository
}

interface SaveMemoryArgs {
  fact: string
}

interface DeleteMemoryArgs {
  id: number
}

export default async function createMemoryPlugin(options: MemoryPluginOptions) {
  const { memoryRepository } = options

  return {
    tool: {
      save_memory: tool({
        description: "Append one durable user fact to memory",
        args: {
          fact: tool.schema.string().describe("A short, stable user fact worth remembering"),
        },
        async execute(args: SaveMemoryArgs) {
          const fact = args.fact.trim()
          if (!fact) return "Skipped: empty memory fact."
          const inserted = memoryRepository.append(fact)
          return inserted ? "Saved durable memory." : "Skipped: similar fact already exists."
        },
      }),
      
      delete_memory: tool({
        description: "Delete a memory fact by ID",
        args: {
          id: tool.schema.number().describe("The memory ID to delete"),
        },
        async execute(args: DeleteMemoryArgs) {
          const deleted = memoryRepository.delete(args.id)
          return deleted ? `Memory ${args.id} deleted.` : `Memory ${args.id} not found.`
        },
      }),
    },
  }
}
