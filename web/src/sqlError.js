export function sqlErrorLine(message) {
  const match = /^第\s*(\d+)\s*行/.exec(String(message || '').trim())
  if (!match) {
    return 0
  }
  const line = Number(match[1])
  return Number.isInteger(line) && line > 0 ? line : 0
}
