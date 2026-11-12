import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyProcedureParams,
  procedureCommentSQL,
  procedureName,
  procedureParams,
  procedureSaveSQL,
  procedureTemplate,
  setProcedureIdentity
} from './procedureScript.js'

const sample = `CREATE PROCEDURE dbo.bsyc_drp_box_order_create
@creator_name varchar(50), -- 制单人
@json varchar(max) -- json数据
AS
begin
  declare @shop_code varchar(20);
  set @shop_code = JSON_VALUE(@json, '$.shopCode');
end
`

test('SQL Server 模板带过程名，主体从 AS BEGIN 开始', () => {
  const sql = procedureTemplate('sqlserver', 'dbo', 'save_order')
  assert.match(sql, /^CREATE PROCEDURE dbo\.save_order\nAS\nBEGIN\n/)
  assert.equal(procedureName(sql), 'save_order')
  assert.equal(procedureTemplate('sqlite', 'main', 'save_order'), '')
})

test('名称输入会改脚本里的过程名，过程体里的变量不动', () => {
  const renamed = setProcedureIdentity(sample, 'sqlserver', 'dbo', 'save_order')
  assert.match(renamed, /CREATE PROCEDURE dbo\.save_order\n/)
  assert.match(renamed, /declare @shop_code/)
  assert.equal(procedureName(renamed), 'save_order')
})

test('参数只取过程头，保留注释，不把过程体里的变量算进去', () => {
  const params = procedureParams(sample)
  assert.deepEqual(params, [
    { name: 'creator_name', type: 'varchar(50)', mode: 'in', comment: '制单人' },
    { name: 'json', type: 'varchar(max)', mode: 'in', comment: 'json数据' }
  ])
  const mysql = procedureParams('CREATE PROCEDURE shop.save_order(IN order_id int, OUT msg varchar(50))\nBEGIN\nEND\n')
  assert.deepEqual(mysql.map((item) => item.name), ['order_id', 'msg'])
  assert.equal(mysql[1].mode, 'out')
})

test('参数类型的括号还没写完时，不改掉过程体', () => {
  const sql = procedureTemplate('sqlserver', 'dbo', 'save_order')
  const partial = applyProcedureParams(sql, 'sqlserver', [
    { name: 'creator_name', type: 'varchar(', mode: 'in', comment: '' }
  ])
  assert.equal(partial, sql)
  const done = applyProcedureParams(sql, 'sqlserver', [
    { name: 'creator_name', type: 'varchar(50)', mode: 'in', comment: '制单人' }
  ])
  assert.match(done, /@creator_name varchar\(50\) -- 制单人/)
  assert.match(done, /SET NOCOUNT ON/)
})

test('在参数页改动后写回来源', () => {
  const next = applyProcedureParams(sample, 'sqlserver', [
    { name: 'creator_name', type: 'varchar(50)', mode: 'in', comment: '制单人' },
    { name: 'json', type: 'nvarchar(max)', mode: 'in', comment: 'json数据' }
  ])
  assert.match(next, /@json nvarchar\(max\) -- json数据/)
  assert.match(next, /declare @shop_code/)
  assert.equal(procedureParams(next)[1].type, 'nvarchar(max)')
})

test('已有过程保存时改成可覆盖的语句', () => {
  const sqlserver = procedureSaveSQL('sqlserver', sample, { existing: true })
  assert.match(sqlserver, /^CREATE OR ALTER PROCEDURE dbo\.bsyc_drp_box_order_create/)
  const again = procedureSaveSQL('sqlserver', sqlserver, { existing: true })
  assert.equal(again.match(/CREATE OR ALTER/g).length, 1)
  const mysql = procedureSaveSQL('mysql', 'CREATE PROCEDURE save_order()\nBEGIN\nEND', {
    existing: true,
    schema: 'shop',
    name: 'save_order'
  })
  assert.match(mysql, /^DROP PROCEDURE IF EXISTS shop\.save_order;/)
  const postgres = procedureSaveSQL('postgres', 'CREATE PROCEDURE app.save_order()\nLANGUAGE plpgsql\nAS $$\nBEGIN\n  NULL;\nEND;\n$$', { existing: true })
  assert.match(postgres, /^CREATE OR REPLACE PROCEDURE app\.save_order/)
})

test('描述语句会转义引号', () => {
  const sql = procedureCommentSQL('sqlserver', 'dbo', 'save_order', '用户的\'订单\'')
  assert.match(sql, /N'用户的''订单'''/)
  assert.match(sql, /^BEGIN\n/)
  assert.match(sql, /sp_addextendedproperty/)
  assert.match(procedureCommentSQL('mysql', 'shop', 'save_order', '备注'), /ALTER PROCEDURE shop\.save_order COMMENT '备注'/)
  assert.match(procedureCommentSQL('mysql', 'order', 'save_order', '备注'), /ALTER PROCEDURE `order`\.save_order COMMENT '备注'/)
})

test('MySQL 带 DEFINER 的过程头仍能读出参数，保存时先删除', () => {
  const shown = "CREATE DEFINER=`root`@`%` PROCEDURE `shop`.`save_order`(IN `order_id` int, OUT `msg` varchar(50))\nBEGIN\n  SET msg = 'ok';\nEND"
  assert.equal(procedureName(shown), 'save_order')
  assert.deepEqual(procedureParams(shown).map((item) => ({ name: item.name, mode: item.mode })), [
    { name: 'order_id', mode: 'in' },
    { name: 'msg', mode: 'out' }
  ])
  const saved = procedureSaveSQL('mysql', shown, { existing: true, schema: 'shop', name: 'save_order' })
  assert.match(saved, /^DROP PROCEDURE IF EXISTS shop\.save_order;\nCREATE DEFINER=`root`@`%` PROCEDURE/)
  assert.match(saved, /SET msg = 'ok'/)
})

test('PostgreSQL 大小写和保留字在改名时继续加引号', () => {
  const sql = 'CREATE PROCEDURE "Sales"."SaveOrder"()\nLANGUAGE plpgsql\nAS $$\nBEGIN\n  NULL;\nEND;\n$$\n'
  assert.equal(setProcedureIdentity(sql, 'postgres', 'Sales', 'SaveOrder'), sql)
  const renamed = setProcedureIdentity(sql, 'postgres', 'Sales', 'SaveOrder2')
  assert.match(renamed, /CREATE PROCEDURE "Sales"\."SaveOrder2"\(\)/)
  assert.match(renamed, /NULL/)
  assert.match(procedureTemplate('postgres', 'app', 'order'), /CREATE PROCEDURE app\."order"\(\)/)
  assert.match(procedureTemplate('sqlserver', 'dbo', 'order'), /CREATE PROCEDURE dbo\.\[order\]\n/)
})
