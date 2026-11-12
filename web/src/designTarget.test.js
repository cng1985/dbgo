import assert from 'node:assert/strict'
import test from 'node:test'
import { designTarget, sameDesignTarget } from './designTarget.js'

test('同一连接、模式、数据库和对象类型才算同一个建表目标', () => {
  const target = designTarget('1', 'main', 'sqlite', 'table')
  assert.equal(sameDesignTarget(target, designTarget('1', 'main', 'sqlite', 'table')), true)
  assert.equal(sameDesignTarget(target, designTarget('2', 'main', 'sqlite', 'table')), false)
  assert.equal(sameDesignTarget(target, designTarget('1', 'sales', 'sqlite', 'table')), false)
  assert.equal(sameDesignTarget(target, designTarget('1', 'main', 'postgres', 'table')), false)
  assert.equal(sameDesignTarget(target, designTarget('1', 'main', 'sqlite', 'view')), false)
  assert.equal(sameDesignTarget(target, null), false)
})
