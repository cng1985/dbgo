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
  const order = compareNumericText(left, right)
  if (order !== null) {
    return order
  }
  return left.localeCompare(right, 'zh', { numeric: true, sensitivity: 'base' })
}

function compareNumericText(left, right) {
  const a = numericParts(left)
  const b = numericParts(right)
  if (!a || !b) {
    return null
  }
  if (a.negative !== b.negative) {
    return a.negative ? -1 : 1
  }
  const sign = a.negative ? -1 : 1
  const integer = compareDigits(a.integer, b.integer)
  if (integer !== 0) {
    return sign * integer
  }
  return sign * compareFraction(a.fraction, b.fraction)
}

function numericParts(value) {
  const match = /^([+-]?)(?:(\d+)(?:\.(\d*))?|\.(\d+))$/.exec(value)
  if (!match) {
    return null
  }
  const integer = (match[2] || '0').replace(/^0+/, '') || '0'
  const fraction = (match[3] || match[4] || '').replace(/0+$/, '')
  return {
    negative: match[1] === '-' && !(integer === '0' && fraction === ''),
    integer,
    fraction
  }
}

function compareDigits(left, right) {
  if (left.length !== right.length) {
    return left.length - right.length
  }
  if (left === right) {
    return 0
  }
  return left < right ? -1 : 1
}

function compareFraction(left, right) {
  const length = Math.max(left.length, right.length)
  const a = left.padEnd(length, '0')
  const b = right.padEnd(length, '0')
  if (a === b) {
    return 0
  }
  return a < b ? -1 : 1
}
