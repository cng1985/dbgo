const BASE_KEYWORDS = [
  'select', 'from', 'where', 'and', 'or', 'not', 'in', 'is', 'null', 'like', 'between',
  'exists', 'as', 'on', 'join', 'inner', 'left', 'right', 'full', 'cross', 'outer',
  'group', 'by', 'order', 'having', 'limit', 'offset', 'distinct', 'all', 'any', 'some',
  'union', 'intersect', 'except', 'insert', 'into', 'values', 'update', 'set', 'delete',
  'create', 'alter', 'drop', 'table', 'view', 'index', 'column', 'add', 'primary', 'key',
  'foreign', 'references', 'constraint', 'unique', 'check', 'default', 'cascade', 'restrict',
  'if', 'case', 'when', 'then', 'else', 'end', 'cast', 'count', 'sum', 'avg', 'min', 'max',
  'coalesce', 'nullif', 'with', 'recursive', 'asc', 'desc', 'true', 'false', 'using',
  'begin', 'commit', 'rollback', 'transaction', 'replace', 'temporary', 'temp', 'explain',
  'analyze', 'over', 'partition', 'window', 'rows', 'range', 'unbounded', 'preceding',
  'following', 'current', 'row', 'only', 'fetch', 'next', 'first', 'last', 'filter',
  'int', 'integer', 'bigint', 'smallint', 'text', 'blob', 'boolean', 'bool', 'datetime',
  'timestamp', 'date', 'time', 'numeric', 'decimal', 'float', 'real', 'double', 'char',
  'varchar', 'precision'
]

const EXTRA_KEYWORDS = {
  mysql: ['show', 'describe', 'desc', 'use', 'engine', 'charset', 'auto_increment', 'unsigned', 'zerofill', 'duplicate', 'straight_join', 'regexp', 'rlike', 'separator', 'interval', 'binary', 'force', 'low_priority', 'ifnull', 'database'],
  postgres: ['ilike', 'similar', 'returning', 'tablespace', 'materialized', 'concurrently', 'vacuum', 'analyse', 'excluded', 'variadic', 'unnest', 'do', 'language', 'schema', 'sequence', 'nothing', 'lateral', 'jsonb', 'uuid', 'serial'],
  sqlite: ['pragma', 'autoincrement', 'conflict', 'nothing', 'abort', 'fail', 'ignore', 'vacuum', 'without', 'rowid', 'strict', 'glob', 'regexp', 'match', 'isnull', 'notnull', 'indexed', 'returning'],
  sqlserver: ['top', 'identity', 'output', 'merge', 'matched', 'pivot', 'unpivot', 'apply', 'nolock', 'nocount', 'exec', 'execute', 'tran', 'clustered', 'nonclustered', 'include', 'datetime2', 'uniqueidentifier', 'declare', 'cursor', 'while', 'try', 'catch', 'nvarchar']
}

const COLUMN_START = ['distinct', 'count', 'sum', 'avg', 'min', 'max', 'case', 'cast', 'coalesce', 'nullif', 'not', 'as', 'from', 'where']
const TABLE_INTRO = new Set(['from', 'join', 'into', 'update', 'table'])
const COLUMN_CLAUSE = new Set(['select', 'where', 'having', 'on', 'set', 'by', 'and', 'or', 'when', 'then', 'else', 'returning', 'values'])
const CLAUSE_WORDS = new Set([
  'select', 'from', 'where', 'group', 'order', 'having', 'limit', 'offset', 'join', 'into',
  'update', 'set', 'on', 'by', 'and', 'or', 'when', 'then', 'else', 'returning', 'values',
  'table', 'delete', 'insert', 'create', 'alter', 'drop'
])

const listCache = new Map()
const setCache = new Map()

export function keywordList(driver) {
  const key = driver || '*'
  if (listCache.has(key)) {
    return listCache.get(key)
  }
  const extra = driver ? (EXTRA_KEYWORDS[driver] || []) : Object.values(EXTRA_KEYWORDS).flat()
  const seen = new Set()
  const list = []
  for (const word of [...BASE_KEYWORDS, ...extra]) {
    const lower = word.toLowerCase()
    if (seen.has(lower)) {
      continue
    }
    seen.add(lower)
    list.push(lower)
  }
  listCache.set(key, list)
  return list
}

