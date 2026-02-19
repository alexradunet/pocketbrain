import { describe, expect, test } from "bun:test"
import { expandDirectWhitelistIDs } from "../../../../src/adapters/channels/whatsapp/adapter"

describe("expandDirectWhitelistIDs", () => {
  test("returns both direct jid variants for numeric s.whatsapp.net IDs", () => {
    expect(expandDirectWhitelistIDs("15551234567@s.whatsapp.net")).toEqual([
      "15551234567@s.whatsapp.net",
      "15551234567@lid",
    ])
  })

  test("returns both direct jid variants for numeric lid IDs", () => {
    expect(expandDirectWhitelistIDs("15551234567@lid")).toEqual([
      "15551234567@s.whatsapp.net",
      "15551234567@lid",
    ])
  })

  test("returns empty list for invalid direct IDs", () => {
    expect(expandDirectWhitelistIDs("abc123@lid")).toEqual([])
  })
})
