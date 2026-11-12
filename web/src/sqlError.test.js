import assert from 'node:assert/strict'
import test from 'node:test'
import { editorErrorLine, lineNumberAt, normalizeNewlines, sqlErrorLine, translateErrorLine } from './sqlError.js'

test('normalizeNewlines turns carriage returns into editor lines', () => {
  assert.equal(normalizeNewlines('select 1;\r\nselect 2;\rselect 3;'), 'select 1;\nselect 2;\nselect 3;')
})

test('sqlErrorLine reads the leading line number', () => {
  assert.equal(sqlErrorLine('第 2 行：SQL 执行失败：no such column: bad'), 2)
  assert.equal(sqlErrorLine('第12行：SQL 执行失败：只读模式只允许查询语句'), 12)
})

test('editorErrorLine shifts a selection back into the full script', () => {
  assert.equal(editorErrorLine('第 1 行：SQL 执行失败：no such table', 10), 10)
  assert.equal(editorErrorLine('第 2 行：SQL 执行失败：no such table', 10), 11)
  assert.equal(editorErrorLine('第 2 行：SQL 执行失败：no such table', 1), 2)
  assert.equal(editorErrorLine('', 10), 0)
})

test('translateErrorLine rewrites only the leading line', () => {
  assert.equal(
    translateErrorLine('第 1 行：SQL 执行失败：no such table', 10),
    '第 10 行：SQL 执行失败：no such table'
  )
  assert.equal(translateErrorLine('第 2 行：SQL 执行失败：bad', 1), '第 2 行：SQL 执行失败：bad')
  assert.equal(translateErrorLine('连接失败', 10), '连接失败')
})

test('lineNumberAt counts from the selection start', () => {
  const sql = 'select 1;\nselect missing;'
  assert.equal(lineNumberAt(sql, 0), 1)
  assert.equal(lineNumberAt(sql, sql.indexOf('select missing')), 2)
})

test('sqlErrorLine ignores messages without a line', () => {
  assert.equal(sqlErrorLine('SQL 执行失败：连接已关闭'), 0)
  assert.equal(sqlErrorLine(''), 0)
  assert.equal(sqlErrorLine('说明里提到第 3 行，但不是开头'), 0)
})