export function keywordSet(driver) {
  const key = driver || '*'
  if (setCache.has(key)) {
    return setCache.get(key)
  }
  const set = new Set(keywordList(driver))
  setCache.set(key, set)
  return set
}

export function quoteIdent(driver, name, keywords) {
  const words = keywords || keywordSet(driver)
  const text = String(name ?? '')
  if (/^[\p{L}_][\p{L}\p{N}_]*$/u.test(text) && !words.has(text.toLowerCase())) {
    return text
  }
  if (driver === 'mysql') {
    return '`' + text.replaceAll('`', '``') + '`'
  }
  if (driver === 'sqlserver') {
    return '[' + text.replaceAll(']', ']]') + ']'
  }
  return '"' + text.replaceAll('"', '""') + '"'
}

export function highlightSQL(sql, driver, tables) {
  const keywords = keywordSet(driver)
  const names = nameSets(tables)
  const tokens = tokenize(sql || '', keywords, driver)
  let html = ''
  for (const token of tokens) {
    if (token.type === 'space') {
      html += escapeHtml(token.text)
      continue
    }
    let kind = token.type
    if (kind === 'ident') {
      const lower = token.text.toLowerCase()
      if (names.tables.has(lower)) {
        kind = 'table'
      } else if (names.columns.has(lower)) {
        kind = 'column'
      }
    }
    html += `<span class="sql-${kind}">${escapeHtml(token.text)}</span>`
  }
  if ((sql || '').endsWith('\n')) {
    html += ' '
  }
  return html
}

export function suggestSQL({ sql = '', cursor = 0, driver = '', tables = [], schemas = [], schema = '' } = {}) {
  const source = String(sql)
  const caret = Math.max(0, Math.min(cursor, source.length))
  const keywords = keywordSet(driver)
  const tokens = tokenize(source, keywords, driver)
  const empty = { items: [], from: caret, to: caret, prefix: '', mode: 'literal', autoOpen: false }
  if (isBlocked(tokens, caret)) {
    return empty
  }
  const activeIndex = findActive(tokens, caret)
  const active = activeIndex >= 0 ? tokens[activeIndex] : null
  const previous = previousToken(tokens, activeIndex, caret)
  const clauseStart = active ? activeIndex - 1 : (previous ? previous.index : -1)
  const clause = clauseStart >= 0 ? findClause(tokens, clauseStart) : ''
  const mode = decideMode(previous, clause, depthAt(tokens, caret))
  const prefix = active ? source.slice(active.start, caret) : ''
  const from = active ? active.start : caret
  const to = active ? active.end : caret
  const catalog = indexCatalog(tables, schemas, schema)
  const referenced = referencedTables(tokens, caret, catalog)
  const parts = mode === 'member' && previous ? qualifierParts(tokens, previous.index) : []
  const items = collectItems({
    mode,
    prefix,
    driver,
    keywords,
    catalog,
    referenced,
    parts,
    sql: source,
    to,
    clause
  })
  return {
    items,
    from,
    to,
    prefix,
    mode,
    autoOpen: shouldAutoOpen(mode, prefix, previous)
  }
}

