import { quoteIdent } from './sqlEditor.js'
import { sqlLiteral } from './sqlActions.js'

const CLAUSE = new Set([
  'where', 'group', 'order', 'having', 'limit', 'offset', 'fetch', 'window', 'for',
  'union', 'except', 'intersect', 'join', 'inner', 'left', 'right', 'full', 'cross',
  'outer', 'on', 'using', 'set', 'into', 'returning'
])

export function querySource(sql) {
  const tokens = tokenize(String(sql || ''))
  while (tokens.length && tokens[tokens.length - 1].kind === 'semi') {
    tokens.pop()
  }
  if (!tokens.length || tokens.some((token) => token.kind === 'semi')) {
    return null
  }
  if (tokens[0].kind !== 'ident' || tokens[0].lower !== 'select') {
    return null
  }
  let depth = 0
  let from = -1
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token.kind === 'lparen') {
      depth += 1
    } else if (token.kind === 'rparen') {
      depth = Math.max(0, depth - 1)
    } else if (depth === 0 && token.kind === 'ident' && token.lower === 'from') {
      from = i
      break
    }
  }
  if (from < 0 || !tokens[from + 1] || tokens[from + 1].kind === 'lparen') {
    return null
  }
  const first = identText(tokens[from + 1])
  if (!first) {
    return null
  }
  let index = from + 2
  let schema = ''
  let table = first
  if (tokens[index]?.kind === 'dot') {
    const second = identText(tokens[index + 1])
    if (!second) {
      return null
    }
    schema = first
    table = second
    index += 2
  }
  if (tokens[index]?.kind === 'ident' && tokens[index].lower === 'as') {
    if (!identText(tokens[index + 1])) {
      return null
    }
    index += 2
  } else if (identText(tokens[index]) && !CLAUSE.has(tokens[index].lower)) {
    index += 1
  }
  for (let i = index; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token.kind === 'lparen') {
      let nested = 1
      i += 1
      for (; i < tokens.length && nested > 0; i += 1) {
        if (tokens[i].kind === 'lparen') {
          nested += 1
        } else if (tokens[i].kind === 'rparen') {
          nested -= 1
        }
      }
      i -= 1
      continue
    }
    if (token.kind === 'comma') {
      return null
    }
    if (token.kind === 'ident' && ['join', 'union', 'except', 'intersect', 'into', 'inner', 'left', 'right', 'full', 'cross', 'outer'].includes(token.lower)) {
      return null
    }
  }
  return { schema, table }
}

export function rowKeys(tableColumns, indexes, resultColumns) {
  const have = new Set((resultColumns || []).map((column) => String(column.name || '').toLowerCase()))
  const primary = (tableColumns || []).filter((column) => column.primary).map((column) => column.name)
  if (primary.length && primary.every((name) => have.has(name.toLowerCase()))) {
    return { names: primary, exact: true }
  }
  for (const index of indexes || []) {
    if (!index.unique) {
      continue
    }
    const names = indexColumns(index.columns)
    if (names.length && names.every((name) => have.has(name.toLowerCase()))) {
      return { names, exact: true }
    }
  }
  const names = (tableColumns || []).filter((column) => have.has(String(column.name || '').toLowerCase())).map((column) => column.name)
  return { names, exact: false }
}

export function buildEditFields(tableColumns, resultColumns, row, keyNames) {
  const keySet = new Set((keyNames || []).map((name) => String(name).toLowerCase()))
  const byName = new Map((resultColumns || []).map((column, index) => [String(column.name || '').toLowerCase(), index]))
  return (tableColumns || []).filter((column) => byName.has(String(column.name || '').toLowerCase())).map((column) => {
    const original = row?.[byName.get(String(column.name || '').toLowerCase())]
    const missing = original === null || original === undefined
    const locked = !!column.identity || binaryType(column.type)
    return {
      name: column.name,
      type: column.type || '',
      key: keySet.has(String(column.name || '').toLowerCase()),
      editable: !locked,
      locked,
      original: missing ? null : original,
      originalNull: missing,
      value: missing ? '' : String(original),
      isNull: missing
    }
  })
}

function binaryType(type) {
  const base = String(type || '').toLowerCase().split('(')[0].trim()
  return ['blob', 'bytea', 'binary', 'varbinary', 'image', 'raw', 'tinyblob', 'mediumblob', 'longblob'].includes(base)
}

export function updateRowSQL(driver, schema, table, fields) {
  const changed = (fields || []).filter((field) => field.editable && fieldChanged(field))
  const keys = (fields || []).filter((field) => field.key)
  if (!changed.length || !keys.length || !table) {
    return ''
  }
  const target = schema ? `${quoteIdent(driver, schema)}.${quoteIdent(driver, table)}` : quoteIdent(driver, table)
  const sets = changed.map((field) => `${quoteIdent(driver, field.name)} = ${sqlValue(field, field.isNull, field.value)}`)
  const where = keys.map((field) => whereTerm(driver, field))
  return `update ${target} set ${sets.join(', ')} where ${where.join(' and ')}`
}

