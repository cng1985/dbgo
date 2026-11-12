import assert from 'node:assert/strict'
import test from 'node:test'
import { sqlErrorLine } from './sqlError.js'

test('sqlErrorLine reads the leading line number', () => {
  assert.equal(sqlErrorLine('第 2 行：SQL 执行失败：no such column: bad'), 2)
  assert.equal(sqlErrorLine('第12行：SQL 执行失败：只读模式只允许查询语句'), 12)
})

test('sqlErrorLine ignores messages without a line', () => {
  assert.equal(sqlErrorLine('SQL 执行失败：连接已关闭'), 0)
  assert.equal(sqlErrorLine(''), 0)
  assert.equal(sqlErrorLine('说明里提到第 3 行，但不是开头'), 0)
})
