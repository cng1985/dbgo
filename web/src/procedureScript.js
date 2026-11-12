import { keywordSet, quoteIdent as quoteSQLIdent } from './sqlEditor.js'

const routineWords = ['procedure', 'proc', 'function', 'view', 'user', 'comment', 'out', 'inout', 'returns', 'definer', 'algorithm', 'security']

function quoteIdent(driver, name) {
  const value = String(name || '').trim()
  if (!value) {
    return ''
  }
  if (driver === 'postgres' && value.toLowerCase() !== value) {
    return '"' + value.replaceAll('"', '""') + '"'
  }
  const words = new Set(keywordSet(driver))
  for (const word of routineWords) {
    words.add(word)
  }
  return quoteSQLIdent(driver, value, words)
}

export function qualifyProcedure(driver, schema, name) {
  const proc = quoteIdent(driver, name)
  const owner = String(schema || '').trim()
  if (!proc || !owner || (driver === 'sqlite' && owner.toLowerCase() === 'main')) {
    return proc
  }
  return quoteIdent(driver, owner) + '.' + proc
}

function skipTrivia(sql, index) {
  let i = index
  while (i < sql.length) {
    if (/\s/.test(sql[i])) {
      i += 1
      continue
    }
    if (sql.startsWith('--', i)) {
      const nl = sql.indexOf('\n', i)
      i = nl < 0 ? sql.length : nl + 1
      continue
    }
    if (sql.startsWith('/*', i)) {
      const end = sql.indexOf('*/', i + 2)
      i = end < 0 ? sql.length : end + 2
      continue
    }
    break
  }
  return i
}

function readIdent(sql, index) {
  const ch = sql[index]
  if (ch === '[') {
    let i = index + 1
    let text = ''
    while (i < sql.length) {
      if (sql[i] === ']' && sql[i + 1] === ']') {
        text += ']'
        i += 2
        continue
      }
      if (sql[i] === ']') {
        return { text, end: i + 1 }
      }
      text += sql[i]
      i += 1
    }
    return null
  }
  if (ch === '"' || ch === '`') {
    let i = index + 1
    let text = ''
    while (i < sql.length) {
      if (sql[i] === ch && sql[i + 1] === ch) {
        text += ch
        i += 2
        continue
      }
      if (sql[i] === ch) {
        return { text, end: i + 1 }
      }
      text += sql[i]
      i += 1
    }
    return null
  }
  const match = /^[\p{L}_][\p{L}\p{N}_]*/u.exec(sql.slice(index))
  if (!match) {
    return null
  }
  return { text: match[0], end: index + match[0].length }
}

function skipQuoted(sql, index) {
  const ch = sql[index]
  if (ch !== '`' && ch !== '\'' && ch !== '"') {
    return index
  }
  let i = index + 1
  while (i < sql.length) {
    if (sql[i] === ch && sql[i + 1] === ch) {
      i += 2
      continue
    }
    if (sql[i] === ch) {
      return i + 1
    }
    i += 1
  }
  return sql.length
}

function skipAccount(sql, index) {
  if (sql[index] === '`' || sql[index] === '\'' || sql[index] === '"') {
    return skipQuoted(sql, index)
  }
  const ident = readIdent(sql, index)
  if (!ident) {
    return index
  }
  if (sql[ident.end] === '(') {
    return matchParen(sql, ident.end)
  }
  return ident.end
}

function skipDefiner(sql, index) {
  if (!/^definer\b/i.test(sql.slice(index))) {
    return index
  }
  let i = skipTrivia(sql, index + 'definer'.length)
  if (sql[i] !== '=') {
    return index
  }
  i = skipAccount(sql, skipTrivia(sql, i + 1))
  const at = skipTrivia(sql, i)
  if (sql[at] === '@') {
    i = skipAccount(sql, skipTrivia(sql, at + 1))
  }
  return skipTrivia(sql, i)
}

