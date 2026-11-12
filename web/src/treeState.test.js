import assert from 'node:assert/strict'
import test from 'node:test'
import { markMatches, mergeTreeChildren, planTreeRefresh, retargetMark } from './treeState.js'

const change = {
  connectionId: 7,
  previousSchema: 'main',
  previousName: 'city',
  schema: 'main',
  name: 'cities'
}

function place(id, extra) {
  return {
    id,
    connectionId: '7',
    schema: 'main',
    table: '',
    folder: '',
    kind: 'folder',
    label: '',
    ...extra
  }
}

test('重命名表时只改这张表的标记，同前缀的表不动', () => {
  const city = place('t-7-main-city', { kind: 'table', table: 'city', label: 'city' })
  const cityColumn = place('col-7-main-city-id', { kind: 'column', table: 'city', label: 'id' })
  const prefixed = place('t-7-main-city-extra', { kind: 'table', table: 'city-extra', label: 'city-extra' })
  const otherConnection = place('t-8-main-city', { kind: 'table', connectionId: '8', table: 'city', label: 'city' })
  assert.equal(retargetMark(city, change).table, 'cities')
  assert.equal(retargetMark(city, change).id, 't-7-main-city')
  assert.equal(retargetMark(cityColumn, change).table, 'cities')
  assert.equal(retargetMark(prefixed, change).table, 'city-extra')
  assert.equal(retargetMark(otherConnection, change).table, 'city')
  assert.equal(retargetMark(place('tables-7-main', { folder: 'tables' }), change).table, '')
})

test('同模式改名后保留展开标记，不额外撑开已折叠的库', () => {
  const open = [
    place('c-7', { kind: 'connection' }),
    place('s-7-main', { kind: 'schema', label: 'main' }),
    place('tables-7-main', { folder: 'tables', label: '表' }),
    place('t-7-main-city', { kind: 'table', table: 'city', label: 'city' }),
    place('cols-7-main-city', { folder: 'columns', table: 'city', label: '列' })
  ]
  const selected = place('col-7-main-city-name', { kind: 'column', table: 'city', label: 'name' })
  const plan = planTreeRefresh(open, selected, change)
  assert.equal(plan.selected.table, 'cities')
  assert.equal(plan.selected.label, 'name')
  assert.equal(plan.open.find((item) => item.kind === 'table').table, 'cities')
  assert.equal(plan.open.find((item) => item.folder === 'columns').table, 'cities')
  assert.equal(plan.open.some((item) => item.id === 's-7-other'), false)
})

test('表换了模式时，原来看得见的表列表会在新模式下保持展开', () => {
  const moved = { ...change, schema: 'archive' }
  const open = [
    place('c-7', { kind: 'connection' }),
    place('s-7-main', { kind: 'schema', label: 'main' }),
    place('tables-7-main', { folder: 'tables', label: '表' })
  ]
  const plan = planTreeRefresh(open, null, moved)
  assert.ok(plan.open.some((item) => item.kind === 'schema' && item.schema === 'archive'))
  assert.ok(plan.open.some((item) => item.folder === 'tables' && item.schema === 'archive'))
  assert.ok(plan.open.some((item) => item.id === 's-7-main'))
})

test('没展开过的表换模式时，不主动打开新模式', () => {
  const moved = { ...change, schema: 'archive' }
  const open = [place('c-7', { kind: 'connection' })]
  const plan = planTreeRefresh(open, null, moved)
  assert.equal(plan.open.some((item) => item.schema === 'archive'), false)
  assert.equal(plan.open.length, 1)
})

test('按位置找回改名后的表，不把前缀相同的表认成同一张', () => {
  const connection = { id: '7' }
  const renamed = {
    id: 't-7-main-cities',
    kind: 'table',
    connection,
    schema: 'main',
    table: 'cities',
    label: 'cities'
  }
  const prefixed = {
    id: 't-7-main-city-extra',
    kind: 'table',
    connection,
    schema: 'main',
    table: 'city-extra',
    label: 'city-extra'
  }
  const mark = retargetMark(place('t-7-main-city', { kind: 'table', table: 'city', label: 'city' }), change)
  assert.equal(markMatches(renamed, mark), true)
  assert.equal(markMatches(prefixed, mark), false)
  assert.equal(markMatches({ id: 't-7-main-city', kind: 'table', connection, schema: 'main', table: 'city', label: 'city' }, mark), false)
})

test('存储过程改名后仍指向原来的过程，同名表不受影响', () => {
  const changeProc = {
    connectionId: '7',
    previousSchema: 'dbo',
    previousName: 'save_order',
    schema: 'dbo',
    name: 'save_box',
    kind: 'procedure'
  }
  const proc = place('r-7-dbo-procedure-save_order', { kind: 'procedure', object: 'save_order', label: 'save_order', schema: 'dbo' })
  const table = place('t-7-dbo-save_order', { kind: 'table', table: 'save_order', label: 'save_order', schema: 'dbo' })
  const moved = retargetMark(proc, changeProc)
  assert.equal(moved.object, 'save_box')
  assert.equal(moved.id, 'r-7-dbo-procedure-save_box')
  assert.equal(retargetMark(table, changeProc).table, 'save_order')
  const plan = planTreeRefresh([proc], proc, changeProc)
  assert.equal(plan.selected.label, 'save_box')
  assert.equal(plan.open.length, 1)
})

test('存储过程换模式时展开新的过程列表', () => {
  const moved = {
    connectionId: '7',
    previousSchema: 'dbo',
    previousName: 'save_order',
    schema: 'sales',
    name: 'save_order',
    kind: 'procedure'
  }
  const open = [
    place('procs-7-dbo', { folder: 'procedures', schema: 'dbo', label: '存储过程' })
  ]
  const plan = planTreeRefresh(open, null, moved)
  assert.ok(plan.open.some((item) => item.folder === 'procedures' && item.schema === 'sales' && item.id === 'procs-7-sales'))
  assert.ok(plan.open.some((item) => item.folder === 'procedures' && item.schema === 'dbo'))
})

test('刷新导航时保留已展开的子节点，并换上新的过程名', () => {
  const column = { id: 'col-id', kind: 'column', label: 'id', leaf: true }
  const columns = { id: 'cols', kind: 'folder', folder: 'columns', loaded: true, children: [column] }
  const table = { id: 't-city', kind: 'table', label: 'city', loaded: true, open: true, load() {}, children: [columns] }
  const previous = [
    { id: 'tables', kind: 'folder', folder: 'tables', loaded: true, children: [table] },
    { id: 'procs', kind: 'folder', folder: 'procedures', loaded: true, children: [
      { id: 'r-old', kind: 'procedure', label: 'save_order', object: 'save_order', leaf: true }
    ] }
  ]
  const next = [
    { id: 'tables', kind: 'folder', folder: 'tables', loaded: true, children: [
      { id: 't-city', kind: 'table', label: 'city', loaded: false, open: false, children: [] },
      { id: 't-new', kind: 'table', label: 'order_line', loaded: false, children: [] }
    ] },
    { id: 'procs', kind: 'folder', folder: 'procedures', loaded: true, children: [
      { id: 'r-save', kind: 'procedure', label: 'save_box', object: 'save_box', leaf: true }
    ] }
  ]
  const merged = mergeTreeChildren(previous, next)
  assert.equal(merged[0].children[0], table)
  assert.equal(merged[0].children[0].children[0].children[0].label, 'id')
  assert.equal(merged[0].children[1].id, 't-new')
  assert.equal(merged[1].children.length, 1)
  assert.equal(merged[1].children[0].label, 'save_box')
  assert.equal(merged[1], previous[1])
})
