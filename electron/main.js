const { app, BrowserWindow, dialog } = require('electron')

if (process.env.DBGO_DEBUG_PORT) {
  app.commandLine.appendSwitch('remote-debugging-port', process.env.DBGO_DEBUG_PORT)
}
const { spawn } = require('child_process')
const fs = require('fs')
const http = require('http')
const net = require('net')
const path = require('path')
const { childEnv } = require('./env')
const { binaryName } = require('./paths')

const readyTimeoutMs = 30000
let backend = null
let mainWindow = null
let quitting = false

function backendBinary() {
  const name = binaryName(process.platform)
  if (process.env.DBGO_BIN) {
    return process.env.DBGO_BIN
  }
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'bin', name)
  }
  return path.join(__dirname, 'resources', 'bin', name)
}

function configDirectory() {
  if (process.env.DBGO_CONFIG_DIR) {
    return process.env.DBGO_CONFIG_DIR
  }
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'configs')
  }
  return path.join(__dirname, 'resources', 'configs')
}

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer()
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => {
      const address = server.address()
      server.close(() => resolve(address.port))
    })
  })
}

function readTail(file) {
  try {
    const text = fs.readFileSync(file, 'utf8')
    return text.slice(-2000).trim()
  } catch {
    return ''
  }
}

function waitUntilReady(port, child, logFile) {
  return new Promise((resolve, reject) => {
    const deadline = Date.now() + readyTimeoutMs
    let settled = false
    const finish = (err) => {
      if (settled) {
        return
      }
      settled = true
      if (err) {
        reject(err)
        return
      }
      resolve()
    }
    const probe = () => {
      if (settled) {
        return
      }
      if (child.exitCode !== null || child.signalCode) {
        finish(new Error(readTail(logFile) || '服务程序已退出'))
        return
      }
      const request = http.get({ hostname: '127.0.0.1', port, path: '/health/live', timeout: 1000 }, (response) => {
        response.resume()
        if (response.statusCode === 200) {
          finish()
          return
        }
        retry()
      })
      request.on('error', retry)
      request.on('timeout', () => {
        request.destroy()
        retry()
      })
    }
    const retry = () => {
      if (settled) {
        return
      }
      if (Date.now() > deadline) {
        finish(new Error(readTail(logFile) || '数据库工作台没有在限定时间内启动'))
        return
      }
      setTimeout(probe, 200)
    }
    probe()
  })
}

function stopBackend() {
  const child = backend
  backend = null
  if (!child || child.exitCode !== null) {
    return
  }
  if (process.platform === 'win32') {
    spawn('taskkill', ['/pid', String(child.pid), '/t', '/f'], { windowsHide: true, stdio: 'ignore' })
    return
  }
  child.kill('SIGTERM')
}

async function startBackend() {
  const bin = backendBinary()
  if (!fs.existsSync(bin)) {
    throw new Error(`找不到服务程序: ${bin}`)
  }
  const dataDir = path.join(app.getPath('userData'), 'data')
  const logDir = path.join(app.getPath('userData'), 'logs')
  fs.mkdirSync(dataDir, { recursive: true })
  fs.mkdirSync(logDir, { recursive: true })
  const logFile = path.join(logDir, 'dbgo.log')
  const log = fs.createWriteStream(logFile, { flags: 'w' })
  const port = await freePort()
  const child = spawn(bin, [
    '--base-dir', dataDir,
    '--config-dir', configDirectory(),
    '--data-dir', dataDir,
    '--set', 'server.host=127.0.0.1',
    '--set', `server.port=${port}`,
    '--set', 'database.driver=sqlite',
    '--set', 'database.sqlite.path=app.db'
  ], {
    env: childEnv(process.env),
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe']
  })
  child.stdout.pipe(log)
  child.stderr.pipe(log)
  child.on('error', (err) => {
    log.write(`${err.message}\n`)
  })
  backend = child
  try {
    await waitUntilReady(port, child, logFile)
  } catch (err) {
    stopBackend()
    throw err
  }
  return { port, dataDir }
}

function maybeExplainPassword(dataDir, window) {
  const file = path.join(dataDir, 'initial-admin-password.txt')
  let stat
  try {
    stat = fs.statSync(file)
  } catch {
    return
  }
  if (Date.now() - stat.mtimeMs > 20000) {
    return
  }
  const password = fs.readFileSync(file, 'utf8').trim()
  if (!password) {
    return
  }
  dialog.showMessageBox(window, {
    type: 'info',
    title: '初始管理员',
    message: '第一次打开已生成管理员账号。',
    detail: `用户名 admin\n口令 ${password}\n口令文件：${file}`
  })
}

function openWindow(port) {
  const window = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 960,
    minHeight: 640,
    title: 'DBGo',
    show: false,
    backgroundColor: '#f3f3f3',
    webPreferences: {
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    }
  })
  mainWindow = window
  const origin = `http://127.0.0.1:${port}`
  window.webContents.setWindowOpenHandler(() => ({ action: 'deny' }))
  window.webContents.on('will-navigate', (event, url) => {
    if (!url.startsWith(`${origin}/`) && url !== origin) {
      event.preventDefault()
    }
  })
  window.once('ready-to-show', () => window.show())
  window.loadURL(`${origin}/`)
  return window
}

async function boot() {
  try {
    const { port, dataDir } = await startBackend()
    const window = openWindow(port)
    window.once('ready-to-show', () => maybeExplainPassword(dataDir, window))
  } catch (err) {
    dialog.showErrorBox('DBGo 没有启动', err.message || '服务程序启动失败')
    app.quit()
  }
}

const gotLock = app.requestSingleInstanceLock()
if (!gotLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) {
        mainWindow.restore()
      }
      mainWindow.focus()
    }
  })
  app.whenReady().then(boot)
  app.on('before-quit', () => {
    quitting = true
    stopBackend()
  })
  app.on('window-all-closed', () => {
    if (!quitting) {
      stopBackend()
    }
    app.quit()
  })
}
