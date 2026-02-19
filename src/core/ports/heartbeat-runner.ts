/**
 * Heartbeat Runner Port
 * Defines the contract for executing heartbeat tasks.
 */

export interface HeartbeatRunner {
  runHeartbeatTasks(): Promise<string>
}