function collectItems(ctx) {
  const prefix = ctx.prefix.toLowerCase()
  const items = []
  if (ctx.mode === 'member') {
    addMember(items, ctx, prefix)
    return items
  }
  if (ctx.mode === 'table') {
    addSchemas(items, ctx, prefix, 8)
    addTables(items, ctx, { prefix, qualify: true, limit: 20 })
    if (prefix) {
      addKeywords(items, ctx, keywordList(ctx.driver), prefix, 8)
    }
    return items
  }
  if (ctx.mode === 'column') {
    const source = columnSource(ctx, prefix)
    addStar(items, ctx, prefix, source)
    const ambiguous = source.length > 1
    addColumns(items, ctx, source, prefix, ambiguous)
    addKeywords(items, ctx, prefix ? keywordList(ctx.driver) : COLUMN_START, prefix, prefix ? 12 : 8)
    return items
  }
  if (ctx.mode === 'follow') {
    const preferred = followWords(ctx.clause)
    const words = prefix ? uniqueWords([...preferred, ...keywordList(ctx.driver)]) : preferred
    addKeywords(items, ctx, words, prefix, 12)
    if (prefix) {
      addTables(items, ctx, { prefix, qualify: true, limit: 10 })
    }
    return items
  }
  addKeywords(items, ctx, prefix ? keywordList(ctx.driver) : keywordList(ctx.driver).slice(0, 12), prefix, 12)
  if (prefix) {
    addSchemas(items, ctx, prefix, 6)
    addTables(items, ctx, { prefix, qualify: true, limit: 12 })
    addColumns(items, ctx, allTables(ctx.catalog), prefix, allTables(ctx.catalog).length > 1)
  }
  return items
}

function addMember(items, ctx, prefix) {
  const parts = ctx.parts
  if (!parts.length) {
    return
  }
  if (parts.length >= 2) {
    const schema = parts[parts.length - 2]
    const table = parts[parts.length - 1]
    const entry = ctx.catalog.byKey.get(schema.toLowerCase() + '.' + table.toLowerCase())
    if (entry) {
      addColumns(items, ctx, [entry], prefix, false)
    }
    return
  }
  const name = parts[0].toLowerCase()
  if (ctx.catalog.schemaNames.has(name)) {
    addTables(items, ctx, { prefix, schemaFilter: parts[0], qualify: false, limit: 20 })
  }
  const matches = ctx.catalog.byName.get(name) || []
  if (matches.length) {
    const preferred = matches.find((item) => item.schema.toLowerCase() === ctx.catalog.defaultSchema.toLowerCase())
    const chosen = preferred ? [preferred] : matches
    addColumns(items, ctx, chosen, prefix, chosen.length > 1)
  }
}

function columnSource(ctx, prefix) {
  if (ctx.referenced.length) {
    return ctx.referenced
  }
  return prefix ? allTables(ctx.catalog) : []
}

function addStar(items, ctx, prefix, source) {
  if (ctx.clause !== 'select' || (prefix && prefix !== '*')) {
    return
  }
  const known = source.some((table) => (table.columns || []).some((column) => column?.name))
  if (!known) {
    return
  }
  items.push({ kind: 'column', label: '*', detail: '全部字段', insert: '*', section: '字段' })
}

function addSchemas(items, ctx, prefix, limit) {
  const names = [...ctx.catalog.schemaNames.values()].sort((a, b) => a.localeCompare(b, 'zh'))
  let count = 0
  for (const name of names) {
    if (prefix && !name.toLowerCase().startsWith(prefix)) {
      continue
    }
    items.push({
      kind: 'schema',
      label: name,
      detail: '',
      insert: quoteIdent(ctx.driver, name, ctx.keywords),
      section: '模式'
    })
    count += 1
    if (count >= limit) {
      break
    }
  }
}

function addTables(items, ctx, { prefix, schemaFilter = '', qualify = false, limit = 20 }) {
  const entries = []
  for (const list of ctx.catalog.byName.values()) {
    for (const entry of list) {
      if (schemaFilter && entry.schema.toLowerCase() !== schemaFilter.toLowerCase()) {
        continue
      }
      if (prefix && !entry.name.toLowerCase().startsWith(prefix)) {
        continue
      }
      entries.push(entry)
    }
  }
  entries.sort((a, b) => {
    const ad = a.schema.toLowerCase() === ctx.catalog.defaultSchema.toLowerCase() ? 0 : 1
    const bd = b.schema.toLowerCase() === ctx.catalog.defaultSchema.toLowerCase() ? 0 : 1
    if (ad !== bd) {
      return ad - bd
    }
    const byName = a.name.localeCompare(b.name, 'zh')
    if (byName !== 0) {
      return byName
    }
    return a.schema.localeCompare(b.schema, 'zh')
  })
  const schemas = new Set(entries.map((entry) => entry.schema.toLowerCase()))
  const showSchema = !schemaFilter && schemas.size > 1
  for (const entry of entries.slice(0, limit)) {
    const kindLabel = entry.type === 'view' ? '视图' : '表'
    items.push({
      kind: 'table',
      label: entry.name,
      detail: showSchema ? `${entry.schema} · ${kindLabel}` : (entry.type === 'view' ? '视图' : ''),
      insert: qualify ? qualifiedTable(entry, ctx) : quoteIdent(ctx.driver, entry.name, ctx.keywords),
      section: '数据表'
    })
  }
}

