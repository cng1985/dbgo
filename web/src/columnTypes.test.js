import assert from 'node:assert/strict'
import test from 'node:test'
import { columnTypeFor, columnTypes, defaultColumnType } from './columnTypes.js'

test('列类型按数据库分开，互不混用', () => {
  assert.deepEqual(columnTypes('sqlite'), ['integer', 'real', 'text', 'blob', 'numeric'])
  assert.ok(columnTypes('mysql').includes('varchar(255)'))
  assert.equal(columnTypes('mysql').includes('jsonb'), false)
  assert.ok(columnTypes('postgres').includes('jsonb'))
  assert.ok(columnTypes('pgsql').includes('uuid'))
  assert.ok(columnTypes('sqlserver').includes('nvarchar(255)'))
  assert.ok(columnTypes('mssql').includes('datetime2'))
  assert.equal(columnTypes('unknown').includes('jsonb'), false)
})

test('切换数据库时保留两边都有的类型，否则回到默认类型', () => {
  assert.equal(defaultColumnType('mysql'), 'int')
  assert.equal(defaultColumnType('sqlserver'), 'int')
  assert.equal(columnTypeFor('postgres', 'varchar(255)'), 'varchar(255)')
  assert.equal(columnTypeFor('sqlite', 'varchar(255)'), 'integer')
  assert.equal(columnTypeFor('mysql', 'timestamptz'), 'int')
})
