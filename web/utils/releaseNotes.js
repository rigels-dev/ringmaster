// Release notes in the release form. Kept apart from the UI state so it can
// be tested on its own. The placeholder matches release.SummaryPlaceholder in Go.

export const SUMMARY_PLACEHOLDER = '<!-- Summary: a sentence or two on what this release is for. Write it here, or let AI write it. -->'

// Puts the summary where the placeholder is, or on top when the user has
// removed it, keeping everything they wrote.
export function insertSummary(notes, summary) {
  const s = String(summary || '').trim()
  if (!s) return notes
  if (notes.includes(SUMMARY_PLACEHOLDER)) return notes.replace(SUMMARY_PLACEHOLDER, () => s)
  if (!notes.trim()) return s + '\n'
  return s + '\n\n' + notes.replace(/^\n+/, '')
}

// Whether the summary is still to be written
export function needsSummary(notes) {
  return notes.includes(SUMMARY_PLACEHOLDER)
}
