/**
 * OpenCode Plugins
 * 
 * Plugins integrate tools with the OpenCode SDK.
 * They use the repository pattern for all persistence.
 */

export { default as createInstallSkillPlugin } from "./install-skill.plugin"
export { default as createMemoryPlugin } from "./memory.plugin"
export { default as createChannelMessagePlugin } from "./channel-message.plugin"
export { default as createSyncthingPlugin } from "./syncthing.plugin"

export type { MemoryPluginOptions } from "./memory.plugin"
export type { ChannelMessagePluginOptions } from "./channel-message.plugin"
