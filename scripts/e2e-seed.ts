/**
 * E2E data seeder.
 *
 * Called by scripts/e2e-entrypoint.sh before PocketBrain starts.
 * Creates file-based chat config so the message loop recognises the mock JID.
 *
 * Seeds:
 *   - data/chats/main/config.json: the mock test chat
 */
import { ensureDataDirs, saveChat } from '../src/store.js';
import { MOCK_CHANNEL_JID } from '../src/channels/mock.js';

ensureDataDirs();

saveChat({
  jid: MOCK_CHANNEL_JID,
  name: 'E2E Test Chat',
  folder: 'main',
  addedAt: new Date().toISOString(),
});

console.log(`[e2e-seed] seeded chat ${MOCK_CHANNEL_JID} → folder=main`);
