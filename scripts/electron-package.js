const fs = require('fs')
const path = require('path')
const zlib = require('zlib')

const execNames = new Set(['dbgo', 'dbgo.exe', 'DBGo', 'DBGo.exe', 'chrome-sandbox', 'chrome_crashpad_handler'])

function describeBinary(file) {
  const buf = Buffer.alloc(8192)
  const fd = fs.openSync(file, 'r')
  let size = 0
  try {
    size = fs.readSync(fd, buf, 0, buf.length, 0)
  } finally {
    fs.closeSync(fd)
  }
  const bytes = buf.subarray(0, size)
  if (bytes.length >= 2 && bytes.toString('ascii', 0, 2) === 'MZ') {
    const lfanew = bytes.readUInt32LE(0x3c)
    if (lfanew + 26 > bytes.length) {
      throw new Error('PE 头超出已读取范围')
    }
    if (bytes.toString('ascii', lfanew, lfanew + 4) !== 'PE\0\0') {
      throw new Error('不是有效的 PE 文件')
    }
    const machine = bytes.readUInt16LE(lfanew + 4)
    const magic = bytes.readUInt16LE(lfanew + 24)
    const arch = machine === 0x8664 ? 'x86-64' : machine === 0xaa64 ? 'arm64' : `machine ${machine}`
    return { kind: 'pe', machine, pe32plus: magic === 0x20b, text: `PE ${magic === 0x20b ? 'PE32+' : 'PE32'} ${arch}` }
  }
  if (bytes.length >= 20 && bytes[0] === 0x7f && bytes.toString('ascii', 1, 4) === 'ELF') {
    const little = bytes[5] === 1
    const machine = little ? bytes.readUInt16LE(18) : bytes.readUInt16BE(18)
    const arch = machine === 62 ? 'x86-64' : machine === 183 ? 'aarch64' : `machine ${machine}`
    return { kind: 'elf', machine, text: `ELF ${arch}` }
  }
  if (bytes.length >= 8) {
    const leMagic = bytes.readUInt32LE(0)
    const beMagic = bytes.readUInt32BE(0)
    if (leMagic === 0xfeedfacf || beMagic === 0xfeedfacf) {
      const little = leMagic === 0xfeedfacf
      const cpu = little ? bytes.readUInt32LE(4) : bytes.readUInt32BE(4)
      const arch = cpu === 0x01000007 ? 'x86_64' : cpu === 0x0100000c ? 'arm64' : `cpu ${cpu}`
      return { kind: 'macho', cpu, text: `Mach-O ${arch}` }
    }
  }
  throw new Error('无法识别服务程序格式')
}

function binaryMatches(folder, info) {
  if (folder === 'win-x64') {
    return info.kind === 'pe' && info.pe32plus && info.machine === 0x8664
  }
  if (folder === 'mac-x64') {
    return info.kind === 'macho' && info.cpu === 0x01000007
  }
  if (folder === 'mac-arm64') {
    return info.kind === 'macho' && info.cpu === 0x0100000c
  }
  if (folder === 'linux-x64') {
    return info.kind === 'elf' && info.machine === 62
  }
  if (folder === 'linux-arm64') {
    return info.kind === 'elf' && info.machine === 183
  }
  return false
}

function checkBinary(file, folder) {
  if (!fs.existsSync(file)) {
    throw new Error(`缺少服务程序 ${file}`)
  }
  const info = describeBinary(file)
  if (!binaryMatches(folder, info)) {
    throw new Error(`${folder} 的服务程序架构不对: ${info.text}`)
  }
  return info.text
}

function isExecName(name) {
  const normalized = name.replace(/\\/g, '/')
  if (normalized.endsWith('/')) {
    return false
  }
  return execNames.has(path.posix.basename(normalized))
}

function findEocd(buf) {
  const min = Math.max(0, buf.length - 22 - 65535)
  for (let i = buf.length - 22; i >= min; i--) {
    if (buf.readUInt32LE(i) === 0x06054b50) {
      return i
    }
  }
  throw new Error('不是 zip')
}

