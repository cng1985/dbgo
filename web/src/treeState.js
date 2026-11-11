export function nodeMark(node) {
  return {
    id: node?.id || '',
    kind: node?.kind || '',
    folder: node?.folder || '',
    connectionId: node?.connection?.id != null && node.connection.id !== '' ? String(node.connection.id) : '',
    schema: node?.schema || '',
    table: node?.table || '',
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
