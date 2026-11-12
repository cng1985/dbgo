export function normalizeNewlines(value) {
  return String(value ?? '').replace(/\r\n/g, '\n').replace(/\r/g, '\n')
}

export function sqlErrorLine(message) {
  const match = /^第\s*(\d+)\s*行/.exec(String(message || '').trim())
  if (!match) {
    return 0
  }
  const line = Number(match[1])
  return Number.isInteger(line) && line > 0 ? line : 0
}

export function editorErrorLine(message, originLine = 1) {
  const line = sqlErrorLine(message)
  if (!line) {
    return 0
  }
  const origin = Number.isInteger(originLine) && originLine > 0 ? originLine : 1
  return line + origin - 1
}

export function translateErrorLine(message, originLine = 1) {
  const line = editorErrorLine(message, originLine)
  if (!line || line === sqlErrorLine(message)) {
    return String(message || '')
  }
  return String(message).replace(/^第\s*\d+\s*行/, `第 ${line} 行`)
}

export function lineNumberAt(text, index) {
  const offset = Math.max(0, Number(index) || 0)
  return String(text || '').slice(0, offset).split('\n').length
}
