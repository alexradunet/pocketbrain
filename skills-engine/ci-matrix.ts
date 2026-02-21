import fs from 'fs';
import path from 'path';

import { readManifest } from './manifest.js';
import { SkillManifest } from './types.js';

export interface SkillOverlapInfo {
  name: string;
  modifies: string[];
  bunDependencies: string[];
}

export interface MatrixEntry {
  skills: [string, string];
  reason: string;
}

export function extractOverlapInfo(
  manifest: SkillManifest,
  dirName: string,
): SkillOverlapInfo {
  return {
    name: dirName,
    modifies: manifest.modifies ?? [],
    bunDependencies: Object.keys(manifest.structured?.bun_dependencies ?? {}),
  };
}

export function computeOverlapMatrix(skills: SkillOverlapInfo[]): MatrixEntry[] {
  const result: MatrixEntry[] = [];

  for (let i = 0; i < skills.length; i++) {
    for (let j = i + 1; j < skills.length; j++) {
      const a = skills[i];
      const b = skills[j];

      const sharedModifies = a.modifies.filter((item) => b.modifies.includes(item));
      const sharedBun = a.bunDependencies.filter((item) =>
        b.bunDependencies.includes(item),
      );

      if (sharedModifies.length === 0 && sharedBun.length === 0) continue;

      const reasons: string[] = [];
      if (sharedModifies.length > 0) {
        reasons.push(`shared modifies: ${sharedModifies.join(', ')}`);
      }
      if (sharedBun.length > 0) {
        reasons.push(`shared bun packages: ${sharedBun.join(', ')}`);
      }

      result.push({
        skills: [a.name, b.name],
        reason: reasons.join('; '),
      });
    }
  }

  return result;
}

export function generateMatrix(skillsDir: string): MatrixEntry[] {
  if (!fs.existsSync(skillsDir)) return [];

  const infos: SkillOverlapInfo[] = [];

  for (const entry of fs.readdirSync(skillsDir, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;

    const dir = path.join(skillsDir, entry.name);
    const manifestPath = path.join(dir, 'manifest.yaml');
    if (!fs.existsSync(manifestPath)) continue;

    try {
      const manifest = readManifest(dir);
      infos.push(extractOverlapInfo(manifest, entry.name));
    } catch {
      // Skip invalid manifests in CI matrix generation.
    }
  }

  return computeOverlapMatrix(infos);
}


