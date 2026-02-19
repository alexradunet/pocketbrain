/**
 * Channel Message Plugin
 * Uses repository pattern for channel operations.
 */

import { tool } from "@opencode-ai/plugin"
import type { ChannelRepository } from "../../core/ports/channel-repository"
import type { OutboxRepository } from "../../core/ports/outbox-repository"

export interface ChannelMessagePluginOptions {
  channelRepository: ChannelRepository
  outboxRepository: OutboxRepository
}

interface SendChannelMessageArgs {
  text: string
}

export default async function createChannelMessagePlugin(options: ChannelMessagePluginOptions) {
  const { channelRepository, outboxRepository } = options

  return {
    tool: {
      send_channel_message: tool({
        description: "Queue a proactive message to the last used chat channel/user.",
        args: {
          text: tool.schema.string().describe("Plain-text message to send"),
        },
        async execute(args: SendChannelMessageArgs) {
          const text = args.text.trim()
          if (!text) return "Skipped: empty message."

          const target = channelRepository.getLastChannel()
          if (!target) return "No last-used channel found yet."

          if (
            typeof target.channel !== "string" ||
            target.channel.trim().length === 0 ||
            typeof target.userID !== "string" ||
            target.userID.trim().length === 0
          ) {
            return "Last-used channel data is invalid."
          }

          const channel = target.channel.trim()
          const userID = target.userID.trim()
          outboxRepository.enqueue(channel, userID, text)
          return `Queued message for ${channel}:${userID}`
        },
      }),
    },
  }
}
