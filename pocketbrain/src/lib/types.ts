/**
 * Type utilities and guards for OpenCode SDK responses
 */

import type { Part, Message } from "@opencode-ai/sdk"

export type { Part, Message }

// OpenCode SDK response types
export interface PromptResult {
  data?: {
    parts?: Part[]
  }
}

export interface MessagesResult {
  data?: Array<{
    info: Message
    parts: Part[]
  }>
}

export interface CreateResult {
  data?: {
    id?: string
  }
}

// SDK request types to avoid `as never` casts
export interface SessionPromptRequest {
  path: { id: string }
  body: {
    noReply: boolean
    system?: string
    parts: Array<{ type: "text"; text: string }>
    model?: { providerID: string; modelID: string }
  }
}

export interface SessionMessagesRequest {
  path: { id: string }
}

export interface SessionCreateRequest {
  body: {
    title: string
  }
}

// Type guards
export function isPromptResult(value: unknown): value is PromptResult {
  if (typeof value !== "object" || value === null) return false
  const result = value as Record<string, unknown>
  if (!("data" in result)) return false
  if (result.data === undefined || result.data === null) return true
  return typeof result.data === "object"
}

export function isMessagesResult(value: unknown): value is MessagesResult {
  if (typeof value !== "object" || value === null) return false
  const result = value as Record<string, unknown>
  if (!("data" in result)) return false
  return Array.isArray(result.data)
}

export function isCreateResult(value: unknown): value is CreateResult {
  if (typeof value !== "object" || value === null) return false
  const result = value as Record<string, unknown>
  if (!("data" in result)) return false
  if (result.data === undefined || result.data === null) return true
  return typeof result.data === "object"
}

// Safe text extraction
export function extractTextFromParts(parts: unknown[]): string {
  if (!Array.isArray(parts)) return ""
  
  return parts
    .filter((p): p is { type: string; text: string } => {
      return (
        typeof p === "object" &&
        p !== null &&
        "type" in p &&
        (p as Record<string, unknown>).type === "text" &&
        "text" in p &&
        typeof (p as Record<string, unknown>).text === "string"
      )
    })
    .map((p) => p.text)
    .join("\n")
    .trim()
}
