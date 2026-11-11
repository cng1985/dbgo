import assert from 'node:assert/strict'
import test from 'node:test'
import { querySource, rowKeys, updateRowSQL } from './rowEdit.js'

test('单表查询能认出模式和表', () => {
  assert.deepEqual(querySource('select * from city'), { schema: '', table: 'city' })
  assert.deepEqual(querySource("select id, name from main.city where id = 1 order by id"), { schema: 'main', table: 'city' })
  assert.deepEqual(querySource('select top 5 * from dbo.city as c'), { schema: 'dbo', table: 'city' })
  assert.deepEqual(querySource('select "from" from [my table]'), { schema: '', table: 'my table' })
  assert.deepEqual(querySource('-- note\nselect * from city;'), { schema: '', table: 'city' })
})

test('连接、子查询和多语句不能当成一张表', () => {
  assert.equal(querySource('select * from city join town on city.id = town.id'), null)
  assert.equal(querySource('select * from (select * from city) t'), null)
  assert.equal(querySource("select * from city; select * from town"), null)
  assert.equal(querySource("print '11'"), null)
  assert.deepEqual(querySource("select * from city where side = 'left' and note = 'join'"), { schema: '', table: 'city' })
})

test('优先用主键，没有主键时用结果里的全部字段', () => {
  const columns = [
    { name: 'id', primary: true },
    { name: 'name', primary: false }
  ]
  assert.deepEqual(rowKeys(columns, [], [{ name: 'id' }, { name: 'name' }]), { names: ['id'], exact: true })
  assert.deepEqual(rowKeys(columns.map((item) => ({ ...item, primary: false })), [{ unique: true, columns: 'name' }], [{ name: 'name' }]), { names: ['name'], exact: true })
  assert.deepEqual(rowKeys([{ name: 'name', primary: false }], [], [{ name: 'name' }]), { names: ['name'], exact: false })
})

test('更新语句只改动过的字段，并用原来的主键定位', () => {
  const sql = updateRowSQL('sqlite', 'main', 'city', [
    { name: 'id', type: 'integer', key: true, editable: false, original: '1', originalNull: false, value: '1', isNull: false },
    { name: 'name', type: 'text', key: false, editable: true, original: '杭州', originalNull: false, value: "杭'州", isNull: false }
  ])
  assert.equal(sql, "update main.city set name = '杭''州' where id = 1")
})

test('空值用 is null 定位，并把字段改成 NULL', () => {
  const sql = updateRowSQL('sqlite', '', 'city', [
    { name: 'id', type: 'integer', key: true, editable: false, original: '2', originalNull: false, value: '2', isNull: false },
    { name: 'name', type: 'text', key: false, editable: true, original: null, originalNull: true, value: '', isNull: false }
  ])
  assert.match(sql, /set name = ''/)
  assert.match(sql, /where id = 2/)
  const clear = updateRowSQL('sqlite', '', 'note', [
    { name: 'id', type: 'integer', key: true, editable: false, original: null, originalNull: true, value: null, isNull: true },
    { name: 'body', type: 'text', key: false, editable: true, original: 'a', originalNull: false, value: '', isNull: true }
  ])
  assert.equal(clear, 'update note set body = NULL where id is null')
})
