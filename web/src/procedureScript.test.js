import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyProcedureParams,
  procedureCommentSQL,
  procedureName,
  procedureParams,
  appendViewTriggers,
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

test('函数用同一套来源模板，保存时改成函数语句', () => {
  const sql = procedureTemplate('sqlserver', 'dbo', 'to_decimal', 'function')
  assert.match(sql, /^CREATE FUNCTION dbo\.to_decimal\(\)\nRETURNS int\nAS\nBEGIN\n/)
  assert.equal(procedureName(sql), 'to_decimal')
  const saved = procedureSaveSQL('sqlserver', sql, { existing: true, kind: 'function' })
  assert.match(saved, /^CREATE OR ALTER FUNCTION dbo\.to_decimal/)
  assert.match(saved, /RETURN 0/)
  const mysql = procedureSaveSQL('mysql', procedureTemplate('mysql', 'shop', 'to_decimal', 'function'), {
    existing: true,
    schema: 'shop',
    name: 'to_decimal',
    kind: 'function'
  })
  assert.match(mysql, /^DROP FUNCTION IF EXISTS shop\.to_decimal;\nCREATE FUNCTION/)
  const postgres = procedureSaveSQL('postgres', procedureTemplate('postgres', 'app', 'to_decimal', 'function'), { existing: true, kind: 'function' })
  assert.match(postgres, /^CREATE OR REPLACE FUNCTION app\.to_decimal/)
  assert.match(procedureCommentSQL('sqlserver', 'dbo', 'to_decimal', '转换', [], 'function'), /N'FUNCTION'/)
  assert.match(procedureCommentSQL('mysql', 'shop', 'to_decimal', '转换', [], 'function'), /ALTER FUNCTION shop\.to_decimal COMMENT '转换'/)
  const rewritten = applyProcedureParams(sql, 'sqlserver', [
    { name: 'amount', type: 'decimal(18, 2)', mode: 'in', comment: '金额' }
  ], 'function')
  assert.match(rewritten, /@amount decimal\(18, 2\) -- 金额/)
  assert.match(rewritten, /RETURNS int/)
  assert.match(rewritten, /RETURN 0/)
})

test('视图用同一套来源编辑，保存时按视图语句覆盖', () => {
  const sql = procedureTemplate('sqlserver', 'dbo', 'city_names', 'view')
  assert.match(sql, /^CREATE VIEW dbo\.city_names\nAS\nSELECT 1 AS id\n/)
  assert.equal(procedureName(sql), 'city_names')
  const renamed = setProcedureIdentity(sql, 'sqlserver', 'dbo', 'city_labels')
  assert.match(renamed, /CREATE VIEW dbo\.city_labels\nAS\nSELECT 1 AS id/)
  assert.match(procedureSaveSQL('sqlserver', sql, { existing: true, kind: 'view' }), /^CREATE OR ALTER VIEW dbo\.city_names\nAS\nSELECT 1 AS id/)
  const postgres = procedureTemplate('postgres', 'app', 'city_names', 'view')
  assert.match(procedureSaveSQL('postgres', postgres, { existing: true, kind: 'view' }), /^CREATE OR REPLACE VIEW app\.city_names AS\nSELECT 1 AS id/)
  assert.equal(
    procedureSaveSQL('postgres', 'CREATE OR REPLACE VIEW app.city_names AS\nSELECT 1', { existing: true, kind: 'view' }),
    'CREATE OR REPLACE VIEW app.city_names AS\nSELECT 1\n'
  )
  const mysql = `CREATE ALGORITHM=UNDEFINED DEFINER=\`root\`@\`localhost\` SQL SECURITY DEFINER VIEW \`shop\`.\`city_names\` AS select 1 as id`
  assert.equal(procedureName(mysql), 'city_names')
  assert.match(procedureSaveSQL('mysql', mysql, { existing: true, schema: 'shop', name: 'city_names', kind: 'view' }), /^CREATE OR REPLACE ALGORITHM=UNDEFINED DEFINER=/)
  const sqlite = procedureTemplate('sqlite', 'main', 'city_names', 'view')
  assert.match(sqlite, /^CREATE VIEW city_names AS\nSELECT 1 AS id\n/)
  assert.match(procedureSaveSQL('sqlite', sqlite, { existing: true, schema: 'main', name: 'city_names', kind: 'view' }), /^DROP VIEW IF EXISTS city_names;\nCREATE VIEW city_names AS/)
  assert.match(procedureCommentSQL('sqlserver', 'dbo', 'city_names', '城市', [], 'view'), /N'VIEW'/)
  assert.equal(procedureCommentSQL('postgres', 'app', 'city_names', '城市', [], 'view'), `COMMENT ON VIEW app.city_names IS '城市'`)
  assert.equal(procedureCommentSQL('mysql', 'shop', 'city_names', '城市', [], 'view'), '')
  assert.equal(procedureCommentSQL('sqlite', 'main', 'city_names', '城市', [], 'view'), '')
  const optional = 'CREATE VIEW IF NOT EXISTS city_names AS\nSELECT 1 AS id'
  assert.equal(procedureName(optional), 'city_names')
  assert.equal(setProcedureIdentity(optional, 'sqlite', 'main', 'city_labels'), 'CREATE VIEW IF NOT EXISTS city_labels AS\nSELECT 1 AS id')
  assert.match(procedureSaveSQL('sqlite', optional, { existing: true, schema: 'main', name: 'city_names', kind: 'view' }), /^DROP VIEW IF EXISTS city_names;\nCREATE VIEW IF NOT EXISTS city_names AS\nSELECT 1 AS id/)
})

test('保存 SQLite 视图时把触发器接回新建的视图', () => {
  const script = procedureSaveSQL('sqlite', 'CREATE VIEW city_names AS\nSELECT 1 AS id', {
    existing: true,
    schema: 'main',
    name: 'city_names',
    kind: 'view'
  })
  const trigger = 'CREATE TRIGGER city_names_ins INSTEAD OF INSERT ON city_names BEGIN SELECT 1; END'
  const saved = appendViewTriggers(script, [trigger, '  '])
  assert.match(saved, /DROP VIEW IF EXISTS city_names;/)
  assert.match(saved, /CREATE VIEW city_names AS/)
  assert.match(saved, /CREATE TRIGGER city_names_ins INSTEAD OF INSERT ON city_names BEGIN SELECT 1; END;/)
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
