/**
 * Message Chunker Service
 * Splits long messages into chunks suitable for delivery.
 * Follows Single Responsibility Principle.
 */

export interface MessageChunkerOptions {
  /** Maximum length of each chunk */
  maxLength: number
  /** Prefer splitting at newlines if they exist within threshold */
  newlineThreshold?: number
}

export class MessageChunker {
  private readonly options: MessageChunkerOptions

  constructor(options: MessageChunkerOptions) {
    if (!Number.isFinite(options.maxLength) || options.maxLength < 1) {
      throw new Error("MessageChunker maxLength must be at least 1")
    }

    const newlineThreshold = options.newlineThreshold ?? 0.5
    if (!Number.isFinite(newlineThreshold) || newlineThreshold < 0 || newlineThreshold > 1) {
      throw new Error("MessageChunker newlineThreshold must be between 0 and 1")
    }

    this.options = {
      newlineThreshold,
      ...options,
    }
  }

  /**
   * Split text into chunks
   */
  split(text: string): string[] {
    const input = text.trim()
    if (input.length <= this.options.maxLength) {
      return input.length > 0 ? [input] : []
    }

    const chunks: string[] = []
    let rest = input
    const threshold = this.options.maxLength * (this.options.newlineThreshold ?? 0.5)

    while (rest.length > this.options.maxLength) {
      let idx = rest.lastIndexOf("\n", this.options.maxLength)
      if (idx < threshold) {
        idx = rest.lastIndexOf(" ", this.options.maxLength)
      }
      if (idx <= 0) {
        idx = this.options.maxLength
      }
      chunks.push(rest.slice(0, idx).trim())
      rest = rest.slice(idx).trim()
    }

    if (rest.length > 0) {
      chunks.push(rest)
    }

    return chunks
  }
}
