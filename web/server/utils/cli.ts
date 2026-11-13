// The Ringmaster CLI prints "Error: <reason>" followed by its usage text when
// a command fails. cliError keeps just the reason.
export function cliError(err: any, fallback: string): string {
  const detail = String(err?.stderr || err?.message || fallback)
  const lines = detail.split('\n')
  const line = lines.find((l) => l.startsWith('Error:')) || lines[0] || fallback
  return line.replace(/^Error:\s*/, '').trim() || fallback
}
