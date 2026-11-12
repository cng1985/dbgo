const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('fs')
const os = require('os')
const path = require('path')
const zlib = require('zlib')
const { checkBinary, fixModes } = require('./electron-package')

function writeOctal(header, offset, width, value) {
  header.write(`${value.toString(8).padStart(width - 1, '0')}\0`, offset, width, 'ascii')
}

function tarHeader(name, size, mode) {
  const header = Buffer.alloc(512)
  header.write(name, 0, 'utf8')
  writeOctal(header, 100, 8, mode)
  writeOctal(header, 108, 8, 0)
  writeOctal(header, 116, 8, 0)
  writeOctal(header, 124, 12, size)
  writeOctal(header, 136, 12, 0)
  header[156] = 48
  header.write('ustar\0', 257, 'ascii')
  header.write('00', 263, 'ascii')
  const copy = Buffer.from(header)
  copy.fill(0x20, 148, 156)
  let sum = 0
  for (const byte of copy) {
    sum += byte
  }
  header.write(`${sum.toString(8).padStart(6, '0')}\0 `, 148, 8, 'ascii')
  return header
}

function makeTar(entries) {
  const parts = []
  for (const entry of entries) {
    const data = Buffer.from(entry.data)
    parts.push(tarHeader(entry.name, data.length, entry.mode))
    const block = Buffer.alloc(Math.ceil(data.length / 512) * 512)
    data.copy(block)
    parts.push(block)
  }
  parts.push(Buffer.alloc(1024))
  return zlib.gzipSync(Buffer.concat(parts))
}

function readTarMode(gz, name) {
  const raw = zlib.gunzipSync(gz)
  let offset = 0
  while (offset + 512 <= raw.length) {
    const header = raw.subarray(offset, offset + 512)
    if (header.every((byte) => byte === 0)) {
      break
    }
    const entry = header.toString('utf8', 0, 100).replace(/\0.*/, '')
    const mode = parseInt(header.toString('ascii', 100, 108).replace(/\0.*/, ''), 8)
    const size = parseInt(header.toString('ascii', 124, 136).replace(/\0.*/, ''), 8)
    if (entry === name) {
      return mode
    }
    offset += 512 + Math.ceil(size / 512) * 512
  }
  throw new Error(`缺少 ${name}`)
}

function u16(n) {
  const buf = Buffer.alloc(2)
  buf.writeUInt16LE(n)
  return buf
}

function u32(n) {
  const buf = Buffer.alloc(4)
  buf.writeUInt32LE(n)
  return buf
}

function makeZip(files) {
  const locals = []
  const centrals = []
  let offset = 0
  for (const file of files) {
    const name = Buffer.from(file.name)
    const data = Buffer.from(file.data)
    const local = Buffer.concat([
      u32(0x04034b50), u16(20), u16(0), u16(0), u16(0), u16(0),
      u32(0), u32(data.length), u32(data.length), u16(name.length), u16(0),
      name, data
    ])
    const made = file.unix ? (3 << 8) | 20 : 20
    const attr = file.unix ? ((file.mode << 16) >>> 0) : 0x20
    const central = Buffer.concat([
      u32(0x02014b50), u16(made), u16(20), u16(0), u16(0), u16(0), u16(0),
      u32(0), u32(data.length), u32(data.length), u16(name.length), u16(0), u16(0),
      u16(0), u16(0), u32(attr), u32(offset), name
    ])
    locals.push(local)
    centrals.push(central)
    offset += local.length
  }
  const central = Buffer.concat(centrals)
  const eocd = Buffer.concat([
    u32(0x06054b50), u16(0), u16(0), u16(files.length), u16(files.length),
    u32(central.length), u32(offset), u16(0)
  ])
  return Buffer.concat([...locals, central, eocd])
}