function qualifiedTable(entry, ctx) {
  const name = quoteIdent(ctx.driver, entry.name, ctx.keywords)
  if (!entry.schema || entry.schema.toLowerCase() === ctx.catalog.defaultSchema.toLowerCase()) {
    return name
  }
  return quoteIdent(ctx.driver, entry.schema, ctx.keywords) + '.' + name
}

function addColumns(items, ctx, tables, prefix, ambiguous) {
  let count = 0
  const seen = new Set()
  for (const table of tables) {
    for (const column of table.columns || []) {
      if (!column?.name) {
        continue
      }
      if (prefix && !column.name.toLowerCase().startsWith(prefix)) {
        continue
      }
      const key = (ambiguous ? table.schema + '.' + table.name + '.' : '') + column.name.toLowerCase()
      if (seen.has(key)) {
        continue
      }
      seen.add(key)
      const detail = ambiguous
        ? [table.name, column.type].filter(Boolean).join(' · ')
        : (column.type || '')
      items.push({
        kind: 'column',
        label: column.name,
        detail,
        insert: quoteIdent(ctx.driver, column.name, ctx.keywords),
        section: '字段'
      })
      count += 1
      if (count >= 24) {
        return
      }
    }
  }
}

function addKeywords(items, ctx, words, prefix, limit) {
  const seen = new Set(items.filter((item) => item.kind === 'keyword').map((item) => item.label))
  let count = 0
  for (const word of words) {
    if (seen.has(word)) {
      continue
    }
    if (prefix && !word.startsWith(prefix)) {
      continue
    }
    seen.add(word)
    items.push({
      kind: 'keyword',
      label: word,
      detail: '',
      insert: keywordInsert(word, ctx.sql, ctx.to),
      section: '关键字'
    })
    count += 1
    if (count >= limit) {
      break
    }
  }
}

function keywordInsert(word, sql, to) {
  const next = sql[to] || ''
  if (!next || /[\s,)]/.test(next)) {
    return /\s/.test(next) ? word : word + ' '
  }
  if (next === '(' || next === '.' ) {
    return word
  }
  return word + ' '
}

function followWords(clause) {
  if (clause === 'group' || clause === 'order') {
    return ['by']
  }
  if (clause === 'update') {
    return ['set', 'where', 'from']
  }
  return ['as', 'where', 'join', 'left', 'inner', 'right', 'full', 'cross', 'on', 'group', 'order', 'having', 'limit', 'offset', 'union', 'and', 'or', 'set']
}

function shouldAutoOpen(mode, prefix, previous) {
  if (prefix) {
    return true
  }
  if (mode === 'member' || mode === 'table' || mode === 'follow') {
    return true
  }
  if (mode === 'column' && previous) {
    if (previous.token.type === 'keyword') {
      return true
    }
    if (previous.token.type === 'punct' && (previous.token.text === ',' || previous.token.text === '(')) {
      return true
    }
  }
  return false
}

function decideMode(previous, clause, depth) {
  if (!previous) {
    return 'any'
  }
  const token = previous.token
  const word = token.text.toLowerCase()
  if (token.type === 'punct' && token.text === '.') {
    return 'member'
  }
  if (token.type === 'keyword' && TABLE_INTRO.has(word)) {
    return 'table'
  }
  if (token.type === 'punct' && token.text === ',' && (clause === 'from' || clause === 'join')) {
    return 'table'
  }
  if (token.type === 'punct' && token.text === '(' && clause === 'into' && depth > 0) {
    return 'column'
  }
  if (token.type === 'keyword' && (word === 'group' || word === 'order')) {
    return 'follow'
  }
  if (COLUMN_CLAUSE.has(clause)) {
    return 'column'
  }
  if (clause === 'from' || clause === 'join' || clause === 'update' || clause === 'table') {
    return 'follow'
  }
  return 'any'
}

