// One label and one colour for each task status, used by every view.
const STATUS = {
  PENDING: { label: 'Queued', cls: 'bg-slate-inner text-zinc-400 border-steel' },
  RUNNING: { label: 'Running', cls: 'bg-cyan-950 text-cyan-300 border-cyan-500' },
  COMPLETED: { label: 'Done', cls: 'bg-green-950 text-green-400 border-green-800' },
  DONE: { label: 'Done', cls: 'bg-green-950 text-green-400 border-green-800' },
  MERGED: { label: 'Merged', cls: 'bg-green-950 text-green-400 border-green-800' },
  AWAITING_INPUT: { label: 'Needs answer', cls: 'bg-rose-950 text-rose-300 border-rose-700' },
  NEEDS_REVIEW: { label: 'Needs review', cls: 'bg-amber-950 text-amber-300 border-amber-700' },
  PAUSED: { label: 'Paused', cls: 'bg-indigo-950 text-indigo-300 border-indigo-700' },
  BLOCKED: { label: 'Blocked', cls: 'bg-red-950 text-red-400 border-red-800' },
  FAILED: { label: 'Failed', cls: 'bg-red-900 text-red-100 border-red-600' }
}

// statusLabel turns a status such as AWAITING_INPUT into words for the UI.
// A status it does not know is shown readably rather than as a raw enum.
export function statusLabel(status) {
  if (!status) return '—'
  const known = STATUS[status]
  if (known) return known.label
  const words = String(status).replace(/_/g, ' ').toLowerCase()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

// statusClass returns the badge colours (background, text, border) of a status.
export function statusClass(status) {
  return (STATUS[status] || STATUS.PENDING).cls
}
