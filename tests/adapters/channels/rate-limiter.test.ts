import { describe, test, expect, beforeEach } from "bun:test"
import { RateLimiter } from "../../../src/adapters/channels/rate-limiter"

describe("RateLimiter", () => {
  let limiter: RateLimiter

  beforeEach(() => {
    limiter = new RateLimiter({ minIntervalMs: 100 })
  })

  test("first call should not wait", async () => {
    const start = Date.now()
    await limiter.throttle("user1")
    const elapsed = Date.now() - start
    expect(elapsed).toBeLessThan(50)
  })

  test("second call should wait at least minIntervalMs", async () => {
    await limiter.throttle("user1")
    const start = Date.now()
    await limiter.throttle("user1")
    const elapsed = Date.now() - start
    expect(elapsed).toBeGreaterThanOrEqual(90)
  })

  test("different users should not wait", async () => {
    await limiter.throttle("user1")
    const start = Date.now()
    await limiter.throttle("user2")
    const elapsed = Date.now() - start
    expect(elapsed).toBeLessThan(50)
  })

  test("reset should clear user state", async () => {
    await limiter.throttle("user1")
    limiter.reset("user1")
    const start = Date.now()
    await limiter.throttle("user1")
    const elapsed = Date.now() - start
    expect(elapsed).toBeLessThan(50)
  })

  test("resetAll should clear all state", async () => {
    await limiter.throttle("user1")
    await limiter.throttle("user2")
    limiter.resetAll()
    const start = Date.now()
    await limiter.throttle("user1")
    const elapsed = Date.now() - start
    expect(elapsed).toBeLessThan(50)
  })

  test("getLastSendTime returns undefined for unknown user", () => {
    const lastTime = limiter.getLastSendTime("unknown")
    expect(lastTime).toBeUndefined()
  })

  test("getLastSendTime returns time after throttle", async () => {
    await limiter.throttle("user1")
    const lastTime = limiter.getLastSendTime("user1")
    expect(lastTime).toBeDefined()
    expect(lastTime).toBeLessThanOrEqual(Date.now())
  })
})