function indexCatalog(tables, schemas, defaultSchema) {
  const schemaNames = new Map()
  for (const name of schemas || []) {
    if (name) {
      schemaNames.set(String(name).toLowerCase(), String(name))
    }
  }
  const byKey = new Map()
  const byName = new Map()
  for (const table of tables || []) {
    if (!table?.name) {
      continue
    }
    const schemaName = table.schema || defaultSchema || ''
    if (schemaName) {
      schemaNames.set(schemaName.toLowerCase(), schemaName)
    }
    const entry = {
      schema: schemaName,
      name: table.name,
      type: table.type || 'table',
      columns: table.columns || []
    }
    byKey.set(schemaName.toLowerCase() + '.' + table.name.toLowerCase(), entry)
    const bucket = byName.get(table.name.toLowerCase()) || []
    bucket.push(entry)
    byName.set(table.name.toLowerCase(), bucket)
  }
  return { schemaNames, byKey, byName, defaultSchema: defaultSchema || '' }
}

function allTables(catalog) {
  return [...catalog.byKey.values()]
}

function referencedTables(tokens, cursor, catalog) {
  const statement = statementTokens(tokens, cursor)
  const found = []
  const seen = new Set()
  for (let i = 0; i < statement.length; i += 1) {
    const token = statement[i]
    if (token.type !== 'keyword' || !TABLE_INTRO.has(token.text.toLowerCase())) {
      continue
    }
    const name = readName(statement, i + 1)
    const resolved = resolveTable(catalog, name)
    if (!resolved) {
      continue
    }
    const key = resolved.schema.toLowerCase() + '.' + resolved.name.toLowerCase()
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    found.push(resolved)
  }
  return found
}

function resolveTable(catalog, name) {
  if (!name?.table) {
    return null
  }
  if (name.schema) {
    return catalog.byKey.get(name.schema.toLowerCase() + '.' + name.table.toLowerCase()) || null
  }
  const list = catalog.byName.get(name.table.toLowerCase()) || []
  return list.find((item) => item.schema.toLowerCase() === catalog.defaultSchema.toLowerCase()) || list[0] || null
}

function readName(tokens, index) {
  let i = index
  while (tokens[i] && tokens[i].type === 'space') {
    i += 1
  }
  const parts = []
  while (tokens[i] && (tokens[i].type === 'ident' || tokens[i].type === 'quoted' || tokens[i].type === 'keyword')) {
    parts.push(unquote(tokens[i].text))
    let j = i + 1
    while (tokens[j] && tokens[j].type === 'space') {
      j += 1
    }
    if (tokens[j] && tokens[j].type === 'punct' && tokens[j].text === '.') {
      i = j + 1
      while (tokens[i] && tokens[i].type === 'space') {
        i += 1
      }
      continue
    }
    break
  }
  if (!parts.length) {
    return null
  }
  if (parts.length === 1) {
    return { table: parts[0] }
  }
  return { schema: parts[parts.length - 2], table: parts[parts.length - 1] }
}

function qualifierParts(tokens, dotIndex) {
  const parts = []
  let i = dotIndex - 1
  while (i >= 0 && tokens[i].type === 'space') {
    i -= 1
  }
  while (i >= 0) {
    const token = tokens[i]
    if (token.type !== 'ident' && token.type !== 'quoted' && token.type !== 'keyword') {
      break
    }
    parts.unshift(unquote(token.text))
    i -= 1
    while (i >= 0 && tokens[i].type === 'space') {
      i -= 1
    }
    if (i >= 0 && tokens[i].type === 'punct' && tokens[i].text === '.') {
      i -= 1
      while (i >= 0 && tokens[i].type === 'space') {
        i -= 1
      }
      continue
    }
    break
  }
  return parts
}

