import assert from 'node:assert/strict'
import test from 'node:test'
import { collectPages, nodeShown } from './navTree.js'

function connection(label, meta, extra = {}) {
  return {
    kind: 'connection',
    label,
    meta,
    loaded: false,
    leaf: false,
    children: [],
    ...extra
  }
}

test('未展开的连接按名称或地址过滤，其余连接隐藏', () => {
  const order = connection('订单库', 'orders.primary.internal.very-long-example-domain.example:3306')
  const stock = connection('库存库', 'inventory.primary.internal.very-long-example-domain.example:3306')
  assert.equal(nodeShown(order, '订单'), true)
  assert.equal(nodeShown(stock, '订单'), false)
  assert.equal(nodeShown(order, 'VERY-LONG-EXAMPLE-DOMAIN'), true)
  assert.equal(nodeShown(stock, '  库存 '), true)
  assert.equal(nodeShown(order, ''), true)
  assert.equal(nodeShown(stock, '不存在'), false)
})

test('已展开的连接会因下级对象名称命中而保留', () => {
  const node = connection('库存库', '10.0.0.8:3306', {
    loaded: true,
    children: [
      {
        kind: 'schema',
        label: 'main',
        loaded: true,
        leaf: false,
        children: [{ kind: 'table', label: 'orders', leaf: true, loaded: true, children: [] }]
      }
    ]
  })
  assert.equal(nodeShown(node, 'orders'), true)
  assert.equal(nodeShown(node, 'missing'), false)
})

test('未加载的模式不会把不相关的连接留在过滤结果里', () => {
  const schema = { kind: 'schema', label: 'dbo', loaded: false, leaf: false, children: [] }
  assert.equal(nodeShown(schema, 'orders'), false)
  assert.equal(nodeShown(schema, 'dbo'), true)
  assert.equal(nodeShown(schema, 'orders', true), true)
  const expanded = connection('库存库', '10.0.0.8:3306', {
    loaded: true,
    children: [{
      kind: 'folder',
      label: '数据库',
      meta: '1',
      loaded: true,
      leaf: false,
      children: [schema]
    }]
  })
  assert.equal(nodeShown(expanded, '订单'), false)
})

test('分页把超过一页的连接全部取回', async () => {
  const all = Array.from({ length: 250 }, (_, index) => ({ id: index + 1, name: `库${index + 1}` }))
  const rows = await collectPages(async (page, pageSize) => {
    const start = (page - 1) * pageSize
    return { data: all.slice(start, start + pageSize), total: all.length }
  })
  assert.equal(rows.length, 250)
  assert.equal(rows[0].name, '库1')
  assert.equal(rows[249].name, '库250')
  const empty = await collectPages(async () => ({ data: [], total: 0 }))
  assert.deepEqual(empty, [])
})
