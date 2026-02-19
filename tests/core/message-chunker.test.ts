import { describe, expect, test } from "bun:test"
import { MessageChunker } from "../../src/core/services/message-chunker"

describe("MessageChunker", () => {
  test("throws when maxLength is zero", () => {
    expect(() => new MessageChunker({ maxLength: 0 })).toThrow()
  })

  test("throws when maxLength is negative", () => {
    expect(() => new MessageChunker({ maxLength: -1 })).toThrow()
  })
})