function statementTokens(tokens, cursor) {
  let start = 0
  let depth = 0
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token.start >= cursor) {
      break
    }
    if (token.type === 'punct' && token.text === '(') {
      depth += 1
    } else if (token.type === 'punct' && token.text === ')') {
      depth = Math.max(0, depth - 1)
    } else if (token.type === 'punct' && token.text === ';' && depth === 0) {
      start = i + 1
    }
  }
  let end = tokens.length
  depth = 0
  for (let i = start; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token.type === 'punct' && token.text === '(') {
      depth += 1
    } else if (token.type === 'punct' && token.text === ')') {
      depth = Math.max(0, depth - 1)
    } else if (token.type === 'punct' && token.text === ';' && depth === 0 && token.start >= cursor) {
      end = i
      break
    }
  }
  return tokens.slice(start, end)
}

function findClause(tokens, startIndex) {
  let depth = 0
  for (let i = startIndex; i >= 0; i -= 1) {
    const token = tokens[i]
    if (token.type === 'punct' && token.text === ')') {
      depth += 1
      continue
    }
    if (token.type === 'punct' && token.text === '(') {
      if (depth > 0) {
        depth -= 1
      }
      continue
    }
    if (depth > 0) {
      continue
    }
    if (token.type === 'keyword' && CLAUSE_WORDS.has(token.text.toLowerCase())) {
      return token.text.toLowerCase()
    }
  }
  return ''
}

function findActive(tokens, cursor) {
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i]
    if (token.start < cursor && cursor <= token.end && (token.type === 'ident' || token.type === 'keyword')) {
      return i
    }
  }
  return -1
}

function previousToken(tokens, activeIndex, cursor) {
  let start = activeIndex >= 0 ? activeIndex - 1 : -1
  if (activeIndex < 0) {
    for (let i = 0; i < tokens.length; i += 1) {
      if (tokens[i].start >= cursor) {
        break
      }
      if (tokens[i].type !== 'space' && tokens[i].end <= cursor) {
        start = i
      }
    }
  }
  for (let i = start; i >= 0; i -= 1) {
    if (tokens[i].type !== 'space') {
      return { token: tokens[i], index: i }
    }
  }
  return null
}

function depthAt(tokens, cursor) {
  let depth = 0
  for (const token of tokens) {
    if (token.start >= cursor) {
      break
    }
    if (token.type === 'punct' && token.text === '(') {
      depth += 1
    } else if (token.type === 'punct' && token.text === ')') {
      depth = Math.max(0, depth - 1)
    }
  }
  return depth
}

function isBlocked(tokens, cursor) {
  return tokens.some((token) => covers(token, cursor))
}

function covers(token, cursor) {
  if (token.type === 'number') {
    return token.start < cursor && cursor < token.end
  }
  if (token.type !== 'string' && token.type !== 'comment' && token.type !== 'quoted') {
    return false
  }
  if (token.line || !token.closed) {
    return token.start < cursor && cursor <= token.end
  }
  return token.start < cursor && cursor < token.end
}

function nameSets(tables) {
  const tableNames = new Set()
  const columnNames = new Set()
  for (const table of tables || []) {
    if (table?.name) {
      tableNames.add(table.name.toLowerCase())
    }
    for (const column of table?.columns || []) {
      if (column?.name) {
        columnNames.add(column.name.toLowerCase())
      }
    }
  }
  return { tables: tableNames, columns: columnNames }
}

function uniqueWords(words) {
  const seen = new Set()
  const list = []
  for (const word of words) {
    if (seen.has(word)) {
      continue
    }
    seen.add(word)
    list.push(word)
  }
  return list
}

function unquote(text) {
  if (!text || text.length < 2) {
    return text || ''
  }
  const first = text[0]
  if (first !== '"' && first !== '`' && first !== '[') {
    return text
  }
  const end = first === '[' ? ']' : first
  if (!text.endsWith(end)) {
    return text
  }
  const body = text.slice(1, -1)
  if (end === ']') {
    return body.replaceAll(']]', ']')
  }
  return body.replaceAll(end + end, end)
}