function skipViewModifiers(sql, index) {
  let i = index
  while (i < sql.length) {
    const afterDefiner = skipDefiner(sql, i)
    if (afterDefiner !== i) {
      i = skipTrivia(sql, afterDefiner)
      continue
    }
    if (/^algorithm\s*=/i.test(sql.slice(i))) {
      const eq = sql.slice(i).indexOf('=')
      const ident = readIdent(sql, skipTrivia(sql, i + eq + 1))
      if (!ident) {
        break
      }
      i = skipTrivia(sql, ident.end)
      continue
    }
    const security = /^sql\s+security\s+(definer|invoker)\b/i.exec(sql.slice(i))
    if (security) {
      i = skipTrivia(sql, i + security[0].length)
      continue
    }
    break
  }
  return i
}

function matchHeader(sql) {
  const source = String(sql || '')
  const startPattern = /(create(?:\s+or\s+(?:alter|replace))?|alter)\b/gi
  let found
  while ((found = startPattern.exec(source))) {
    const afterCreate = skipTrivia(source, found.index + found[0].length)
    const routineAt = skipViewModifiers(source, afterCreate)
    const routine = /^(proc|procedure|function|view)\b/i.exec(source.slice(routineAt))
    if (!routine) {
      continue
    }
    let i = skipTrivia(source, routineAt + routine[0].length)
    if (/^view$/i.test(routine[0])) {
      const exists = /^if\s+not\s+exists\b/i.exec(source.slice(i))
      if (exists) {
        i = skipTrivia(source, i + exists[0].length)
      }
    }
    const nameStart = i
    const first = readIdent(source, i)
    if (!first) {
      continue
    }
    i = first.end
    let schema = ''
    let name = first.text
    const afterFirst = skipTrivia(source, i)
    if (source[afterFirst] === '.') {
      const second = readIdent(source, skipTrivia(source, afterFirst + 1))
      if (second) {
        schema = first.text
        name = second.text
        i = second.end
      }
    }
    return {
      keywordStart: found.index,
      keywordEnd: routineAt + routine[0].length,
      nameStart,
      nameEnd: i,
      schema,
      name
    }
  }
  return null
}

function matchParen(sql, start) {
  let depth = 0
  let i = start
  while (i < sql.length) {
    if (sql.startsWith('--', i)) {
      const nl = sql.indexOf('\n', i)
      i = nl < 0 ? sql.length : nl + 1
      continue
    }
    if (sql.startsWith('/*', i)) {
      const end = sql.indexOf('*/', i + 2)
      i = end < 0 ? sql.length : end + 2
      continue
    }
    const ch = sql[i]
    if (ch === '\'') {
      i += 1
      while (i < sql.length) {
        if (sql[i] === '\'' && sql[i + 1] === '\'') {
          i += 2
          continue
        }
        if (sql[i] === '\'') {
          i += 1
          break
        }
        i += 1
      }
      continue
    }
    if (ch === '(') {
      depth += 1
    } else if (ch === ')') {
      depth -= 1
      if (depth === 0) {
        return i + 1
      }
    }
    i += 1
  }
  return sql.length
}

function findBody(sql, start) {
  let depth = 0
  let i = start
  while (i < sql.length) {
    if (sql.startsWith('--', i)) {
      const nl = sql.indexOf('\n', i)
      i = nl < 0 ? sql.length : nl + 1
      continue
    }
    if (sql.startsWith('/*', i)) {
      const end = sql.indexOf('*/', i + 2)
      i = end < 0 ? sql.length : end + 2
      continue
    }
    const ch = sql[i]
    if (ch === '\'') {
      i += 1
      while (i < sql.length) {
        if (sql[i] === '\'' && sql[i + 1] === '\'') {
          i += 2
          continue
        }
        if (sql[i] === '\'') {
          i += 1
          break
        }
        i += 1
      }
      continue
    }
    if (ch === '(') {
      depth += 1
      i += 1
      continue
    }
    if (ch === ')') {
      depth = Math.max(0, depth - 1)
      i += 1
      continue
    }
    if (depth === 0) {
      const word = /^[\p{L}_][\p{L}\p{N}_]*/u.exec(sql.slice(i))
      if (word && ['as', 'begin', 'language', 'returns', 'with'].includes(word[0].toLowerCase())) {
        return i
      }
      if (word) {
        i += word[0].length
        continue
      }
    }
    i += 1
  }
  return sql.length
}

