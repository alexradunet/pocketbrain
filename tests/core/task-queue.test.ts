import { describe, test, expect } from "bun:test"
import { createTaskQueue } from "../../src/lib/task-queue"

describe("createTaskQueue", () => {
  test("runs tasks serially by default", async () => {
    const queue = createTaskQueue()
    const execution: string[] = []

    void queue.add(async () => {
      execution.push("start-1")
      await Bun.sleep(10)
      execution.push("end-1")
    })

    void queue.add(async () => {
      execution.push("start-2")
      execution.push("end-2")
    })

    await queue.onIdle()

    expect(execution).toEqual(["start-1", "end-1", "start-2", "end-2"])
  })
})
