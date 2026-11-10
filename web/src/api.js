export class ApiError extends Error {
  constructor(message, code) {
    super(message)
    this.code = code
  }
}

export async function api(path, body) {
  const response = await fetch(path, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      'X-DBGo-Request': '1'
    },
    body: JSON.stringify(body ?? {})
  })
  const payload = await response.json()
  if (Object.prototype.hasOwnProperty.call(payload, 'code') && payload.code !== 200) {
    if (payload.code === 401) {
      window.dispatchEvent(new Event('dbgo-unauthorized'))
    }
    throw new ApiError(payload.msg || '请求失败', payload.code)
  }
  return payload
}

export async function downloadBackup(id, filename) {
  const response = await fetch('/api/backup/download?id=' + encodeURIComponent(id), {
    credentials: 'same-origin',
    headers: { 'X-DBGo-Request': '1' }
  })
  if (!response.ok) {
    let message = '下载失败'
    try {
      const payload = await response.json()
      message = payload.msg || message
    } catch {
      message = '下载失败'
    }
    throw new ApiError(message, response.status)
  }
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename || 'backup.sql'
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

export function formatTime(value) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }
  return new Intl.DateTimeFormat('zh-CN', {
    hour12: false,
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  }).format(date)
}

export function formatBytes(value) {
  const size = Number(value || 0)
  if (size < 1024) {
    return size + ' B'
  }
  if (size < 1024 * 1024) {
    return (size / 1024).toFixed(1) + ' KB'
  }
  return (size / 1024 / 1024).toFixed(1) + ' MB'
}

export const drivers = [
  { value: 'sqlite', label: 'SQLite' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'postgres', label: 'PostgreSQL' },
  { value: 'sqlserver', label: 'SQL Server' }
]

export function driverLabel(value) {
  return drivers.find((item) => item.value === value)?.label || value || '—'
}

export const statusText = {
  success: '成功',
  failed: '失败',
  draft: '草稿',
  applying: '执行中',
  applied: '已执行',
  reverting: '回滚中',
  running: '进行中',
  ready: '就绪'
}
