const storageKey = 'dbgo-sql-suggest'

export function readSuggestEnabled() {
  try {
    return localStorage.getItem(storageKey) !== '0'
  } catch {
    return true
  }
}

export function writeSuggestEnabled(enabled) {
  const value = !!enabled
  try {
    localStorage.setItem(storageKey, value ? '1' : '0')
  } catch {
    // 当前页面仍然按这次选择工作。
  }
  window.dispatchEvent(new CustomEvent('dbgo-suggest', { detail: value }))
}
