import assert from 'node:assert/strict'
import test from 'node:test'
import { applyTabClose } from './tabClose.js'

const tabs = [{ id: 'a' }, { id: 'b' }, { id: 'c' }, { id: 'd' }]

test('关闭左边只留下目标及其右侧', () => {
  const next = applyTabClose(tabs, 'd', 'c', 'left')
  assert.deepEqual(next.tabs.map((tab) => tab.id), ['c', 'd'])
  assert.equal(next.activeId, 'd')
})

test('关闭右边只留下目标及其左侧', () => {
  const next = applyTabClose(tabs, 'a', 'b', 'right')
  assert.deepEqual(next.tabs.map((tab) => tab.id), ['a', 'b'])
  assert.equal(next.activeId, 'a')
})

test('关闭其他窗口只留右键的那个', () => {
  const next = applyTabClose(tabs, 'a', 'c', 'others')
  assert.deepEqual(next.tabs.map((tab) => tab.id), ['c'])
  assert.equal(next.activeId, 'c')
})

test('关闭全部后没有窗口', () => {
  const next = applyTabClose(tabs, 'b', 'b', 'all')
  assert.deepEqual(next.tabs, [])
  assert.equal(next.activeId, '')
})

test('关闭当前窗口时激活相邻窗口', () => {
  const next = applyTabClose(tabs, 'b', 'b', 'one')
  assert.deepEqual(next.tabs.map((tab) => tab.id), ['a', 'c', 'd'])
  assert.equal(next.activeId, 'c')
})

test('关掉的窗口不换对象，留下的还是原来的页签', () => {
  const next = applyTabClose(tabs, 'b', 'c', 'left')
  assert.equal(next.tabs[0], tabs[2])
})
