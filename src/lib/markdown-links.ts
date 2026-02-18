export interface WikiLinkMatch {
  raw: string
  target: string
  alias?: string
  normalizedTarget: string
}

export function normalizeWikiLinkTarget(target: string): string {
  return target.trim().toLowerCase()
}

export function parseWikiLinks(content: string): WikiLinkMatch[] {
  const matches: WikiLinkMatch[] = []
  const pattern = /\[\[([^\[\]\n]+)\]\]/g

  let match = pattern.exec(content)
  while (match) {
    const rawInner = match[1]?.trim() ?? ""
    if (rawInner.length > 0) {
      const [targetPart, ...aliasParts] = rawInner.split("|")
      const target = (targetPart ?? "").trim()
      const alias = aliasParts.join("|").trim()

      if (target.length > 0) {
        matches.push({
          raw: `[[${rawInner}]]`,
          target,
          ...(alias.length > 0 ? { alias } : {}),
          normalizedTarget: normalizeWikiLinkTarget(target),
        })
      }
    }

    match = pattern.exec(content)
  }

  return matches
}