export function fieldChanged(field) {
  if (!field || !field.editable) {
    return false
  }
  if (!!field.isNull !== !!field.originalNull) {
    return true
  }
  if (field.isNull) {
    return false
  }
  return String(field.value ?? '') !== String(field.original ?? '')
}

function whereTerm(driver, field) {
  const name = quoteIdent(driver, field.name)
  if (field.originalNull) {
    return `${name} is null`
  }
  return `${name} = ${sqlValue(field, false, field.original)}`
}

function sqlValue(field, isNull, value) {
  if (isNull || value === null || value === undefined) {
    return 'NULL'
  }
  const text = String(value)
  const base = String(field.type || '').toLowerCase().split('(')[0].trim()
  if (['int', 'integer', 'bigint', 'smallint', 'tinyint', 'mediumint', 'decimal', 'numeric', 'float', 'real', 'double', 'number'].includes(base) && /^-?\d+(\.\d+)?$/.test(text)) {
    return text
  }
  if (['bool', 'boolean', 'bit'].includes(base) && /^(true|false|0|1)$/i.test(text)) {
    return text.toLowerCase()
  }
  return sqlLiteral(text)
}

function indexColumns(text) {
  const raw = String(text || '').trim()
  if (!raw || /[\s()]/.test(raw)) {
    return []
  }
  return raw.split(',').map((item) => item.trim()).filter(Boolean)
}

function identText(token) {
  if (!token || (token.kind !== 'ident' && token.kind !== 'quoted')) {
    return ''
  }
  return token.raw
}

function tokenize(sql) {
  const tokens = []
  let i = 0
  while (i < sql.length) {
    const ch = sql[i]
    if (/\s/.test(ch)) {
      i += 1
      continue
    }
    if (ch === '-' && sql[i + 1] === '-') {
      while (i < sql.length && sql[i] !== '\n') {
        i += 1
      }
      continue
    }
    if (ch === '/' && sql[i + 1] === '*') {
      i += 2
      while (i < sql.length && !(sql[i] === '*' && sql[i + 1] === '/')) {
        i += 1
      }
      i += 2
      continue
    }
    if (ch === "'" || ch === '"' || ch === '`' || ch === '[') {
      const parsed = readQuote(sql, i)
      tokens.push(parsed.token)
      i = parsed.next
      continue
    }
    if (ch === '.') {
      tokens.push({ kind: 'dot', lower: '.' })
      i += 1
      continue
    }
    if (ch === ',') {
      tokens.push({ kind: 'comma', lower: ',' })
      i += 1
      continue
    }
    if (ch === '(') {
      tokens.push({ kind: 'lparen', lower: '(' })
      i += 1
      continue
    }
    if (ch === ')') {
      tokens.push({ kind: 'rparen', lower: ')' })
      i += 1
      continue
    }
    if (ch === ';') {
      tokens.push({ kind: 'semi', lower: ';' })
      i += 1
      continue
    }
    if (/[\p{L}_]/u.test(ch)) {
      let j = i + 1
      while (j < sql.length && /[\p{L}\p{N}_]/u.test(sql[j])) {
        j += 1
      }
      const raw = sql.slice(i, j)
      tokens.push({ kind: 'ident', raw, lower: raw.toLowerCase() })
      i = j
      continue
    }
    if (/[0-9]/.test(ch)) {
      let j = i + 1
      while (j < sql.length && /[0-9.]/.test(sql[j])) {
        j += 1
      }
      tokens.push({ kind: 'other', lower: '' })
      i = j
      continue
    }
    tokens.push({ kind: 'other', lower: '' })
    i += 1
  }
  return tokens
}

function readQuote(sql, start) {
  const quote = sql[start]
  if (quote === '[') {
    let raw = ''
    for (let i = start + 1; i < sql.length; i += 1) {
      if (sql[i] === ']' && sql[i + 1] === ']') {
        raw += ']'
        i += 1
        continue
      }
      if (sql[i] === ']') {
        return { next: i + 1, token: { kind: 'quoted', raw, lower: raw.toLowerCase() } }
      }
      raw += sql[i]
    }
    return { next: sql.length, token: { kind: 'quoted', raw, lower: raw.toLowerCase() } }
  }
  let raw = ''
  for (let i = start + 1; i < sql.length; i += 1) {
    if (sql[i] === quote && sql[i + 1] === quote) {
      raw += quote
      i += 1
      continue
    }
    if (sql[i] === quote) {
      const kind = quote === "'" ? 'string' : 'quoted'
      return { next: i + 1, token: { kind, raw, lower: raw.toLowerCase() } }
    }
    raw += sql[i]
  }
  return { next: sql.length, token: { kind: 'string', raw, lower: raw.toLowerCase() } }
}
