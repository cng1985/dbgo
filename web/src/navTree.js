export function matchesText(node, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) {
    return false
  }
  const label = String(node?.label || '').toLowerCase()
  const meta = String(node?.meta || '').toLowerCase()
  return label.includes(q) || meta.includes(q)
}

// 名称或地址命中才显示。未展开的连接、模式不再一律留下，否则上百个连接时按名称过滤收不窄。
// 已经加载的下级对象仍会把父节点留下来，表和过程名过滤继续有效。
export function nodeShown(node, query, force = false) {
  const q = String(query || '').trim().toLowerCase()
  if (!q || force) {
    return true
  }
  if (matchesText(node, q)) {
    return true
  }
  return (node?.children || []).some((child) => nodeShown(child, q, false))
}

export async function collectPages(fetchPage, pageSize = 200) {
  const rows = []
  let page = 1
  while (page <= 50) {
    const result = await fetchPage(page, pageSize)
    const batch = Array.isArray(result?.data) ? result.data : []
    rows.push(...batch)
    const total = Number(result?.total || 0)
    if (!batch.length || batch.length < pageSize || rows.length >= total) {
      return rows
    }
    page += 1
  }
  return rows
}
