/**
 * PocketBrain Infrastructure E2E Tests
 *
 * Validates routing, session handling, and outbox delivery without asserting on
 * AI response quality. These tests pass with any LLM, including small local models
 * like qwen2.5:3b via Ollama — no ANTHROPIC_API_KEY required.
 *
 * Run via: bun run e2e:local
 * Or directly against a running stack: POCKETBRAIN_MOCK_URL=http://localhost:3456 bun test src/e2e/infra.test.ts
 */
import { afterEach, beforeAll, describe, expect, it } from 'bun:test';

import { clearOutbox, injectMessage, waitForReady, waitForResponse } from './harness.js';

const TEST_JID = 'e2e-test@mock.test';

// Local LLMs are slower — use a generous timeout.
const TEST_TIMEOUT_MS = 180_000;
const AGENT_WAIT_MS = 150_000;

beforeAll(async () => {
  await waitForReady(30_000);
  await clearOutbox();
});

afterEach(async () => {
  await clearOutbox();
  await Bun.sleep(1000);
});

// ---------------------------------------------------------------------------
// Routing: message reaches agent and response is delivered to correct JID
// ---------------------------------------------------------------------------

describe('Infra — message routing', () => {
  it('delivers a response to the correct JID', async () => {
    await injectMessage('Say hello.', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    expect(msgs[0].jid).toBe(TEST_JID);
  }, TEST_TIMEOUT_MS);

  it('response contains non-empty text', async () => {
    await injectMessage('Reply with any word.', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    const fullText = msgs.map((m) => m.text).join('');
    expect(fullText.length).toBeGreaterThan(0);
  }, TEST_TIMEOUT_MS);

  it('response has a valid ISO timestamp', async () => {
    await injectMessage('Hi.', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    const ts = Date.parse(msgs[0].timestamp);
    expect(Number.isNaN(ts)).toBe(false);
  }, TEST_TIMEOUT_MS);
});

// ---------------------------------------------------------------------------
// Outbox: clearOutbox actually empties the queue
// ---------------------------------------------------------------------------

describe('Infra — outbox management', () => {
  it('outbox is empty after clearOutbox', async () => {
    await injectMessage('Hello.', { jid: TEST_JID });
    await waitForResponse(AGENT_WAIT_MS);

    await clearOutbox();
    // A fresh getOutbox call should return empty — waitForResponse with a short
    // timeout will throw, confirming no stale messages remain.
    const { getOutbox } = await import('./harness.js');
    const remaining = await getOutbox();
    expect(remaining.length).toBe(0);
  }, TEST_TIMEOUT_MS);
});

// ---------------------------------------------------------------------------
// Multi-turn: two sequential messages in the same session both get responses
// ---------------------------------------------------------------------------

describe('Infra — multi-turn session', () => {
  it('second message in same session gets a response', async () => {
    // First turn
    await injectMessage('Message one.', { jid: TEST_JID });
    const first = await waitForResponse(AGENT_WAIT_MS);
    expect(first.length).toBeGreaterThan(0);

    await clearOutbox();
    await Bun.sleep(2000); // let session settle

    // Second turn — same JID → same session
    await injectMessage('Message two.', { jid: TEST_JID });
    const second = await waitForResponse(AGENT_WAIT_MS);
    expect(second.length).toBeGreaterThan(0);
    expect(second[0].jid).toBe(TEST_JID);
  }, TEST_TIMEOUT_MS * 2);
});
