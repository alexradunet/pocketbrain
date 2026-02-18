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

export async function retryWithBackoff<T>(
  operation: () => Promise<T>,
  policy: RetryPolicy,
  hooks: RetryHooks = {},
): Promise<T> {
  const options: PRetryOptions = {
    retries: policy.retries,
    minTimeout: policy.minTimeoutMs,
    maxTimeout: policy.maxTimeoutMs,
    factor: policy.factor ?? 2,
    randomize: true,
    onFailedAttempt: hooks.onFailedAttempt,
  }

  return pRetry(operation, options)
}
