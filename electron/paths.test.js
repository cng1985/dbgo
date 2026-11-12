const test = require('node:test')
const assert = require('node:assert/strict')
const { binaryName, resourceFolder } = require('./paths')

test('各系统的服务程序文件名', () => {
  assert.equal(binaryName('win32'), 'dbgo.exe')
  assert.equal(binaryName('darwin'), 'dbgo')
  assert.equal(binaryName('linux'), 'dbgo')
})

test('发行目录按系统和架构分开', () => {
  assert.equal(resourceFolder('win32', 'x64'), 'win-x64')
  assert.equal(resourceFolder('darwin', 'arm64'), 'mac-arm64')
  assert.equal(resourceFolder('linux', 1), 'linux-x64')
  assert.equal(resourceFolder('linux', 3), 'linux-arm64')
})
