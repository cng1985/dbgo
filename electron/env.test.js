const test = require('node:test')
const assert = require('node:assert/strict')
const { childEnv } = require('./env')

test('丢掉继承的应用库配置', () => {
  const env = childEnv({
    PATH: '/usr/bin',
    HOME: '/home/me',
    APP_DATABASE_DRIVER: 'mysql',
    APP_DATABASE_DSN: 'user:pass@tcp(127.0.0.1:3306)/app',
    APP_DATABASE_SQLITE_PATH: '/var/lib/dbgo/app.db',
    app_database_dsn: 'postgres://dbgo@127.0.0.1/app',
    APP_AUTH_SECRET: 'shared-secret',
    APP_PROFILE: 'pro',
    APP_AUTH_COOKIE_SECURE: 'true'
  })
  assert.equal(env.PATH, '/usr/bin')
  assert.equal(env.HOME, '/home/me')
  assert.equal(env.APP_DATABASE_DRIVER, undefined)
  assert.equal(env.APP_DATABASE_DSN, undefined)
  assert.equal(env.APP_DATABASE_SQLITE_PATH, undefined)
  assert.equal(env.app_database_dsn, undefined)
  assert.equal(env.APP_AUTH_SECRET, undefined)
  assert.equal(env.APP_PROFILE, undefined)
  assert.equal(env.APP_AUTH_COOKIE_SECURE, undefined)
})