function paramRegion(sql, header) {
  const after = skipTrivia(sql, header.nameEnd)
  if (sql[after] === '(') {
    const end = matchParen(sql, after)
    if (sql[end - 1] !== ')') {
      return { unbalanced: true, start: after, end, style: 'paren', text: '' }
    }
    return { start: after, end, style: 'paren', text: sql.slice(after + 1, end - 1) }
  }
  const body = findBody(sql, header.nameEnd)
  const text = sql.slice(header.nameEnd, body)
  let depth = 0
  for (const ch of text) {
    if (ch === '(') depth += 1
    else if (ch === ')') depth = Math.max(0, depth - 1)
  }
  if (body >= sql.length && depth > 0) {
    return { unbalanced: true, start: header.nameEnd, end: body, style: 'bare', text }
  }
  return { start: header.nameEnd, end: body, style: 'bare', text }
}

function splitTop(text) {
  const parts = []
  let depth = 0
  let start = 0
  for (let i = 0; i < text.length; i += 1) {
    if (text.startsWith('--', i)) {
      const nl = text.indexOf('\n', i)
      i = nl < 0 ? text.length : nl
      continue
    }
    const ch = text[i]
    if (ch === '\'') {
      i += 1
      while (i < text.length) {
        if (text[i] === '\'' && text[i + 1] === '\'') {
          i += 2
          continue
        }
        if (text[i] === '\'') {
          break
        }
        i += 1
      }
      continue
    }
    if (ch === '(') {
      depth += 1
    } else if (ch === ')') {
      depth = Math.max(0, depth - 1)
    } else if (ch === ',' && depth === 0) {
      let end = i
      let j = i + 1
      while (j < text.length && text[j] !== '\n' && /\s/.test(text[j])) {
        j += 1
      }
      if (text.startsWith('--', j)) {
        const nl = text.indexOf('\n', j)
        end = nl < 0 ? text.length : nl
        parts.push(text.slice(start, i) + ' ' + text.slice(j, end))
        start = end + (nl < 0 ? 0 : 1)
        i = end
        continue
      }
      parts.push(text.slice(start, i))
      start = i + 1
    }
  }
  parts.push(text.slice(start))
  return parts
}

function parseParam(chunk) {
  let text = String(chunk || '').trim()
  if (!text) {
    return null
  }
  let comment = ''
  const commentAt = text.search(/\s--/)
  if (commentAt >= 0) {
    comment = text.slice(commentAt).replace(/^\s--\s?/, '').trim()
    text = text.slice(0, commentAt).trim()
  }
  let mode = 'in'
  const prefix = /^(in\s+out|inout|in|out)\s+/i.exec(text)
  if (prefix) {
    mode = prefix[1].toLowerCase().replace(/\s+/g, '')
    if (mode === 'in') {
      mode = 'in'
    }
    text = text.slice(prefix[0].length).trim()
  }
  const suffix = /\s+(output|out|inout)$/i.exec(text)
  if (suffix) {
    const word = suffix[1].toLowerCase()
    mode = word === 'inout' ? 'inout' : 'out'
    text = text.slice(0, suffix.index).trim()
  }
  let name = ''
  let type = ''
  if (text[0] === '@') {
    const ident = readIdent(text, 1)
    if (!ident) {
      return null
    }
    name = ident.text
    type = text.slice(ident.end)
  } else {
    const ident = readIdent(text, 0)
    if (!ident) {
      return null
    }
    name = ident.text
    type = text.slice(ident.end)
  }
  type = type.trim().replace(/,$/, '')
  if (!name || !type) {
    return null
  }
  return {
    name,
    type,
    mode: mode === 'in' ? 'in' : mode,
    comment
  }
}

