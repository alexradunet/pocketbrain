/**
 * E2E test harness for PocketBrain.
 *
 * Talks to the MockChannel HTTP API running inside the pocketbrain-e2e container.
 * Set POCKETBRAIN_MOCK_URL to point at the container (default: http://localhost:3456).
 */

const BASE_URL = (process.env.POCKETBRAIN_MOCK_URL || 'http://localhost:3456').replace(/\/$/, '');

export interface OutboxMessage {
  jid: string;
  text: string;
  timestamp: string;
}

/** Inject a user message into PocketBrain via the mock channel. */
export async function injectMessage(
  content: string,
  opts: { sender?: string; senderName?: string; jid?: string } = {},
): Promise<{ id: string }> {
  const resp = await fetch(`${BASE_URL}/inbox`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, ...opts }),
  });
  if (!resp.ok) {
    throw new Error(`injectMessage failed: ${resp.status} ${await resp.text()}`);
  }
  return resp.json() as Promise<{ id: string }>;
}

/** Get all outbound messages PocketBrain has sent so far. */
export async function getOutbox(): Promise<OutboxMessage[]> {
  const resp = await fetch(`${BASE_URL}/outbox`);
  if (!resp.ok) throw new Error(`getOutbox failed: ${resp.status}`);
  const { messages } = (await resp.json()) as { messages: OutboxMessage[] };
  return messages;
}

/** Clear the outbox between tests. */
export async function clearOutbox(): Promise<void> {
  const resp = await fetch(`${BASE_URL}/outbox`, { method: 'DELETE' });
  if (!resp.ok) throw new Error(`clearOutbox failed: ${resp.status}`);
}

/**
 * Poll the outbox until at least one message appears or timeout elapses.
 * Returns all outbox messages accumulated up to that point.
 */
export async function waitForResponse(
  timeoutMs = 90000,
  pollMs = 1500,
): Promise<OutboxMessage[]> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const msgs = await getOutbox();
    if (msgs.length > 0) return msgs;
    await Bun.sleep(pollMs);
  }
  throw new Error(`waitForResponse: no agent response within ${timeoutMs}ms`);
}

/**
 * Use Claude Haiku (via Anthropic API) to judge whether a response satisfies
 * a natural-language criterion. This is the "AI agent verifying AI agent" layer.
 *
 * Requires ANTHROPIC_API_KEY (or OPENCODE_API_KEY as fallback) in the environment.
 * Returns true if the criterion is met, false otherwise.
 */
export async function llmAssert(response: string, criterion: string): Promise<boolean> {
  const apiKey = process.env.ANTHROPIC_API_KEY || process.env.OPENCODE_API_KEY;
  if (!apiKey) {
    throw new Error(
      'llmAssert requires ANTHROPIC_API_KEY or OPENCODE_API_KEY. ' +
        'Set it in the environment or skip llmAssert for non-AI assertions.',
    );
  }

  const resp = await fetch('https://api.anthropic.com/v1/messages', {
    method: 'POST',
    headers: {
      'x-api-key': apiKey,
      'anthropic-version': '2023-06-01',
      'content-type': 'application/json',
    },
    body: JSON.stringify({
      model: 'claude-haiku-4-5-20251001',
      max_tokens: 16,
      messages: [
        {
          role: 'user',
          content:
            `Criterion: "${criterion}"\n\n` +
            `Response to evaluate:\n${response}\n\n` +
            'Does the response satisfy the criterion? Answer only YES or NO.',
        },
      ],
    }),
  });

  if (!resp.ok) {
    const body = await resp.text();
    throw new Error(`llmAssert API error: ${resp.status} — ${body}`);
  }

  const data = (await resp.json()) as {
    content: Array<{ type: string; text: string }>;
  };
  const text = data.content.find((c) => c.type === 'text')?.text?.trim().toUpperCase() ?? '';
  return text.startsWith('YES');
}

/**
 * Wait for the MockChannel health endpoint to be reachable.
 * Useful in test suites that run before the depends_on health check settles.
 */
export async function waitForReady(timeoutMs = 30000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`${BASE_URL}/health`, { signal: AbortSignal.timeout(2000) });
      if (resp.ok) return;
    } catch {
      /* not ready yet */
    }
    await Bun.sleep(500);
  }
  throw new Error(`waitForReady: PocketBrain mock channel not reachable at ${BASE_URL}`);
}
