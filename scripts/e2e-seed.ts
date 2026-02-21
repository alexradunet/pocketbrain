/**
 * E2E database seeder.
 *
 * Called by scripts/e2e-entrypoint.sh before PocketBrain starts.
 * Uses the project's own DB helpers so the schema always matches production.
 *
 * Seeds:
 *   - registered_groups: the mock test group (folder=main, requiresTrigger=false)
 *   - chats: FK-required row for the mock JID
 */
import { initDatabase, setRegisteredGroup, storeChatMetadata } from '../src/db.js';
import { MOCK_CHANNEL_JID } from '../src/channels/mock.js';

initDatabase();

setRegisteredGroup(MOCK_CHANNEL_JID, {
  name: 'E2E Test Group',
  folder: 'main',
  trigger: '@bot',
  added_at: new Date().toISOString(),
  requiresTrigger: false, // no trigger word needed — every message goes to the agent
});

// Seed the chats row so messages FK constraint is satisfied before connect()
storeChatMetadata(MOCK_CHANNEL_JID, new Date().toISOString(), 'E2E Test Group', 'mock', true);

console.log(`[e2e-seed] seeded group ${MOCK_CHANNEL_JID} → folder=main`);