function routineKind(kind) {
  return kind === 'function' ? 'function' : 'procedure'
}

function routineWord(kind) {
  return routineKind(kind) === 'function' ? 'FUNCTION' : 'PROCEDURE'
}

export function procedureTemplate(driver, schema, name, kind = 'procedure') {
  const routine = routineKind(kind)
  const proc = String(name || '').trim() || (kind === 'view' ? 'new_view' : (routine === 'function' ? 'new_function' : 'new_procedure'))
  const qualified = qualifyProcedure(driver, schema, proc)
  if (!qualified) {
    return ''
  }
  if (kind === 'view') {
    if (driver === 'sqlserver') {
      return `CREATE VIEW ${qualified}\nAS\nSELECT 1 AS id\n`
    }
    if (driver === 'postgres' || driver === 'mysql' || driver === 'sqlite') {
      return `CREATE VIEW ${qualified} AS\nSELECT 1 AS id\n`
    }
    return ''
  }
  const word = routineWord(routine)
  if (driver === 'sqlserver') {
    if (routine === 'function') {
      return `CREATE FUNCTION ${qualified}()\nRETURNS int\nAS\nBEGIN\n  RETURN 0\nEND\n`
    }
    return `CREATE PROCEDURE ${qualified}\nAS\nBEGIN\n  SET NOCOUNT ON;\nEND\n`
  }
  if (driver === 'postgres') {
    if (routine === 'function') {
      return `CREATE FUNCTION ${qualified}()\nRETURNS integer\nLANGUAGE plpgsql\nAS $$\nBEGIN\n  RETURN NULL;\nEND;\n$$\n`
    }
    return `CREATE PROCEDURE ${qualified}()\nLANGUAGE plpgsql\nAS $$\nBEGIN\n  NULL;\nEND;\n$$\n`
  }
  if (driver === 'mysql') {
    if (routine === 'function') {
      return `CREATE FUNCTION ${qualified}()\nRETURNS INT\nDETERMINISTIC\nBEGIN\n  RETURN 0;\nEND\n`
    }
    return `CREATE ${word} ${qualified}()\nBEGIN\nEND\n`
  }
  return ''
}

export function procedureName(sql) {
  return matchHeader(sql)?.name || ''
}

export function setProcedureIdentity(sql, driver, schema, name) {
  const header = matchHeader(sql)
  const proc = String(name || '').trim()
  if (!header || !proc) {
    return sql
  }
  const owner = String(schema || '').trim()
  if (header.name === proc && header.schema === owner) {
    return sql
  }
  const qualified = qualifyProcedure(driver, owner, proc)
  return sql.slice(0, header.nameStart) + qualified + sql.slice(header.nameEnd)
}

export function procedureParams(sql) {
  const header = matchHeader(sql)
  if (!header) {
    return []
  }
  const region = paramRegion(sql, header)
  return splitTop(region.text).map(parseParam).filter(Boolean)
}

function formatParam(driver, param, kind = 'procedure') {
  if (driver === 'sqlserver') {
    const output = kind !== 'function' && (param.mode === 'out' || param.mode === 'inout') ? ' OUTPUT' : ''
    return `@${param.name} ${param.type}${output}`
  }
  const mode = String(param.mode || 'in').toUpperCase()
  return `${mode} ${quoteIdent(driver, param.name)} ${param.type}`
}

function withComment(text, comment) {
  const note = String(comment || '').trim()
  return note ? `${text} -- ${note}` : text
}

function parensBalanced(text) {
  let depth = 0
  for (const ch of String(text || '')) {
    if (ch === '(') {
      depth += 1
    } else if (ch === ')') {
      depth -= 1
      if (depth < 0) {
        return false
      }
    }
  }
  return depth === 0
}

