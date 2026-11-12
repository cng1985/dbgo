export function defaultSchemaName(driver, databaseName = '') {
  switch (String(driver || '').toLowerCase()) {
    case 'sqlserver':
    case 'mssql':
      return 'dbo'
    case 'postgres':
    case 'postgresql':
    case 'pgsql':
      return 'public'
    case 'sqlite':
      return 'main'
    case 'mysql':
      return String(databaseName || '')
    default:
      return ''
  }
}

export function schemaPlaceholder(driver, databaseName = '') {
  const name = defaultSchemaName(driver, databaseName)
  return name ? `默认 ${name}` : '模式，可空'
}

export function pickSchema(driver, schemas, current = '', databaseName = '') {
  const names = (schemas || []).map((item) => (typeof item === 'string' ? item : item?.name)).filter(Boolean)
  if (current && names.includes(current)) {
    return current
  }
  const preferred = defaultSchemaName(driver, databaseName)
  const found = names.find((name) => name.toLowerCase() === preferred.toLowerCase())
  return found || names[0] || ''
}

export function filterTables(tables, keyword) {
  const query = String(keyword || '').trim().toLowerCase()
  if (!query) {
    return tables || []
  }
  return (tables || []).filter((item) => String(item?.name || '').toLowerCase().includes(query))
}
