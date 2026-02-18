import PQueue from "p-queue"

export interface QueuePolicy {
  concurrency?: number
}

export function createTaskQueue(policy: QueuePolicy = {}): PQueue {
  return new PQueue({ concurrency: policy.concurrency ?? 1 })
}