export function applyProcedureParams(sql, driver, params, kind = 'procedure') {
  const header = matchHeader(sql)
  if (!header) {
    return sql
  }
  const list = (params || []).filter((item) => String(item?.name || '').trim() && String(item?.type || '').trim())
  if (list.some((item) => !parensBalanced(item.type))) {
    return sql
  }
  const region = paramRegion(sql, header)
  if (region.unbalanced) {
    return sql
  }
  let formatted = ''
  if (region.style === 'paren') {
    if (!list.length) {
      formatted = '()'
    } else if (list.some((item) => String(item.comment || '').trim())) {
      const lines = list.map((item, index) => {
        const comma = index < list.length - 1 ? ',' : ''
        return '  ' + withComment(formatParam(driver, item, kind) + comma, item.comment)
      })
      formatted = '(\n' + lines.join('\n') + '\n)'
    } else {
      formatted = '(' + list.map((item) => formatParam(driver, item, kind)).join(', ') + ')'
    }
  } else if (list.length) {
    formatted = '\n' + list.map((item, index) => {
      const comma = index < list.length - 1 ? ',' : ''
      return '  ' + withComment(formatParam(driver, item, kind) + comma, item.comment)
    }).join('\n') + '\n'
  } else {
    formatted = '\n'
  }
  return sql.slice(0, region.start) + formatted + sql.slice(region.end)
}

function rewriteExistingView(driver, script, header, keyword, options) {
  const replaced = /\bor\s+(?:alter|replace)\b/.test(keyword)
  const created = keyword.startsWith('create')
  const between = script.slice(header.keywordEnd, header.nameStart)
  const ifNotExists = /\bif\s+not\s+exists\b/i.test(between)
  const dropped = /\bdrop\s+view\b/i.test(script.slice(0, header.keywordStart))
  if ((driver === 'sqlite' || ifNotExists) && !dropped) {
    const qualified = qualifyProcedure(driver, options.schema, options.name || header.name)
    if (qualified) {
      return `DROP VIEW IF EXISTS ${qualified};\n${script}`
    }
  }
  if (!created || replaced || ifNotExists) {
    return script
  }
  if (driver === 'sqlserver' && keyword === 'create view') {
    return script.slice(0, header.keywordStart) + 'CREATE OR ALTER VIEW' + script.slice(header.keywordEnd)
  }
  if (driver === 'postgres' || driver === 'mysql') {
    return script.slice(0, header.keywordStart) + 'CREATE OR REPLACE' + script.slice(header.keywordStart + 'create'.length)
  }
  return script
}

export function procedureSaveSQL(driver, sql, options = {}) {
  let script = String(sql || '').replace(/\s+$/, '')
  if (!script) {
    return ''
  }
  if (options.existing) {
    const header = matchHeader(script)
    if (header) {
      const keyword = script.slice(header.keywordStart, header.keywordEnd).toLowerCase().replace(/\s+/g, ' ')
      if (/\bview$/.test(keyword)) {
        script = rewriteExistingView(driver, script, header, keyword, options)
        return script + '\n'
      }
      const routine = /\bfunction$/.test(keyword)
        ? 'function'
        : (/\bproc(?:edure)?$/.test(keyword) ? 'procedure' : routineKind(options.kind))
      const word = routineWord(routine)
      const created = routine === 'function'
        ? keyword === 'create function'
        : (keyword === 'create procedure' || keyword === 'create proc')
      if (driver === 'sqlserver' && created) {
        script = script.slice(0, header.keywordStart) + `CREATE OR ALTER ${word}` + script.slice(header.keywordEnd)
      } else if (driver === 'postgres' && created) {
        script = script.slice(0, header.keywordStart) + `CREATE OR REPLACE ${word}` + script.slice(header.keywordEnd)
      } else if (driver === 'mysql' && !new RegExp(`\\bdrop\\s+${routine}\\b`, 'i').test(script.slice(0, header.keywordStart))) {
        const qualified = qualifyProcedure(driver, options.schema, options.name || procedureName(script))
        script = `DROP ${word} IF EXISTS ${qualified};\n${script}`
      }
    }
  }
  return script + '\n'
}

