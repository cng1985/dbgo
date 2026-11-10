import { quoteIdent } from './sqlEditor.js'

const breaks = [
  'union all',
  'left outer join',
  'right outer join',
  'full outer join',
  'inner join',
  'left join',
  'right join',
  'full join',
  'cross join',
  'group by',
  'order by',
  'insert into',
  'delete from',
  'union',
  'select',
  'from',
  'where',
  'having',
  'limit',
  'offset',
  'values',
  'update',
  'set',
  'join',
  'on'
]

export function formatSQL(sql) {
  const source = String(sql || '').replace(/\s+$/, '')
  if (!source.trim()) {
    return source
  }
  const tokens = []
  let plain = ''
  let i = 0
  while (i < source.length) {
    const ch = source[i]
    if (ch === "'" || ch === '"' || ch === '`' || ch === '[') {
      const end = endQuote(source, i)
      tokens.push(source.slice(i, end))
      plain += '\u0000'
      i = end
      continue
    }
    plain += ch
    i += 1
  }
  let next = plain.replace(/[ \t]+/g, ' ').replace(/ *\n */g, '\n')
  const sorted = [...breaks].sort((a, b) => b.length - a.length)
  for (const word of sorted) {
    const pattern = new RegExp(`(^|\\n| )(${word.replace(/ /g, '\\s+')})(?=\\s|\\n|$)`, 'gi')
    next = next.replace(pattern, (_all, lead, matched) => `${lead ? '\n' : ''}${matched.toLowerCase()}`)
  }
  next = next.replace(/^\n+/, '').replace(/\n{3,}/g, '\n\n')
  let index = 0
  return next.replace(/\u0000/g, () => tokens[index++] || '')
}

function endQuote(source, start) {
  const quote = source[start]
  if (quote === '[') {
    for (let i = start + 1; i < source.length; i += 1) {
      if (source[i] === ']' && source[i + 1] === ']') {
        i += 1
        continue
      }
      if (source[i] === ']') {
        return i + 1
      }
    }
    return source.length
  }
  for (let i = start + 1; i < source.length; i += 1) {
    if (source[i] === quote && source[i + 1] === quote) {
      i += 1
      continue
    }
    if (source[i] === quote) {
      return i + 1
    }
  }
  return source.length
}

export function commentSQL(sql) {
  const lines = String(sql || '').split('\n')
  const active = lines.filter((line) => line.trim() !== '')
  const commented = active.length > 0 && active.every((line) => line.trim().startsWith('--'))
  if (commented) {
    return lines.map((line) => line.replace(/^(\s*)--\s?/, '$1')).join('\n')
  }
  return lines.map((line) => (line.trim() === '' ? line : `-- ${line}`)).join('\n')
}

export function explainSQL(driver, sql) {
  const body = String(sql || '').trim()
  if (!body) {
    return ''
  }
  if (driver === 'sqlite') {
    return `explain query plan\n${body}`
  }
  return `explain\n${body}`
}

export function sqlLiteral(value) {
  if (value === null || value === undefined) {
    return 'NULL'
  }
  return `'${String(value).replaceAll("'", "''")}'`
}

export function insertSQL(columns, row) {
  const names = (columns || []).map((column) => column.name).join(', ')
  const values = (row || []).map((cell) => sqlLiteral(cell)).join(', ')
  return `insert into copied_row (${names}) values (${values})`
}

export function filterSQL(driver, source, column, value) {
  const body = String(source || '').trim().replace(/;+\s*$/, '')
  const ident = quoteIdent(driver, column)
  const literal = sqlLiteral(value)
  if (!body) {
    return `select *\nwhere ${ident} = ${literal}`
  }
  return `select * from (\n${body}\n) dbgo_filter\nwhere ${ident} = ${literal}`
}

export async function copyText(value) {
  const text = String(value ?? '')
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }
  const area = document.createElement('textarea')
  area.value = text
  document.body.appendChild(area)
  area.select()
  document.execCommand('copy')
  area.remove()
}
