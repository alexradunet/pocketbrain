export interface VaultTextEntry {
  path: string
  content: string
}

export function extractMarkdownTags(content: string): string[] {
  const tags = new Set<string>()
  const lines = content.split(/\r?\n/)

  for (const line of lines) {
    if (/^#{1,6}\s/.test(line)) {
      continue
    }

    const pattern = /(^|[^\w/])#([A-Za-z0-9][A-Za-z0-9_-]*(?:\/[A-Za-z0-9][A-Za-z0-9_-]*)*)/g
    let match = pattern.exec(line)

    while (match) {
      const tagValue = `#${(match[2] ?? "").toLowerCase()}`
      if (tagValue.length > 1) {
        tags.add(tagValue)
      }
      match = pattern.exec(line)
    }
  }

  return Array.from(tags).sort()
}

export function buildTagIndex(entries: VaultTextEntry[]): Map<string, string[]> {
  const index = new Map<string, string[]>()

  for (const entry of entries) {
    const tags = extractMarkdownTags(entry.content)
    for (const tag of tags) {
      const paths = index.get(tag) ?? []
      if (!paths.includes(entry.path)) {
        paths.push(entry.path)
        index.set(tag, paths)
      }
    }
  }

  return index
}
