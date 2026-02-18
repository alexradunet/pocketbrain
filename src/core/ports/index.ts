/**
 * Core Ports (Repository Interfaces)
 * 
 * These interfaces define the contracts that the core business logic
 * depends on. They follow the Dependency Inversion Principle from SOLID.
 */

export type { MemoryRepository, MemoryEntry } from "./memory-repository"
export type { ChannelRepository, LastChannel, ChannelType } from "./channel-repository"
export type { SessionRepository } from "./session-repository"
export type { WhitelistRepository } from "./whitelist-repository"
export type { OutboxRepository, OutboxMessage } from "./outbox-repository"
export type { HeartbeatRepository } from "./heartbeat-repository"
export type { ChannelAdapter, ChannelAuthOptions, WhatsAppDependencies, MessageHandler } from "./channel-adapter"