function readZipAttr(buf, name) {
  const min = Math.max(0, buf.length - 22 - 65535)
  let eocd = -1
  for (let i = buf.length - 22; i >= min; i--) {
    if (buf.readUInt32LE(i) === 0x06054b50) {
      eocd = i
      break
    }
  }
  let offset = buf.readUInt32LE(eocd + 16)
  const count = buf.readUInt16LE(eocd + 10)
  for (let n = 0; n < count; n++) {
    const nameLen = buf.readUInt16LE(offset + 28)
    const extraLen = buf.readUInt16LE(offset + 30)
    const commentLen = buf.readUInt16LE(offset + 32)
    const entry = buf.toString('utf8', offset + 46, offset + 46 + nameLen)
    if (entry === name) {
      return { made: buf.readUInt16LE(offset + 4), attr: buf.readUInt32LE(offset + 38) }
    }
    offset += 46 + nameLen + extraLen + commentLen
  }
  throw new Error(`缺少 ${name}`)
}

test('压缩包里的服务程序补上可执行权限', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dbgo-archive-'))
  fs.writeFileSync(path.join(dir, 'app.tar.gz'), makeTar([
    { name: 'DBGo.app/Contents/MacOS/DBGo', data: 'mac', mode: 0o644 },
    { name: 'DBGo.app/Contents/Resources/bin/dbgo', data: 'go', mode: 0o644 },
    { name: 'readme.txt', data: 'hi', mode: 0o644 }
  ]))
  fs.writeFileSync(path.join(dir, 'app.zip'), makeZip([
    { name: 'resources/bin/dbgo.exe', data: 'win', unix: false, mode: 0 },
    { name: 'LICENSE.txt', data: 'lic', unix: true, mode: 0o100644 }
  ]))
  assert.equal(fixModes(dir), 2)
  assert.equal(fixModes(dir), 0)
  const gz = fs.readFileSync(path.join(dir, 'app.tar.gz'))
  assert.equal(readTarMode(gz, 'DBGo.app/Contents/MacOS/DBGo'), 0o755)
  assert.equal(readTarMode(gz, 'DBGo.app/Contents/Resources/bin/dbgo'), 0o755)
  assert.equal(readTarMode(gz, 'readme.txt'), 0o644)
  const raw = zlib.gunzipSync(gz)
  let offset = 0
  while (offset + 512 <= raw.length && !raw.subarray(offset, offset + 512).every((byte) => byte === 0)) {
    const header = raw.subarray(offset, offset + 512)
    const copy = Buffer.from(header)
    copy.fill(0x20, 148, 156)
    let sum = 0
    for (const byte of copy) {
      sum += byte
    }
    const stored = parseInt(header.toString('ascii', 148, 154), 8)
    assert.equal(stored, sum)
    const size = parseInt(header.toString('ascii', 124, 136).replace(/\0.*/, ''), 8)
    offset += 512 + Math.ceil(size / 512) * 512
  }
  const zip = fs.readFileSync(path.join(dir, 'app.zip'))
  const exe = readZipAttr(zip, 'resources/bin/dbgo.exe')
  assert.equal(exe.made >> 8, 3)
  assert.equal((exe.attr >> 16) & 0o777, 0o755)
  const license = readZipAttr(zip, 'LICENSE.txt')
  assert.equal((license.attr >> 16) & 0o777, 0o644)
  fs.rmSync(dir, { recursive: true, force: true })
})

test('能认出各平台服务程序', () => {
  const root = path.join(__dirname, '..', 'electron', 'dist')
  const cases = [
    ['win-unpacked/resources/bin/dbgo.exe', 'win-x64', 'PE'],
    ['mac/DBGo.app/Contents/Resources/bin/dbgo', 'mac-x64', 'Mach-O'],
    ['mac-arm64/DBGo.app/Contents/Resources/bin/dbgo', 'mac-arm64', 'Mach-O'],
    ['linux-unpacked/resources/bin/dbgo', 'linux-x64', 'ELF'],
    ['linux-arm64-unpacked/resources/bin/dbgo', 'linux-arm64', 'ELF']
  ]
  for (const [rel, folder, kind] of cases) {
    const file = path.join(root, rel)
    if (!fs.existsSync(file)) {
      continue
    }
    const text = checkBinary(file, folder)
    assert.equal(text.startsWith(kind), true, text)
    assert.throws(() => checkBinary(file, folder === 'win-x64' ? 'linux-x64' : 'win-x64'))
  }
})
