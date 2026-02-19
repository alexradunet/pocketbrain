import { describe, test, expect } from "bun:test"
import { retryWithBackoff, buildRetryOptions } from "../../src/lib/retry"

describe("retryWithBackoff", () => {
  test("retries until operation succeeds", async () => {
    let attempts = 0

    const result = await retryWithBackoff(
      async () => {
        attempts += 1
        if (attempts < 3) {
          throw new Error("transient failure")
        }
        return "ok"
      },
      {
        retries: 3,
        minTimeoutMs: 1,
        maxTimeoutMs: 10,
      },
    )

    expect(result).toBe("ok")
    expect(attempts).toBe(3)
  })

  test("throws when retries are exhausted", async () => {
    let attempts = 0

    await expect(
      retryWithBackoff(
        async () => {
          attempts += 1
          throw new Error("still failing")
        },
        {
          retries: 2,
          minTimeoutMs: 1,
          maxTimeoutMs: 10,
        },
      ),
    ).rejects.toThrow()

    expect(attempts).toBe(3)
  })

  test("builds retry options with deterministic backoff defaults", () => {
    const options = buildRetryOptions({
      retries: 3,
      minTimeoutMs: 100,
      maxTimeoutMs: 1000,
    })

    expect(options.retries).toBe(3)
    expect(options.minTimeout).toBe(100)
    expect(options.maxTimeout).toBe(1000)
    expect(options.factor).toBe(2)
    expect(options.randomize).toBe(false)
  })
})
