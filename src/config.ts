import path from 'path';

import { readEnvFile } from './env.js';

// Read config values from .env (falls back to process.env).
// Secrets are NOT read here — they stay on disk and are loaded only
// where needed (opencode-manager.ts) to avoid leaking to child processes.
const envConfig = readEnvFile([
  'ASSISTANT_HAS_OWN_NUMBER',
]);

export const ASSISTANT_HAS_OWN_NUMBER =
  (process.env.ASSISTANT_HAS_OWN_NUMBER || envConfig.ASSISTANT_HAS_OWN_NUMBER) === 'true';
export const POLL_INTERVAL = 2000;
export const SCHEDULER_POLL_INTERVAL = 60000;

const PROJECT_ROOT = process.cwd();
export const WORKSPACE_DIR = process.env.WORKSPACE_DIR || PROJECT_ROOT;

export const STORE_DIR = path.resolve(WORKSPACE_DIR, 'store');
export const GROUPS_DIR = path.resolve(WORKSPACE_DIR, 'groups');
export const DATA_DIR = path.resolve(WORKSPACE_DIR, 'data');

export const IPC_POLL_INTERVAL = 1000;
export const IDLE_TIMEOUT = parseInt(
  process.env.IDLE_TIMEOUT || '1800000',
  10,
); // 30min default — how long to keep session alive after last result
export const MAX_CONCURRENT_SESSIONS = Math.max(
  1,
  parseInt(process.env.MAX_CONCURRENT_SESSIONS || '5', 10) || 5,
);

// Timezone for scheduled tasks (cron expressions, etc.)
// Uses system timezone by default
export const TIMEZONE =
  process.env.TZ || Intl.DateTimeFormat().resolvedOptions().timeZone;
