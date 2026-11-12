export function nodeMark(node) {
  return {
    id: node?.id || '',
    kind: node?.kind || '',
    folder: node?.folder || '',
    connectionId: node?.connection?.id != null && node.connection.id !== '' ? String(node.connection.id) : '',
    schema: node?.schema || '',
    table: node?.table || '',
    object: node?.object || '',
    label: node?.label || ''
  }
}

export function retargetMark(mark, change) {
  if (!mark || !change?.connectionId || !change.previousName || !change.name) {
    return mark
  }
  if (String(mark.connectionId) !== String(change.connectionId)) {
    return mark
  }
  if ((mark.schema || '') !== (change.previousSchema || '')) {
    return mark
  }
  if (change.kind === 'procedure' || change.kind === 'function') {
    if (mark.kind !== change.kind) {
      return mark
    }
    if ((mark.object || mark.label || '') !== change.previousName) {
      return mark
    }
    const schema = change.schema || ''
    return {
      ...mark,
      schema,
      object: change.name,
      label: change.name,
      id: `r-${mark.connectionId}-${schema}-${change.kind}-${change.name}`
    }
  }
  if ((mark.table || '') !== change.previousName) {
    return mark
  }
  return {
    ...mark,
    schema: change.schema || '',
    table: change.name
  }
}

function samePlace(node, mark) {
  return !!mark.connectionId
    && !!node.connection
    && String(node.connection.id) === String(mark.connectionId)
    && (node.schema || '') === (mark.schema || '')
    && (node.table || '') === (mark.table || '')
    && (node.kind || '') === (mark.kind || '')
    && (node.folder || '') === (mark.folder || '')
}

export function markMatches(node, mark) {
  if (!node || !mark) {
    return false
  }
  if (mark.id && node.id === mark.id && samePlace(node, mark)) {
    return true
  }
  if (mark.id && node.id === mark.id && !mark.connectionId) {
    return true
  }
  if (!samePlace(node, mark)) {
    return false
  }
  if (mark.folder) {
    return true
  }
  if (mark.kind === 'table' || mark.kind === 'view' || mark.kind === 'schema' || mark.kind === 'connection') {
    return true
  }
  return !!mark.label && (node.label || '') === mark.label
}

function mentionsTable(mark, connectionId, schema, tableName) {
  if (!mark || String(mark.connectionId) !== String(connectionId)) {
    return false
  }
  if ((mark.schema || '') !== (schema || '')) {
    return false
  }
  if ((mark.table || '') !== (tableName || '')) {
    return false
  }
  if (mark.folder === 'columns' || mark.folder === 'indexes') {
    return true
  }
  return mark.kind === 'table' || mark.kind === 'view' || mark.kind === 'column' || mark.kind === 'key' || mark.kind === 'index'
}

function isTablesFolder(mark, connectionId, schema) {
  return !!mark
    && mark.folder === 'tables'
    && String(mark.connectionId) === String(connectionId)
    && (mark.schema || '') === (schema || '')
}

function isRoutineFolder(mark, connectionId, schema, kind) {
  const folder = kind === 'function' ? 'functions' : 'procedures'
  return !!mark
    && mark.folder === folder
    && String(mark.connectionId) === String(connectionId)
    && (mark.schema || '') === (schema || '')
}

function mentionsRoutine(mark, connectionId, schema, name, kind) {
  if (!mark || String(mark.connectionId) !== String(connectionId) || mark.kind !== kind) {
    return false
  }
  if ((mark.schema || '') !== (schema || '')) {
    return false
  }
  return (mark.object || mark.label || '') === (name || '')
}

export function planTreeRefresh(openMarks, selectedMark, change) {
  const source = Array.isArray(openMarks) ? openMarks : []
  const open = source.map((mark) => retargetMark(mark, change))
  const selected = selectedMark ? retargetMark(selectedMark, change) : null
  if (!change?.connectionId || !change.previousName || !change.name) {
    return { open, selected }
  }
  const previousSchema = change.previousSchema || ''
  const nextSchema = change.schema || ''
  if (previousSchema === nextSchema) {
    return { open, selected }
  }
  const connectionId = String(change.connectionId)
  if (change.kind === 'procedure' || change.kind === 'function') {
    const folder = change.kind === 'function' ? 'functions' : 'procedures'
    const folderId = change.kind === 'function' ? 'funcs' : 'procs'
    const shown = open.some((mark) => mentionsRoutine(mark, connectionId, nextSchema, change.name, change.kind))
      || mentionsRoutine(selected, connectionId, nextSchema, change.name, change.kind)
    const listShown = source.some((mark) => isRoutineFolder(mark, connectionId, previousSchema, change.kind))
    if (!shown && !listShown) {
      return { open, selected }
    }
    open.push(
      { id: `c-${connectionId}`, kind: 'connection', folder: '', connectionId, schema: '', table: '', object: '', label: '' },
      { id: `dbs-${connectionId}`, kind: 'folder', folder: 'databases', connectionId, schema: '', table: '', object: '', label: '数据库' },
      { id: `s-${connectionId}-${nextSchema}`, kind: 'schema', folder: '', connectionId, schema: nextSchema, table: '', object: '', label: nextSchema },
      { id: `${folderId}-${connectionId}-${nextSchema}`, kind: 'folder', folder, connectionId, schema: nextSchema, table: '', object: '', label: folder === 'functions' ? '函数' : '存储过程' }
    )
    return { open, selected }
  }
  const tableShown = open.some((mark) => mentionsTable(mark, connectionId, nextSchema, change.name))
    || mentionsTable(selected, connectionId, nextSchema, change.name)
  const listShown = source.some((mark) => isTablesFolder(mark, connectionId, previousSchema))
  if (!tableShown && !listShown) {
    return { open, selected }
  }
  open.push(
    { id: `c-${connectionId}`, kind: 'connection', folder: '', connectionId, schema: '', table: '', label: '' },
    { id: `dbs-${connectionId}`, kind: 'folder', folder: 'databases', connectionId, schema: '', table: '', label: '数据库' },
    { id: `s-${connectionId}-${nextSchema}`, kind: 'schema', folder: '', connectionId, schema: nextSchema, table: '', label: nextSchema },
    { id: `tables-${connectionId}-${nextSchema}`, kind: 'folder', folder: 'tables', connectionId, schema: nextSchema, table: '', label: '表' }
  )
  return { open, selected }
}

export function mergeTreeChildren(previous, next) {
  const oldById = new Map()
  for (const node of previous || []) {
    if (node?.id) {
      oldById.set(node.id, node)
    }
  }
  const merged = []
  for (const item of next || []) {
    const old = item?.id ? oldById.get(item.id) : null
    if (!old) {
      merged.push(item)
      continue
    }
    old.label = item.label
    old.meta = item.meta || ''
    old.kind = item.kind
    old.schema = item.schema || ''
    old.table = item.table || ''
    old.object = item.object || ''
    old.folder = item.folder || ''
    old.leaf = !!item.leaf
    if (item.connection) {
      old.connection = item.connection
    }
    if (typeof item.load === 'function') {
      old.load = item.load
    }
    if (item.open) {
      old.open = true
    }
    if (!old.load && Array.isArray(item.children)) {
      old.children = mergeTreeChildren(old.children || [], item.children)
      old.loaded = true
    }
    merged.push(old)
  }
  return merged
}
