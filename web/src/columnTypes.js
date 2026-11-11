const columnTypesByDriver = {
  sqlite: ['integer', 'real', 'text', 'blob', 'numeric'],
  mysql: [
    'tinyint',
    'smallint',
    'int',
    'bigint',
    'decimal(10,2)',
    'float',
    'double',
    'bit(1)',
    'boolean',
    'char(32)',
    'varchar(64)',
    'varchar(255)',
    'text',
    'mediumtext',
    'longtext',
    'blob',
    'date',
    'time',
    'datetime',
    'timestamp',
    'year',
    'json'
  ],
  postgres: [
    'smallint',
    'integer',
    'bigint',
    'numeric',
    'real',
    'double precision',
    'boolean',
    'char(32)',
    'varchar(255)',
    'text',
    'bytea',
    'date',
    'time',
    'timestamp',
    'timestamptz',
    'json',
    'jsonb',
    'uuid'
  ],
  sqlserver: [
    'tinyint',
    'smallint',
    'int',
    'bigint',
    'bit',
    'decimal(18,2)',
    'float',
    'real',
    'money',
    'char(32)',
    'varchar(255)',
    'nchar(32)',
    'nvarchar(255)',
    'text',
    'ntext',
    'date',
    'time',
    'datetime',
    'datetime2',
    'datetimeoffset',
    'uniqueidentifier',
    'varbinary(max)',
    'xml'
  ]
}

const defaultTypeByDriver = {
  sqlite: 'integer',
  mysql: 'int',
  postgres: 'integer',
  sqlserver: 'int'
}

export function normalizeColumnDriver(driver) {
  switch (String(driver || '').trim().toLowerCase()) {
    case 'mysql':
      return 'mysql'
    case 'postgres':
    case 'pgsql':
    case 'postgresql':
      return 'postgres'
    case 'sqlserver':
    case 'mssql':
      return 'sqlserver'
    default:
      return 'sqlite'
  }
}

export function columnTypes(driver) {
  return columnTypesByDriver[normalizeColumnDriver(driver)]
}

export function defaultColumnType(driver) {
  return defaultTypeByDriver[normalizeColumnDriver(driver)]
}

export function matchColumnType(driver, current) {
  const type = String(current || '').trim()
  const options = columnTypes(driver)
  if (!type) {
    return defaultColumnType(driver)
  }
  if (options.includes(type)) {
    return type
  }
  const folded = options.find((item) => item.toLowerCase() === type.toLowerCase())
  return folded || type
}

export function columnTypeOptions(driver, current) {
  const options = columnTypes(driver)
  const type = matchColumnType(driver, current)
  if (!type || options.includes(type)) {
    return options
  }
  return [type, ...options]
}

export function columnTypeFor(driver, current) {
  const options = columnTypes(driver)
  if (options.includes(current)) {
    return current
  }
  const folded = options.find((item) => item.toLowerCase() === String(current || '').trim().toLowerCase())
  return folded || defaultColumnType(driver)
}
