import { describe, expect, test, vi } from "bun:test"
import createChannelMessagePlugin from "../../../src/adapters/plugins/channel-message.plugin"
import type { ChannelRepository } from "../../../src/core/ports/channel-repository"
import type { OutboxRepository } from "../../../src/core/ports/outbox-repository"

describe("channel-message plugin", () => {
  test("queues proactive messages for any last-used channel", async () => {
    const channelRepository: ChannelRepository = {
      saveLastChannel: () => {},
      getLastChannel: () => ({ channel: "telegram", userID: "user-123" }),
    }
    const outboxRepository: OutboxRepository = {
      enqueue: vi.fn(),
      listPending: () => [],
      acknowledge: () => {},
      markRetry: () => {},
    }

    const plugin = await createChannelMessagePlugin({ channelRepository, outboxRepository })
    const result = await plugin.tool.send_channel_message!.execute({ text: "  hello there  " }, {} as never)

    expect(outboxRepository.enqueue).toHaveBeenCalledWith("telegram", "user-123", "hello there")
    expect(result).toBe("Queued message for telegram:user-123")
  })

  test("rejects invalid last-used channel payload", async () => {
    const channelRepository: ChannelRepository = {
      saveLastChannel: () => {},
      getLastChannel: () => ({ channel: "", userID: "" }),
    }
    const outboxRepository: OutboxRepository = {
      enqueue: vi.fn(),
      listPending: () => [],
      acknowledge: () => {},
      markRetry: () => {},
    }

    const plugin = await createChannelMessagePlugin({ channelRepository, outboxRepository })
    const result = await plugin.tool.send_channel_message!.execute({ text: "ping" }, {} as never)

    expect(outboxRepository.enqueue).not.toHaveBeenCalled()
    expect(result).toBe("Last-used channel data is invalid.")
  })
})
