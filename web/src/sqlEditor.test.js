import assert from 'node:assert/strict'
import test from 'node:test'
import { highlightSQL, quoteIdent, suggestSQL } from './sqlEditor.js'

const tables = [
  {
    schema: 'main',
    name: 'city',
    type: 'table',
    columns: [
      { name: 'id', type: 'INTEGER' },
      { name: 'name', type: 'TEXT' },
      { name: 'population', type: 'INTEGER' }
    ]
  },
  {
    schema: 'main',
    name: 'note',
    type: 'table',
    columns: [
      { name: 'id', type: 'INTEGER' },
      { name: 'body', type: 'TEXT' }
    ]
  },
  {
    schema: 'archive',
    name: 'city',
    type: 'view',
    columns: [{ name: 'name', type: 'TEXT' }]
  }
]

function at(sql, extra = {}) {
  const cursor = sql.indexOf('|')
  if (cursor < 0) {
    throw new Error('缺少光标')
  }
  return suggestSQL({
    sql: sql.replace('|', ''),
    cursor,
    driver: 'sqlite',
    tables,
    schemas: ['main', 'archive'],
    schema: 'main',
    ...extra
  })
}

function labels(result, kind) {
  return result.items.filter((item) => !kind || item.kind === kind).map((item) => item.label)
}

test('高亮关键字、字符串、数字、注释和表名，并转义 HTML', () => {
  const html = highlightSQL("select 1 as ok from city where name = '<b>' -- 注释\n", 'sqlite', tables)
  assert.match(html, /class="sql-keyword">select</)
  assert.match(html, /class="sql-number">1</)
  assert.match(html, /class="sql-table">city</)
  assert.match(html, /class="sql-column">name</)
  assert.match(html, /class="sql-string">/)
  assert.match(html, /&lt;b&gt;/)
  assert.doesNotMatch(html, /<b>/)
  assert.match(html, /class="sql-comment">-- 注释/)
  assert.equal(highlightSQL('/* select */', 'sqlite', tables).includes('sql-keyword'), false)
})

test('按前缀提示关键字，SQLite 才提示 pragma', () => {
  const result = at('sel|')
  const select = result.items.find((item) => item.label === 'select')
  assert.equal(select.insert, 'select ')
  assert.equal(result.from, 0)
  assert.equal(result.to, 3)
  assert.ok(at('prag|').items.some((item) => item.label === 'pragma'))
  const mysql = suggestSQL({ sql: 'prag', cursor: 4, driver: 'mysql', tables: [], schema: '' })
  assert.equal(mysql.items.some((item) => item.label === 'pragma'), false)
})

test('FROM 后提示表，非默认模式带模式名前缀', () => {
  const result = at('select * from |')
  assert.equal(result.mode, 'table')
  assert.equal(result.autoOpen, true)
  assert.deepEqual(
    result.items.filter((item) => item.kind === 'table').map((item) => `${item.label}:${item.insert}:${item.detail}`),
    ['city:city:main · 表', 'note:note:main · 表', 'city:archive.city:archive · 视图']
  )
  assert.deepEqual(labels(at('select * from ci|'), 'table'), ['city', 'city'])
})

test('SELECT 和点号后提示当前表的字段', () => {
  const selected = at('select | from city')
  assert.equal(selected.mode, 'column')
  assert.deepEqual(selected.items.filter((item) => item.section === '字段').map((item) => item.label), ['*', 'id', 'name', 'population'])
  assert.equal(selected.items.find((item) => item.label === 'population').detail, 'INTEGER')
  assert.equal(selected.items.some((item) => item.label === 'body'), false)
  assert.deepEqual(labels(at('select city.|'), 'column'), ['id', 'name', 'population'])
  assert.deepEqual(labels(at('select archive.city.|'), 'column'), ['name'])
  assert.deepEqual(labels(at('select main.c|'), 'table'), ['city'])
  assert.deepEqual(labels(at('select | from archive.city'), 'column').filter((label) => label !== '*'), ['name'])
})

test('语句边界、更新语句和标识符引用', () => {
  const second = at('select population from city; select | from note')
  assert.deepEqual(second.items.filter((item) => item.section === '字段' && item.label !== '*').map((item) => item.label), ['id', 'body'])
  assert.ok(at('update city set |').items.some((item) => item.label === 'population'))
  assert.equal(at('select id |').autoOpen, false)
  const cursor = 'select * from city'.indexOf('city') + 2
  const middle = suggestSQL({
    sql: 'select * from city',
    cursor,
    driver: 'sqlite',
    tables,
    schemas: ['main'],
    schema: 'main'
  })
  assert.equal(middle.prefix, 'ci')
  assert.equal(middle.from, 'select * from '.length)
  assert.equal(middle.to, 'select * from city'.length)
})

test('字符串和注释里不提示', () => {
  assert.equal(at("select 'sel|'").items.length, 0)
  assert.equal(at('select 1 -- sel|').items.length, 0)
  assert.equal(at('select /* sel| */ 1').items.length, 0)
})

test('关键字和带空格的名称会按驱动加引号', () => {
  assert.equal(quoteIdent('sqlite', 'city'), 'city')
  assert.equal(quoteIdent('sqlite', '订单'), '订单')
  assert.equal(quoteIdent('sqlite', 'order'), '"order"')
  assert.equal(quoteIdent('mysql', 'order'), '`order`')
  assert.equal(quoteIdent('sqlserver', 'order'), '[order]')
  assert.equal(quoteIdent('sqlite', 'a b'), '"a b"')
  const odd = suggestSQL({
    sql: 'select * from ord|',
    cursor: 'select * from ord'.length,
    driver: 'mysql',
    tables: [{ schema: 'app', name: 'order', type: 'table', columns: [] }],
    schemas: ['app'],
    schema: 'app'
  })
  assert.equal(odd.items.find((item) => item.kind === 'table').insert, '`order`')
})
