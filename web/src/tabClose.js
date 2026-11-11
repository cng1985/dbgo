export function applyTabClose(tabs, activeId, targetId, mode) {
  const list = Array.isArray(tabs) ? tabs : []
  const index = list.findIndex((tab) => tab.id === targetId)
  if (index < 0) {
    return { tabs: list.slice(), activeId }
  }
  let next = list
  if (mode === 'others') {
    next = [list[index]]
  } else if (mode === 'all') {
    next = []
  } else if (mode === 'left') {
    next = list.slice(index)
  } else if (mode === 'right') {
    next = list.slice(0, index + 1)
  } else if (mode === 'one') {
    next = list.filter((tab) => tab.id !== targetId)
  } else {
    return { tabs: list.slice(), activeId }
  }
  let active = activeId
  if (!next.some((tab) => tab.id === active)) {
    if (mode === 'one') {
      const neighbor = list[index + 1] || list[index - 1]
      active = next.find((tab) => tab.id === neighbor?.id)?.id || next[0]?.id || ''
    } else if (next.some((tab) => tab.id === targetId)) {
      active = targetId
    } else {
      active = next[0]?.id || ''
    }
  }
  return { tabs: next, activeId: active }
}
