export const cellPreviewLimit = 80

export function cellPreview(value) {
  if (value === null || value === undefined) {
    return { text: 'NULL', long: false }
  }
  const chars = Array.from(String(value))
  if (chars.length <= cellPreviewLimit) {
    return { text: chars.join(''), long: false }
  }
  return { text: chars.slice(0, cellPreviewLimit).join('') + '…', long: true }
}

export function cycleSort(current, column) {
  if (!current || current.column !== column) {
    return { column, direction: 'asc' }
  }
  if (current.direction === 'asc') {
    return { column, direction: 'desc' }
  }
  return null
}

export function sortResultRows(rows, column, direction) {
  const indexed = (rows || []).map((row, index) => ({ row, index }))
  if (!Number.isInteger(column) || column < 0 || (direction !== 'asc' && direction !== 'desc')) {
    return indexed
  }
  indexed.sort((left, right) => compareCells(left.row?.[column], right.row?.[column], direction) || left.index - right.index)
  return indexed
}

export function formatCopiedCells(cells) {
  const lines = []
  let current = null
  for (const cell of cells) {
    const text = cell.value === null || cell.value === undefined ? 'NULL' : String(cell.value)
    if (!lines.length || cell.row !== current) {
      lines.push([text])
      current = cell.row
    } else {
      lines[lines.length - 1].push(text)
    }
  }
  return lines.map((line) => line.join('\t')).join('\n')
}

function compareCells(left, right, direction) {
  const leftMissing = left === null || left === undefined
  const rightMissing = right === null || right === undefined
  if (leftMissing || rightMissing) {
    if (leftMissing && rightMissing) {
      return 0
    }
    return leftMissing ? 1 : -1
  }
  const order = compareText(String(left), String(right))
  return direction === 'desc' ? -order : order
}

function compareText(left, right) {
  const leftNumber = asNumber(left)
  const rightNumber = asNumber(right)
  if (leftNumber !== null && rightNumber !== null && leftNumber !== rightNumber) {
    return leftNumber - rightNumber
  }
  return left.localeCompare(right, 'zh', { numeric: true, sensitivity: 'base' })
}

function asNumber(value) {
  if (!/^[+-]?(?:\d+\.?\d*|\.\d+)$/.test(value)) {
    return null
  }
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}