export function appendViewTriggers(script, triggers) {
  const list = (triggers || []).map((item) => String(item || '').trim()).filter(Boolean)
  if (!list.length) {
    return script
  }
  let text = String(script || '').replace(/\s+$/, '')
  if (text && !text.endsWith(';')) {
    text += ';'
  }
  for (const item of list) {
    text += '\n' + (item.endsWith(';') ? item : item + ';')
  }
  return text + '\n'
}

function sqlString(value, unicode) {
  const text = String(value ?? '').replaceAll('\'', '\'\'')
  return (unicode ? 'N' : '') + `'${text}'`
}

function viewCommentSQL(driver, schema, name, description) {
  const view = String(name || '').trim()
  if (!view) {
    return ''
  }
  if (driver === 'sqlserver') {
    const owner = sqlString(schema || 'dbo', true)
    const objectName = sqlString(view, true)
    const text = sqlString(description || '', true)
    return `BEGIN
  IF EXISTS (
    SELECT 1
    FROM sys.extended_properties ep
    INNER JOIN sys.objects o ON o.object_id = ep.major_id AND ep.minor_id = 0
    INNER JOIN sys.schemas s ON s.schema_id = o.schema_id
    WHERE ep.name = N'MS_Description' AND s.name = ${owner} AND o.name = ${objectName}
  )
    EXEC sys.sp_updateextendedproperty @name = N'MS_Description', @value = ${text}, @level0type = N'SCHEMA', @level0name = ${owner}, @level1type = N'VIEW', @level1name = ${objectName};
  ELSE
    EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = ${text}, @level0type = N'SCHEMA', @level0name = ${owner}, @level1type = N'VIEW', @level1name = ${objectName};
END`
  }
  if (driver === 'postgres') {
    return `COMMENT ON VIEW ${qualifyProcedure(driver, schema, view)} IS ${sqlString(description || '', false)}`
  }
  return ''
}

export function procedureCommentSQL(driver, schema, name, description, params = [], kind = 'procedure') {
  if (kind === 'view') {
    return viewCommentSQL(driver, schema, name, description)
  }
  const proc = String(name || '').trim()
  if (!proc) {
    return ''
  }
  const word = routineWord(kind)
  if (driver === 'sqlserver') {
    const owner = sqlString(schema || 'dbo', true)
    const objectName = sqlString(proc, true)
    const text = sqlString(description || '', true)
    return `BEGIN
  IF EXISTS (
    SELECT 1
    FROM sys.extended_properties ep
    INNER JOIN sys.objects o ON o.object_id = ep.major_id AND ep.minor_id = 0
    INNER JOIN sys.schemas s ON s.schema_id = o.schema_id
    WHERE ep.name = N'MS_Description' AND s.name = ${owner} AND o.name = ${objectName}
  )
    EXEC sys.sp_updateextendedproperty @name = N'MS_Description', @value = ${text}, @level0type = N'SCHEMA', @level0name = ${owner}, @level1type = N'${word}', @level1name = ${objectName};
  ELSE
    EXEC sys.sp_addextendedproperty @name = N'MS_Description', @value = ${text}, @level0type = N'SCHEMA', @level0name = ${owner}, @level1type = N'${word}', @level1name = ${objectName};
END`
  }
  if (driver === 'mysql') {
    return `ALTER ${word} ${qualifyProcedure(driver, schema, proc)} COMMENT ${sqlString(description || '', false)}`
  }
  if (driver === 'postgres') {
    const types = (params || []).map((item) => String(item.type || '').trim()).filter(Boolean).join(', ')
    return `COMMENT ON ${word} ${qualifyProcedure(driver, schema, proc)}(${types}) IS ${sqlString(description || '', false)}`
  }
  return ''
}
