// A skill's name is its folder name; the Go side accepts the same shape.
// Checked here too, since the name becomes a command-line argument.
const SKILL_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export function isSkillName(name: unknown): name is string {
  return typeof name === 'string' && SKILL_NAME.test(name)
}
