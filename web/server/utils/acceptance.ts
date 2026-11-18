// acceptanceOf reads the 'acceptance:' list of a task file's frontmatter,
// as Ringmaster writes it: one quoted or plain command per "- " line, or an
// inline [..] list.
export function acceptanceOf(spec: string): string[] {
  const m = spec.match(/^---\r?\n([\s\S]*?)\r?\n---/)
  if (!m) return []
  const lines = m[1].split(/\r?\n/)
  const at = lines.findIndex((l) => /^acceptance:/.test(l))
  if (at < 0) return []
  const unquote = (v: string) => {
    v = v.trim()
    if (v.startsWith('"')) {
      try {
        return JSON.parse(v)
      } catch {}
    }
    if (v.startsWith("'") && v.endsWith("'")) return v.slice(1, -1).replace(/''/g, "'")
    return v
  }
  const inline = lines[at].slice('acceptance:'.length).trim()
  if (inline.startsWith('[')) {
    try {
      return JSON.parse(inline).map(String)
    } catch {
      return []
    }
  }
  const out: string[] = []
  for (const line of lines.slice(at + 1)) {
    const item = line.match(/^\s*-\s+(.*)$/)
    if (!item) break
    out.push(unquote(item[1]))
  }
  return out.filter(Boolean)
}
