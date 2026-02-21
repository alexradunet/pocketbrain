import { describe, it, expect } from 'bun:test';
import {
  POCKETBRAIN_DIR,
  STATE_FILE,
  BASE_DIR,
  BACKUP_DIR,
  LOCK_FILE,
  CUSTOM_DIR,
  RESOLUTIONS_DIR,
  SKILLS_SCHEMA_VERSION,
} from '../constants.js';

describe('constants', () => {
  const allConstants = {
    POCKETBRAIN_DIR,
    STATE_FILE,
    BASE_DIR,
    BACKUP_DIR,
    LOCK_FILE,
    CUSTOM_DIR,
    RESOLUTIONS_DIR,
    SKILLS_SCHEMA_VERSION,
  };

  it('all constants are non-empty strings', () => {
    for (const [name, value] of Object.entries(allConstants)) {
      expect(value, `${name} should be a non-empty string`).toBeTruthy();
      expect(typeof value, `${name} should be a string`).toBe('string');
    }
  });

  it('path constants use forward slashes and .pocketbrain prefix', () => {
    const pathConstants = [BASE_DIR, BACKUP_DIR, LOCK_FILE, CUSTOM_DIR, RESOLUTIONS_DIR];
    for (const p of pathConstants) {
      expect(p).not.toContain('\\');
      expect(p).toMatch(/^\.pocketbrain\//);
    }
  });

  it('POCKETBRAIN_DIR is .pocketbrain', () => {
    expect(POCKETBRAIN_DIR).toBe('.pocketbrain');
  });
});

