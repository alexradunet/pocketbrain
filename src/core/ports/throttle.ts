/**
 * Throttle Port
 * Defines the contract for per-user send throttling.
 */

export interface ThrottlePort {
  throttle(userID: string): Promise<void>
}
