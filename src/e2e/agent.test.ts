/**
 * PocketBrain E2E Test Suite
 *
 * Runs against a live pocketbrain-e2e container (CHANNEL=mock, real OpenCode/LLM).
 * Each test injects a message via MockChannel HTTP API, waits for the AI agent to
 * respond, then uses llmAssert() — itself an LLM call — to verify quality.
 *
 * Requirements:
 *   - POCKETBRAIN_MOCK_URL pointing at the running pocketbrain-e2e container
 *   - ANTHROPIC_API_KEY (or OPENCODE_API_KEY) for the agent AND for llmAssert()
 *
 * Run via: docker compose -f docker-compose.e2e.yml up --build --exit-code-from e2e-runner
 */
import { afterEach, beforeAll, describe, expect, it } from 'bun:test';

import { clearOutbox, injectMessage, llmAssert, waitForReady, waitForResponse } from './harness.js';

const TEST_JID = 'e2e-test@mock.test';

// Long timeout per test — AI agent inference takes 10-90s
const TEST_TIMEOUT_MS = 120_000;
// Slightly shorter agent wait so we fail before bun:test's own timeout
const AGENT_WAIT_MS = 90_000;

beforeAll(async () => {
  // Mock channel health check should already be green (depends_on in compose),
  // but guard against race conditions in local runs.
  await waitForReady(30_000);
  await clearOutbox();
});

afterEach(async () => {
  await clearOutbox();
  // Brief settle time so the next test's poll cycle starts fresh
  await Bun.sleep(2000);
});

// ---------------------------------------------------------------------------
// Basic connectivity
// ---------------------------------------------------------------------------

describe('E2E — connectivity', () => {
  it('receives a response within timeout', async () => {
    await injectMessage('Hello! Are you there?', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    // Agent may stream multiple sendMessage chunks — join them all
    const fullText = msgs.map((m) => m.text).join('\n');
    expect(fullText.length).toBeGreaterThan(0);
    expect(msgs[0].jid).toBe(TEST_JID);
  }, TEST_TIMEOUT_MS);
});

// ---------------------------------------------------------------------------
// AI reasoning quality — verified by a second LLM (llmAssert)
// ---------------------------------------------------------------------------

describe('E2E — AI response quality', () => {
  it('answers a simple arithmetic question correctly', async () => {
    await injectMessage('What is 7 multiplied by 6?', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    const fullText = msgs.map((m) => m.text).join('\n');

    const correct = await llmAssert(
      fullText,
      'The response states that 7 multiplied by 6 equals 42',
    );
    expect(correct).toBe(true);
  }, TEST_TIMEOUT_MS);

  it('answers a factual geography question', async () => {
    await injectMessage('What is the capital city of Japan?', { jid: TEST_JID });
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    const fullText = msgs.map((m) => m.text).join('\n');

    const mentionsTokyo = await llmAssert(
      fullText,
      'The response mentions Tokyo as the capital of Japan',
    );
    expect(mentionsTokyo).toBe(true);
  }, TEST_TIMEOUT_MS);

  it('gives a coherent response to an open-ended question', async () => {
    await injectMessage(
      'Can you briefly explain what machine learning is in one or two sentences?',
      { jid: TEST_JID },
    );
    const msgs = await waitForResponse(AGENT_WAIT_MS);

    expect(msgs.length).toBeGreaterThan(0);
    const fullText = msgs.map((m) => m.text).join('\n');
    // Response must be substantive (not just "yes" or "I cannot help")
    expect(fullText.length).toBeGreaterThan(30);

    const isCoherent = await llmAssert(
      fullText,
      'The response provides a brief, accurate description of machine learning as a field of AI or computer science',
    );
    expect(isCoherent).toBe(true);
  }, TEST_TIMEOUT_MS);
});

// ---------------------------------------------------------------------------
// Multi-turn conversation
// ---------------------------------------------------------------------------

describe('E2E — multi-turn', () => {
  it('handles two sequential messages in the same session', async () => {
    // First turn
    await injectMessage('My favourite number is 17. Remember that.', { jid: TEST_JID });
    const first = await waitForResponse(AGENT_WAIT_MS);
    expect(first.length).toBeGreaterThan(0);

    await clearOutbox();
    await Bun.sleep(3000); // let session settle

    // Second turn — agent should recall the number from context
    await injectMessage('What is my favourite number?', { jid: TEST_JID });
    const second = await waitForResponse(AGENT_WAIT_MS);
    expect(second.length).toBeGreaterThan(0);
    const secondText = second.map((m) => m.text).join('\n');

    const recalls = await llmAssert(
      secondText,
      'The response mentions 17 as the user\'s favourite number',
    );
    expect(recalls).toBe(true);
  }, TEST_TIMEOUT_MS * 2);
});
