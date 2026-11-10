import assert from 'node:assert/strict'
import test from 'node:test'
import { commentSQL, explainSQL, filterSQL, formatSQL, insertSQL, sqlLiteral } from './sqlActions.js'

test('格式化把主要子句换行并保留字符串', () => {
  const formatted = formatSQL("select id, 'from here' from city where name = 'a'")
  assert.match(formatted, /\nfrom city/)
  assert.match(formatted, /\nwhere name/)
  assert.match(formatted, /'from here'/)
})

test('注释可以来回切换', () => {
  const once = commentSQL('select 1\nselect 2')
  assert.equal(once, '-- select 1\n-- select 2')
  assert.equal(commentSQL(once), 'select 1\nselect 2')
})

test('解释语句按驱动生成', () => {
  assert.match(explainSQL('sqlite', 'select 1'), /^explain query plan/)
  assert.match(explainSQL('postgres', 'select 1'), /^explain\n/)
})

test('右键可以生成字面量、插入和筛选', () => {
  assert.equal(sqlLiteral(null), 'NULL')
  assert.equal(sqlLiteral("a'b"), "'a''b'")
  assert.equal(insertSQL([{ name: 'id' }], ['7']), 'insert into copied_row (id) values (\'7\')')
  const filtered = filterSQL('sqlite', 'select * from city', 'name', '杭州')
  assert.match(filtered, /dbgo_filter/)
  assert.match(filtered, /where /)
  assert.match(filtered, /杭州/)
})
