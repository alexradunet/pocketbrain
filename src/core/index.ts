/**
 * Core Module Exports
 */

// Core classes
export { AssistantCore, type AssistantInput, type AssistantCoreOptions } from "./assistant"
export { RuntimeProvider, type RuntimeProviderOptions } from "./runtime-provider"
export { SessionManager, type SessionManagerOptions } from "./session-manager"
export { PromptBuilder, type PromptBuilderOptions } from "./prompt-builder"
export { ChannelManager } from "./channel-manager"

// Ports (Repository Interfaces)
export type {
  MemoryRepository,
  MemoryEntry,
  ChannelRepository,
  LastChannel,
  ChannelType,
  SessionRepository,
  WhitelistRepository,
  OutboxRepository,
  OutboxMessage,
  HeartbeatRepository,
  HeartbeatRunner,
  ThrottlePort,
  ChannelAdapter,
  ChannelAuthOptions,
  WhatsAppDependencies,
  MessageHandler,
} from "./ports"

// Services
export { MessageChunker, MessageSender, type MessageChunkerOptions, type MessageSenderOptions } from "./services"