function tokenize(sql, keywords, driver) {
  const tokens = []
  const n = sql.length
  let i = 0
  while (i < n) {
    const ch = sql[i]
    const next = sql[i + 1] || ''
    if (ch === '-' && next === '-') {
      const start = i
      i += 2
      while (i < n && sql[i] !== '\n') {
        i += 1
      }
      tokens.push(token('comment', sql, start, i, { line: true, closed: i < n }))
      continue
    }
    if (ch === '#' && driver === 'mysql') {
      const start = i
      i += 1
      while (i < n && sql[i] !== '\n') {
        i += 1
      }
      tokens.push(token('comment', sql, start, i, { line: true, closed: i < n }))
      continue
    }
    if (ch === '/' && next === '*') {
      const start = i
      i += 2
      while (i < n && !(sql[i] === '*' && sql[i + 1] === '/')) {
        i += 1
      }
      const closed = i < n
      if (closed) {
        i += 2
      }
      tokens.push(token('comment', sql, start, i, { closed }))
      continue
    }
    if (ch === "'") {
      const start = i
      const scanned = scanQuoted(sql, i, "'")
      i = scanned.end
      tokens.push(token('string', sql, start, i, { closed: scanned.closed }))
      continue
    }
    if (ch === '"' || ch === '`' || ch === '[') {
      const start = i
      const scanned = scanQuoted(sql, i, ch)
      i = scanned.end
      tokens.push(token('quoted', sql, start, i, { closed: scanned.closed }))
      continue
    }
    if (isSpace(ch)) {
      const start = i
      while (i < n && isSpace(sql[i])) {
        i += 1
      }
      tokens.push(token('space', sql, start, i))
      continue
    }
    if (/[0-9]/.test(ch)) {
      const start = i
      i = scanNumber(sql, i)
      tokens.push(token('number', sql, start, i))
      continue
    }
    if (isIdentStart(ch)) {
      const start = i
      i += 1
      while (i < n && isIdentPart(sql[i])) {
        i += 1
      }
      const text = sql.slice(start, i)
      const type = keywords.has(text.toLowerCase()) ? 'keyword' : 'ident'
      tokens.push({ type, start, end: i, text, closed: true })
      continue
    }
    if (ch === '.' || ch === ',' || ch === ';' || ch === '(' || ch === ')') {
      tokens.push(token('punct', sql, i, i + 1))
      i += 1
      continue
    }
    const start = i
    i += 1
    while (i < n && isOperator(sql[i])) {
      i += 1
    }
    tokens.push(token('operator', sql, start, i))
  }
  return tokens
}

function token(type, sql, start, end, extra = {}) {
  return { type, start, end, text: sql.slice(start, end), closed: true, ...extra }
}

function scanQuoted(sql, i, quote) {
  const endChar = quote === '[' ? ']' : quote
  i += 1
  while (i < sql.length) {
    if (sql[i] === endChar && sql[i + 1] === endChar) {
      i += 2
      continue
    }
    if (sql[i] === endChar) {
      return { end: i + 1, closed: true }
    }
    i += 1
  }
  return { end: sql.length, closed: false }
}

function scanNumber(sql, i) {
  if (sql[i] === '0' && (sql[i + 1] === 'x' || sql[i + 1] === 'X')) {
    i += 2
    while (i < sql.length && /[0-9a-fA-F]/.test(sql[i])) {
      i += 1
    }
    return i
  }
  while (i < sql.length && /[0-9]/.test(sql[i])) {
    i += 1
  }
  if (sql[i] === '.' && /[0-9]/.test(sql[i + 1] || '')) {
    i += 1
    while (i < sql.length && /[0-9]/.test(sql[i])) {
      i += 1
    }
  }
  return i
}

function isSpace(ch) {
  return ch === ' ' || ch === '\n' || ch === '\t' || ch === '\r' || ch === '\f'
}

function isOperator(ch) {
  return !!ch && '=<>!+-*/%|&^~$@?:\\'.includes(ch)
}

function isIdentStart(ch) {
  return /[\p{L}_]/u.test(ch)
}

function isIdentPart(ch) {
  return /[\p{L}\p{N}_]/u.test(ch)
}

function escapeHtml(text) {
  return text.replace(/[&<>"']/g, (ch) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[ch]))
}
