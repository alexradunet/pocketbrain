import pRetry, { type FailedAttemptError, type Options as PRetryOptions } from "p-retry"

export interface RetryPolicy {
  retries: number
  minTimeoutMs: number
  maxTimeoutMs: number
  factor?: number
}

export interface RetryHooks {
  onFailedAttempt?: (error: FailedAttemptError) => void
}

export interface BuiltRetryOptions {
  retries: number
  minTimeout: number
  maxTimeout: number
  factor: number
  randomize: boolean
  onFailedAttempt?: (error: FailedAttemptError) => void
}

export async function retryWithBackoff<T>(
  operation: () => Promise<T>,
  policy: RetryPolicy,
  hooks: RetryHooks = {},
): Promise<T> {
  const options = buildRetryOptions(policy, hooks)
  return pRetry(operation, options as PRetryOptions)
}

export function buildRetryOptions(policy: RetryPolicy, hooks: RetryHooks = {}): BuiltRetryOptions {
  const options: BuiltRetryOptions = {
    retries: policy.retries,
    minTimeout: policy.minTimeoutMs,
    maxTimeout: policy.maxTimeoutMs,
    factor: policy.factor ?? 2,
    randomize: false,
    onFailedAttempt: hooks.onFailedAttempt,
  }

  return options
}
