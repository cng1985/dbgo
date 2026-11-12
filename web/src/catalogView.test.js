import assert from 'node:assert/strict'
import test from 'node:test'
import { defaultSchemaName, filterTables, nextAutoSchema, pickSchema, schemaPlaceholder } from './catalogView.js'

test('SQL Server 默认 dbo，PostgreSQL 默认 public', () => {
  assert.equal(defaultSchemaName('sqlserver'), 'dbo')
  assert.equal(defaultSchemaName('pgsql'), 'public')
  assert.equal(defaultSchemaName('postgres'), 'public')
  assert.equal(defaultSchemaName('sqlite'), 'main')
  assert.equal(schemaPlaceholder('sqlserver'), '默认 dbo')
  assert.equal(schemaPlaceholder('postgres'), '默认 public')
})

test('已有模式优先，否则落到驱动的默认模式', () => {
  const schemas = [{ name: 'db_owner' }, { name: 'dbo' }, { name: 'sales' }]
  assert.equal(pickSchema('sqlserver', schemas, ''), 'dbo')
  assert.equal(pickSchema('sqlserver', schemas, 'sales'), 'sales')
  assert.equal(pickSchema('postgres', [{ name: 'app' }, { name: 'public' }], 'missing'), 'public')
  assert.equal(pickSchema('mysql', [{ name: 'other' }], '', 'shop'), 'other')
  assert.equal(pickSchema('mysql', [{ name: 'shop' }, { name: 'other' }], '', 'shop'), 'shop')
})

test('自动填入的模式会跟着连接变，手填的模式保持不变', () => {
  const shop = nextAutoSchema('', '', 'mysql', 'shop', false)
  assert.deepEqual(shop, { schema: 'shop', autoSchema: 'shop' })
  const other = nextAutoSchema(shop.schema, shop.autoSchema, 'mysql', 'other', false)
  assert.deepEqual(other, { schema: 'other', autoSchema: 'other' })
  assert.deepEqual(nextAutoSchema('sales', 'shop', 'mysql', 'other', false), { schema: 'sales', autoSchema: 'shop' })
  assert.deepEqual(nextAutoSchema('shop', 'shop', 'mysql', 'other', true), { schema: 'shop', autoSchema: 'shop' })
  assert.deepEqual(nextAutoSchema('main', 'main', 'postgres', '', false), { schema: 'public', autoSchema: 'public' })
})

test('按表名过滤，忽略大小写和首尾空白', () => {
  const tables = [{ name: 'Customer' }, { name: 'order' }, { name: 'city' }]
  assert.deepEqual(filterTables(tables, '  CUST ').map((item) => item.name), ['Customer'])
  assert.equal(filterTables(tables, '').length, 3)
  assert.equal(filterTables(tables, 'missing').length, 0)
})
