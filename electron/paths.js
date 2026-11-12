const osFolder = {
  win32: 'win',
  darwin: 'mac',
  linux: 'linux'
}

const archById = {
  0: 'ia32',
  1: 'x64',
  2: 'armv7l',
  3: 'arm64',
  4: 'universal'
}

function binaryName(platform) {
  return platform === 'win32' ? 'dbgo.exe' : 'dbgo'
}

function archName(arch) {
  if (typeof arch === 'number') {
    return archById[arch] || ''
  }
  return String(arch || '')
}

function resourceFolder(platform, arch) {
  const osName = osFolder[platform]
  const name = archName(arch)
  if (!osName || !name) {
    throw new Error('不支持的系统或架构')
  }
  return `${osName}-${name}`
}

module.exports = {
  archById,
  archName,
  binaryName,
  osFolder,
  resourceFolder
}