function fixZip(file) {
  const buf = fs.readFileSync(file)
  const eocd = findEocd(buf)
  const count = buf.readUInt16LE(eocd + 10)
  const offsetValue = buf.readUInt32LE(eocd + 16)
  if (count === 0xffff || offsetValue === 0xffffffff) {
    throw new Error('暂不支持 zip64')
  }
  let offset = offsetValue
  let changed = false
  for (let n = 0; n < count; n++) {
    if (buf.readUInt32LE(offset) !== 0x02014b50) {
      throw new Error('zip 中央目录损坏')
    }
    const nameLen = buf.readUInt16LE(offset + 28)
    const extraLen = buf.readUInt16LE(offset + 30)
    const commentLen = buf.readUInt16LE(offset + 32)
    const name = buf.toString('utf8', offset + 46, offset + 46 + nameLen)
    if (isExecName(name)) {
      const made = buf.readUInt16LE(offset + 4)
      const attr = buf.readUInt32LE(offset + 38)
      const unix = (made >> 8) === 3
      const mode = (attr >> 16) & 0o7777
      if (!unix || (mode & 0o111) === 0) {
        buf.writeUInt16LE((made & 0xff) | (3 << 8), offset + 4)
        const nextAttr = (((0o100755 << 16) >>> 0) | (attr & 0xffff)) >>> 0
        buf.writeUInt32LE(nextAttr, offset + 38)
        changed = true
      }
    }
    offset += 46 + nameLen + extraLen + commentLen
  }
  if (changed) {
    const tmp = `${file}.tmp`
    fs.writeFileSync(tmp, buf)
    fs.renameSync(tmp, file)
  }
  return changed
}

function octal(text) {
  return parseInt(text.replace(/\0.*/, '').trim() || '0', 8)
}

function writeOctal(header, offset, width, value) {
  const field = value.toString(8).padStart(width - 1, '0')
  header.write(`${field}\0`, offset, width, 'ascii')
}

function writeChecksum(header) {
  const sum = headerChecksum(header)
  header.write(`${sum.toString(8).padStart(6, '0')}\0 `, 148, 8, 'ascii')
}

function headerChecksum(header) {
  const copy = Buffer.from(header.subarray(0, 512))
  copy.fill(0x20, 148, 156)
  let sum = 0
  for (const byte of copy) {
    sum += byte
  }
  return sum
}

function tarName(header) {
  const name = header.toString('utf8', 0, 100).replace(/\0.*/, '')
  const magic = header.toString('ascii', 257, 262)
  if (magic.startsWith('ustar')) {
    const prefix = header.toString('utf8', 345, 500).replace(/\0.*/, '')
    if (prefix) {
      return `${prefix}/${name}`
    }
  }
  return name
}

function fixTar(file) {
  const raw = zlib.gunzipSync(fs.readFileSync(file))
  let changed = false
  let offset = 0
  let longName = ''
  while (offset + 512 <= raw.length) {
    const header = raw.subarray(offset, offset + 512)
    if (header.every((byte) => byte === 0)) {
      break
    }
    const type = header[156]
    const size = octal(header.toString('ascii', 124, 136))
    const dataStart = offset + 512
    let name = tarName(header)
    if (longName) {
      name = longName
      longName = ''
    }
    if (type === 76) {
      longName = raw.toString('utf8', dataStart, dataStart + size).replace(/\0.*/, '')
    }
    if ((type === 48 || type === 0) && isExecName(name)) {
      const mode = octal(header.toString('ascii', 100, 108))
      if ((mode & 0o111) === 0) {
        writeOctal(header, 100, 8, 0o755)
        writeChecksum(header)
        changed = true
      }
    }
    const blocks = Math.ceil(size / 512)
    offset = dataStart + blocks * 512
  }
  if (changed) {
    const tmp = `${file}.tmp`
    fs.writeFileSync(tmp, zlib.gzipSync(raw))
    fs.renameSync(tmp, file)
  }
  return changed
}

function fixModes(dir) {
  const names = fs.readdirSync(dir)
  let changed = 0
  for (const name of names) {
    const file = path.join(dir, name)
    if (!fs.statSync(file).isFile()) {
      continue
    }
    if (name.endsWith('.zip')) {
      if (fixZip(file)) {
        changed += 1
      }
    } else if (name.endsWith('.tar.gz')) {
      if (fixTar(file)) {
        changed += 1
      }
    }
  }
  return changed
}

function main() {
  const [cmd, ...rest] = process.argv.slice(2)
  if (cmd === 'check-binary') {
    const text = checkBinary(rest[0], rest[1])
    process.stdout.write(`${text}\n`)
    return
  }
  if (cmd === 'fix-modes') {
    const changed = fixModes(rest[0])
    process.stdout.write(`已修正 ${changed} 个压缩包里的可执行权限\n`)
    return
  }
  throw new Error('用法: check-binary <文件> <目录名> 或 fix-modes <目录>')
}

if (require.main === module) {
  try {
    main()
  } catch (err) {
    process.stderr.write(`${err.message}\n`)
    process.exit(1)
  }
}

module.exports = {
  binaryMatches,
  checkBinary,
  describeBinary,
  fixModes,
  fixTar,
  fixZip
}
