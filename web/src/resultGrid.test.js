import assert from 'node:assert/strict'
import test from 'node:test'
import { cellPreview, cellPreviewLimit, cycleSort, formatCopiedCells, sortResultRows } from './resultGrid.js'

test('cellPreview keeps short values and clips long ones', () => {
  assert.deepEqual(cellPreview(null), { text: 'NULL', long: false })
  assert.deepEqual(cellPreview('城市'), { text: '城市', long: false })
  const long = '甲'.repeat(cellPreviewLimit + 1)
  const preview = cellPreview(long)
  assert.equal(preview.long, true)
  assert.equal(Array.from(preview.text).length, cellPreviewLimit + 1)
  assert.equal(preview.text.endsWith('…'), true)
  assert.equal(preview.text.includes(long), false)
})

test('sortResultRows orders numbers and keeps nulls last', () => {
  const rows = [['10', 'b'], ['2', null], ['1', 'a'], [null, 'a']]
  const ascending = sortResultRows(rows, 0, 'asc').map((item) => item.row[0])
  assert.deepEqual(ascending, ['1', '2', '10', null])
  const descending = sortResultRows(rows, 1, 'desc').map((item) => item.row[1])
  assert.deepEqual(descending, ['b', 'a', 'a', null])
  assert.deepEqual(rows.map((row) => row[0]), ['10', '2', '1', null])
})

test('sortResultRows keeps precision past the float limit', () => {
  const rows = [['-9007199254740992'], ['-9007199254740993'], ['9007199254740993'], ['9007199254740992']]
  assert.deepEqual(sortResultRows(rows, 0, 'asc').map((item) => item.row[0]), [
    '-9007199254740993',
    '-9007199254740992',
    '9007199254740992',
    '9007199254740993'
  ])
  const decimals = [['1.0000000000000002'], ['1.00000000000000019']]
  assert.deepEqual(sortResultRows(decimals, 0, 'asc').map((item) => item.row[0]), [
    '1.00000000000000019',
    '1.0000000000000002'
  ])
  const same = sortResultRows([['1.0'], ['1'], ['01.00']], 0, 'asc')
  assert.deepEqual(same.map((item) => item.index), [0, 1, 2])
})

test('sortResultRows keeps the original row number', () => {
  const sorted = sortResultRows([['b'], ['a']], 0, 'asc')
  assert.deepEqual(sorted.map((item) => item.index), [1, 0])
})

test('cycleSort walks through ascending, descending, and the original order', () => {
  assert.deepEqual(cycleSort(null, 1), { column: 1, direction: 'asc' })
  assert.deepEqual(cycleSort({ column: 1, direction: 'asc' }, 1), { column: 1, direction: 'desc' })
  assert.equal(cycleSort({ column: 1, direction: 'desc' }, 1), null)
  assert.deepEqual(cycleSort({ column: 1, direction: 'desc' }, 2), { column: 2, direction: 'asc' })
})

test('formatCopiedCells uses the full cell value', () => {
  const full = '甲'.repeat(cellPreviewLimit + 5)
  assert.equal(formatCopiedCells([
    { row: 0, value: full },
    { row: 0, value: null },
    { row: 3, value: '下一行' }
  ]), `${full}\tNULL\n下一行`)
})
