<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api, connectionTarget, driverLabel, listConnections } from '../api'
import { useSession } from '../stores/session'
import { selectStar } from '../sqlText.js'
import { applyTabClose } from '../tabClose.js'
import Icon from '../components/Icon.vue'
import Navigator from '../components/Navigator.vue'
import ScriptEditor from '../components/ScriptEditor.vue'
import ObjectView from '../components/ObjectView.vue'
import Overview from './Overview.vue'
import Connections from './Connections.vue'
import Catalog from './Catalog.vue'
import Migrations from './Migrations.vue'
import Backups from './Backups.vue'
import Etl from './Etl.vue'
import Designer from './Designer.vue'
import Monitor from './Monitor.vue'
import Models from './Models.vue'
import Snippets from './Snippets.vue'
import Account from './Account.vue'

const storageKey = 'dbgo-workbench-v1'
const tools = {
  overview: { title: '总览', path: '/overview' },
  connections: { title: '连接', path: '/connections' },
  catalog: { title: '结构', path: '/catalog' },
  migrations: { title: '迁移', path: '/migrations' },
  backups: { title: '备份', path: '/backups' },
  etl: { title: 'ETL', path: '/etl' },
  designer: { title: '对象设计', path: '/designer' },
  monitor: { title: '性能', path: '/monitor' },
  models: { title: '大模型', path: '/models' },
  snippets: { title: '代码段', path: '/snippets' },
  account: { title: '个人中心', path: '/account' }
}
const pathTool = {
  '/': 'script',
  '/query': 'script',
  '/overview': 'overview',
  '/connections': 'connections',
  '/catalog': 'catalog',
  '/migrations': 'migrations',
  '/backups': 'backups',
  '/etl': 'etl',
  '/designer': 'designer',
  '/monitor': 'monitor',
  '/models': 'models',
  '/snippets': 'snippets',
  '/account': 'account'
}

const session = useSession()
const route = useRoute()
const router = useRouter()
const connections = ref([])
const tabs = ref([])
const activeId = ref('')
const revision = ref(0)
const structureRefresh = ref(null)
const selection = ref(null)
const runNote = ref('')
const openMenu = ref('')
const tabMenu = ref(false)
const context = ref(null)
const contextEl = ref(null)
const tabContext = ref(null)
const tabContextEl = ref(null)
const menuBar = ref(null)
const navigatorRef = ref(null)
const passwordDialog = ref(null)
const aboutDialog = ref(null)
const currentPassword = ref('')
const nextPassword = ref('')
const passwordError = ref('')
const navOpen = ref(localStorage.getItem('dbgo-nav-open') !== '0')
const navWidth = ref(Math.min(560, Math.max(200, Number(localStorage.getItem('dbgo-nav-width')) || 280)))
const showResults = ref(true)
const editorMap = new Map()
const zone = new Intl.DateTimeFormat('zh-CN', { timeZoneName: 'short' })
  .formatToParts(new Date())
  .find((part) => part.type === 'timeZoneName')?.value || ''

let scriptSeq = 1
let routing = false
let persist = false
let saveTimer = 0

const activeTab = computed(() => tabs.value.find((tab) => tab.id === activeId.value) || null)
const sqlActive = computed(() => activeTab.value?.kind === 'sql')
const userName = computed(() => session.profile?.display_name || session.profile?.username || '')
const statusConnection = computed(() => {
  const tab = activeTab.value
  const tabId = tab && (tab.kind === 'sql' || tab.kind === 'object') ? tab.connectionId : ''
  const selectedId = selection.value?.connection?.id || ''
  const fallback = [...tabs.value].reverse().find((item) => (item.kind === 'sql' || item.kind === 'object') && item.connectionId)
  const id = tabId || selectedId || fallback?.connectionId || ''
  return connections.value.find((item) => item.id === id) || selection.value?.connection || null
})
const statusSchema = computed(() => {
  const tab = activeTab.value
  if (tab?.kind === 'sql' || tab?.kind === 'object') {
    return tab.schema || ''
  }
  if (selection.value?.schema) {
    return selection.value.schema
  }
  return [...tabs.value].reverse().find((item) => item.kind === 'sql' && item.schema)?.schema || ''
})
const menus = computed(() => [
  {
    id: 'file',
    label: '文件',
    items: [
      { label: '新建 SQL 脚本', run: () => newScript() },
      { label: '新建连接', run: createConnection },
      { label: '导入连接', run: () => openTool('connections', { command: { type: 'import', seq: Date.now() } }) },
      { label: '导出连接', run: () => openTool('connections', { command: { type: 'export', seq: Date.now() } }) },
      { label: '代码段', run: () => openTool('snippets') },
      { type: 'sep' },
      { label: '关闭当前标签', disabled: !activeTab.value, run: () => activeTab.value && closeTab(activeTab.value) },
      { type: 'sep' },
      { label: '退出', run: logout }
    ]
  },
  {
    id: 'edit',
    label: '编辑',
    items: [
      { label: '格式化 SQL', disabled: !sqlActive.value, run: () => editorAction('format') },
      { label: '修改密码', run: openPassword },
      { label: '个人中心', run: () => openTool('account') }
    ]
  },
  {
    id: 'nav',
    label: '导航',
    items: [
      { label: '刷新', run: () => navigatorRef.value?.refresh() },
      { label: '全部折叠', run: () => navigatorRef.value?.collapseAll() },
      { label: '过滤对象', run: focusFilter }
    ]
  },
  {
    id: 'search',
    label: '搜索',
    items: [
      { label: '在导航中查找', run: focusFilter }
    ]
  },
  {
    id: 'sql',
    label: 'SQL 编辑器',
    items: [
      { label: '执行', hint: 'Ctrl+Enter', disabled: !sqlActive.value, run: runActive },
      { label: '格式化', disabled: !sqlActive.value, run: () => editorAction('format') },
      { label: '解释', disabled: !sqlActive.value, run: () => editorAction('explain') },
      { label: 'AI 优化', disabled: !sqlActive.value, run: () => editorAction('optimize') },
      { label: '新建脚本', run: () => newScript() },
      { label: '代码段', run: () => openTool('snippets') },
      { label: sqlActive.value && activeTab.value.readOnly ? '取消只读' : '设为只读', disabled: !sqlActive.value, run: toggleReadOnly },
      { label: showResults.value ? '隐藏结果' : '显示结果', run: () => { showResults.value = !showResults.value } }
    ]
  },
  {
    id: 'database',
    label: '数据库',
    items: [
      { label: '连接管理', run: () => openTool('connections') },
      { label: '结构浏览', run: () => openTool('catalog') },
      { label: '新建表', run: () => design('table') },
      { label: '新建视图', run: () => design('view') },
      { label: '新建存储过程', run: () => design('procedure') },
      { label: '新建函数', run: () => design('function') },
      { label: '性能监控', run: () => openTool('monitor', { connectionId: statusConnection.value?.id || '' }) },
      { label: '大模型', run: () => openTool('models') },
      { label: '代码段', run: () => openTool('snippets') },
      { label: '迁移', run: () => openTool('migrations') },
      { label: '备份', run: () => openTool('backups') },
      { label: 'ETL', run: () => openTool('etl') },
      { type: 'sep' },
      { label: '测试连接', disabled: !statusConnection.value, run: () => statusConnection.value && testConnection(statusConnection.value) },
      { label: '刷新导航', run: () => navigatorRef.value?.refresh() }
    ]
  },
  {
    id: 'window',
    label: '窗口',
    items: [
      { label: (navOpen.value ? '✓ ' : '') + '数据库导航', run: () => { navOpen.value = !navOpen.value } },
      { label: (showResults.value ? '✓ ' : '') + '结果面板', run: () => { showResults.value = !showResults.value } },
      { type: 'sep' },
      { label: '总览', run: () => openTool('overview') }
    ]
  },
  {
    id: 'help',
    label: '帮助',
    items: [
      { label: '关于 DBGo', run: () => aboutDialog.value?.showModal() }
    ]
  }
])
const contextItems = computed(() => {
  const node = context.value?.node
  if (!node) {
    return []
  }
  const items = []
  if (node.kind === 'table' || node.kind === 'view') {
    items.push(
      { label: '查看数据', run: () => openData(node) },
      { label: '查看结构', run: () => openObject(node) },
      { label: '编辑定义', run: () => design(node.kind, node, node.table) },
      { label: '在当前脚本中查询', run: () => fillSelect(node) },
      { type: 'sep' }
    )
  }
  if (node.kind === 'procedure' || node.kind === 'function') {
    items.push({ label: '编辑定义', run: () => design(node.kind, node, node.object) })
  }
  if (node.kind === 'schema' && node.connection) {
    items.push(
      { label: '查看结构', run: () => openTool('catalog', { connectionId: node.connection.id, schema: node.schema || '', focusSeq: Date.now() }) },
      { label: '新建表', run: () => design('table', node) },
      { label: '新建视图', run: () => design('view', node) },
      { label: '新建存储过程', run: () => design('procedure', node) },
      { label: '新建函数', run: () => design('function', node) },
      { label: '性能监控', run: () => openTool('monitor', { connectionId: node.connection.id }) },
      { type: 'sep' }
    )
  }
  if (node.connection && (node.kind === 'connection' || node.kind === 'schema' || node.kind === 'table' || node.kind === 'view')) {
    items.push({ label: '新建 SQL 脚本', run: () => newScript({ connectionId: node.connection.id, schema: node.schema || '' }) })
  }
  if (node.kind === 'connection') {
    items.push(
      { label: '测试连接', run: () => testConnection(node.connection) },
      { label: '性能监控', run: () => openTool('monitor', { connectionId: node.connection.id }) },
      { label: '迁移', run: () => openTool('migrations', { connectionId: node.connection.id }) },
      { label: '备份', run: () => openTool('backups', { connectionId: node.connection.id }) },
      { label: 'ETL', run: () => openTool('etl', { connectionId: node.connection.id }) },
      { type: 'sep' },
      { label: '编辑连接', run: () => editConnection(node.connection) },
      { label: '导出连接', run: () => openTool('connections', { command: { type: 'export', id: node.connection.id, seq: Date.now() } }) },
      { label: '删除连接', danger: true, run: () => removeConnection(node.connection) }
    )
  }
  if (node.load) {
    items.push({ type: 'sep' }, { label: '刷新', run: () => navigatorRef.value?.refresh(node) })
  }
  return items.filter((item, index, list) => !(item.type === 'sep' && (index === 0 || index === list.length - 1 || list[index - 1]?.type === 'sep')))
})

watch(navOpen, (value) => localStorage.setItem('dbgo-nav-open', value ? '1' : '0'))
watch(navWidth, (value) => localStorage.setItem('dbgo-nav-width', String(value)))
watch(activeTab, (tab) => {
  document.title = `${tab?.title || 'DBGo'} - DBGo`
}, { immediate: true })
watch(tabs, scheduleSave, { deep: true })
watch(activeId, scheduleSave)

function scheduleSave() {
  if (!persist) {
    return
  }
  window.clearTimeout(saveTimer)
  saveTimer = window.setTimeout(saveState, 200)
}

function saveState() {
  const payload = {
    seq: scriptSeq,
    activeId: activeId.value,
    tabs: tabs.value.map((tab) => ({ ...tab, command: null, autorun: 0 }))
  }
  sessionStorage.setItem(storageKey, JSON.stringify(payload))
}

function sanitize(tab) {
  if (!tab || typeof tab !== 'object') {
    return null
  }
  if (tab.kind === 'sql') {
    const match = /^script-(\d+)$/.exec(String(tab.id || ''))
    if (match) {
      scriptSeq = Math.max(scriptSeq, Number(match[1]) + 1)
    }
    return {
      id: String(tab.id || `script-${scriptSeq++}`),
      kind: 'sql',
      title: String(tab.title || 'Script'),
      closable: true,
      connectionId: String(tab.connectionId || ''),
      schema: String(tab.schema || ''),
      sql: String(tab.sql || ''),
      readOnly: tab.readOnly !== false,
      maxRows: Number(tab.maxRows || 100),
      autorun: 0
    }
  }
  if (tab.kind === 'object' && tab.table && tab.connectionId) {
    return {
      id: String(tab.id),
      kind: 'object',
      title: String(tab.title || tab.table),
      closable: true,
      connectionId: String(tab.connectionId),
      schema: String(tab.schema || ''),
      table: String(tab.table),
      driver: String(tab.driver || '')
    }
  }
  if (tab.kind === 'tool' && tools[tab.tool]) {
    return {
      id: `tool-${tab.tool}`,
      kind: 'tool',
      tool: tab.tool,
      title: tools[tab.tool].title,
      closable: true,
      preferredConnection: String(tab.preferredConnection || ''),
      preferredSchema: String(tab.preferredSchema || ''),
      focusSeq: 0,
      command: null
    }
  }
  return null
}

function restore() {
  try {
    const raw = sessionStorage.getItem(storageKey)
    if (!raw) {
      return false
    }
    const data = JSON.parse(raw)
    const next = (data.tabs || []).map(sanitize).filter(Boolean)
    if (!next.length) {
      return false
    }
    tabs.value = next
    activeId.value = next.some((tab) => tab.id === data.activeId) ? data.activeId : next[0].id
    scriptSeq = Math.max(scriptSeq, Number(data.seq) || 1)
    return true
  } catch {
    return false
  }
}

function go(path) {
  if (!path || route.path === path) {
    return
  }
  routing = true
  router.push(path).finally(() => {
    routing = false
  })
}

function newScript(partial = {}) {
  const number = scriptSeq++
  const connectionId = partial.connectionId || selection.value?.connection?.id || connections.value[0]?.id || ''
  let schema = partial.schema || ''
  if (!partial.schema && !partial.connectionId && selection.value?.schema && selection.value.connection?.id === connectionId) {
    schema = selection.value.schema
  }
  const tab = {
    id: `script-${number}`,
    kind: 'sql',
    title: partial.title || `Script-${number}`,
    closable: true,
    connectionId,
    schema,
    sql: partial.sql || '',
    readOnly: partial.readOnly !== false,
    maxRows: 100,
    autorun: partial.autorun || 0
  }
  tabs.value.push(tab)
  activeId.value = tab.id
  if (!partial.skipRoute) {
    go('/query')
  }
  return tab
}

function toolTitle(tool, command) {
  if (command?.kind === 'procedure') {
    return command.name || '存储过程'
  }
  if (command?.kind === 'function') {
    return command.name || '函数'
  }
  return tools[tool]?.title || tool
}

function openTool(tool, opts = {}) {
  const meta = tools[tool]
  if (!meta) {
    return
  }
  let tab = tabs.value.find((item) => item.kind === 'tool' && item.tool === tool)
  if (!tab) {
    tab = {
      id: `tool-${tool}`,
      kind: 'tool',
      tool,
      title: toolTitle(tool, opts.command),
      closable: true,
      preferredConnection: opts.connectionId || '',
      preferredSchema: opts.schema || '',
      focusSeq: opts.focusSeq || 0,
      command: opts.command || null
    }
    tabs.value.push(tab)
  } else {
    if (opts.connectionId) {
      tab.preferredConnection = opts.connectionId
    }
    if (Object.prototype.hasOwnProperty.call(opts, 'schema')) {
      tab.preferredSchema = opts.schema || ''
    }
    if (opts.focusSeq) {
      tab.focusSeq = opts.focusSeq
    }
    if (opts.command) {
      tab.command = opts.command
      tab.title = toolTitle(tool, opts.command)
    }
  }
  activeId.value = tab.id
  if (!opts.fromRoute) {
    go(meta.path)
  }
}

function onCatalogSchema(name) {
  const tab = tabs.value.find((item) => item.kind === 'tool' && item.tool === 'catalog')
  if (tab && tab.preferredSchema !== (name || '')) {
    tab.preferredSchema = name || ''
  }
}

function applyRoute(path) {
  const tool = pathTool[path] || 'script'
  if (tool === 'script') {
    const current = tabs.value.find((tab) => tab.id === activeId.value)
    if (current && current.kind !== 'tool') {
      return
    }
    const script = [...tabs.value].reverse().find((tab) => tab.kind === 'sql' || tab.kind === 'object')
    if (script) {
      activeId.value = script.id
      return
    }
    newScript({ skipRoute: true })
    return
  }
  openTool(tool, { fromRoute: true })
}

function activate(tab) {
  activeId.value = tab.id
  tabMenu.value = false
  if (tab.kind === 'tool') {
    go(tools[tab.tool].path)
    return
  }
  if (route.path !== '/' && route.path !== '/query') {
    go('/query')
  }
}

function closeTab(tab) {
  closeTabs(tab?.id, 'one')
}

function closeTabs(targetId, mode) {
  tabContext.value = null
  const before = activeId.value
  const next = applyTabClose(tabs.value, before, targetId, mode)
  const keep = new Set(next.tabs.map((tab) => tab.id))
  for (const tab of tabs.value) {
    if (!keep.has(tab.id)) {
      editorMap.delete(tab.id)
    }
  }
  tabs.value = next.tabs
  if (!next.activeId) {
    activeId.value = ''
    go('/')
    return
  }
  if (next.activeId !== before) {
    const tab = next.tabs.find((item) => item.id === next.activeId)
    if (tab) {
      activate(tab)
    }
  }
}

function openTabMenu(event, tab) {
  event.preventDefault()
  context.value = null
  openMenu.value = ''
  tabMenu.value = false
  const index = tabs.value.findIndex((item) => item.id === tab.id)
  tabContext.value = {
    x: event.clientX,
    y: event.clientY,
    items: [
      { label: '关闭', run: () => closeTabs(tab.id, 'one') },
      { label: '关闭其他窗口', disabled: tabs.value.length < 2, run: () => closeTabs(tab.id, 'others') },
      { label: '关闭全部', run: () => closeTabs(tab.id, 'all') },
      { label: '关闭左边', disabled: index <= 0, run: () => closeTabs(tab.id, 'left') },
      { label: '关闭右边', disabled: index < 0 || index >= tabs.value.length - 1, run: () => closeTabs(tab.id, 'right') }
    ]
  }
  nextTick(() => {
    const el = tabContextEl.value
    if (!el || !tabContext.value) {
      return
    }
    const rect = el.getBoundingClientRect()
    tabContext.value = {
      ...tabContext.value,
      x: rect.right > window.innerWidth ? Math.max(8, window.innerWidth - rect.width - 8) : tabContext.value.x,
      y: rect.bottom > window.innerHeight ? Math.max(8, window.innerHeight - rect.height - 8) : tabContext.value.y
    }
  })
}

function runTabContext(item) {
  if (item.disabled) {
    return
  }
  const action = item.run
  tabContext.value = null
  action()
}

function bindEditor(id, el) {
  if (el) {
    editorMap.set(id, el)
  } else {
    editorMap.delete(id)
  }
}

function runActive() {
  const tab = activeTab.value
  if (tab?.kind === 'sql') {
    editorMap.get(tab.id)?.run()
  }
}

function toggleReadOnly() {
  const tab = activeTab.value
  if (tab?.kind === 'sql') {
    tab.readOnly = !tab.readOnly
  }
}

function tabIcon(tab) {
  if (tab.kind === 'object') return 'table'
  if (tab.kind === 'sql') return 'sql'
  if (tab.tool === 'connections') return 'database'
  if (tab.tool === 'migrations') return 'migration'
  if (tab.tool === 'backups') return 'backup'
  if (tab.tool === 'etl') return 'etl'
  if (tab.tool === 'catalog' || tab.tool === 'designer') return 'catalog'
  if (tab.tool === 'monitor') return 'overview'
  if (tab.tool === 'models' || tab.tool === 'snippets') return 'sql'
  if (tab.tool === 'account') return 'overview'
  return 'overview'
}

function onSelect(node) {
  selection.value = node
  const tab = activeTab.value
  if (!tab || tab.kind !== 'sql' || !node.connection) {
    return
  }
  if (tab.connectionId !== node.connection.id) {
    tab.schema = node.schema || ''
    tab.connectionId = node.connection.id
  } else if (node.schema) {
    tab.schema = node.schema
  }
}

function openData(node) {
  newScript({
    title: node.table,
    connectionId: node.connection.id,
    schema: node.schema || '',
    sql: selectStar(node.connection.driver, node.schema, node.table),
    readOnly: true,
    autorun: 1
  })
}

function openObject(node) {
  const id = `object-${node.connection.id}-${node.schema}-${node.table}`
  let tab = tabs.value.find((item) => item.id === id)
  if (!tab) {
    tab = {
      id,
      kind: 'object',
      title: node.table,
      closable: true,
      connectionId: node.connection.id,
      schema: node.schema || '',
      table: node.table,
      driver: node.connection.driver
    }
    tabs.value.push(tab)
  }
  activeId.value = tab.id
  if (route.path !== '/' && route.path !== '/query') {
    go('/query')
  }
}

function queryObject(tab) {
  const connection = connections.value.find((item) => item.id === tab.connectionId)
  newScript({
    title: tab.table,
    connectionId: tab.connectionId,
    schema: tab.schema,
    sql: selectStar(connection?.driver || tab.driver, tab.schema, tab.table),
    readOnly: true,
    autorun: 1
  })
}

function insertSnippet(body) {
  if (!body) {
    return
  }
  const tab = activeTab.value?.kind === 'sql' ? activeTab.value : newScript({ skipRoute: true })
  tab.sql = tab.sql ? `${tab.sql.replace(/\s*$/, '')}\n${body}` : body
  activeId.value = tab.id
  if (route.path !== '/' && route.path !== '/query') {
    go('/query')
  }
}

function fillSelect(node) {
  const sql = selectStar(node.connection.driver, node.schema, node.table)
  const tab = activeTab.value?.kind === 'sql' ? activeTab.value : newScript({ connectionId: node.connection.id, schema: node.schema || '', skipRoute: true })
  tab.schema = node.schema || ''
  tab.connectionId = node.connection.id
  tab.sql = tab.sql ? `${tab.sql.replace(/\s*$/, '')}\n${sql}` : sql
  activeId.value = tab.id
  if (route.path !== '/' && route.path !== '/query') {
    go('/query')
  }
}

async function testConnection(row) {
  runNote.value = `正在测试 ${row.name}`
  try {
    const result = (await api('/api/connection/test', { id: row.id })).data
    runNote.value = `${result.name} 可连接，版本 ${result.server_version || '—'}，耗时 ${result.latency_ms ?? 0} ms`
  } catch (err) {
    runNote.value = err.message
  }
}

async function removeConnection(row) {
  if (!window.confirm(`删除连接「${row.name}」？已保存的查询历史和备份记录会保留。`)) {
    return
  }
  try {
    await api('/api/connection/delete', { id: row.id })
    if (selection.value?.connection?.id === row.id) {
      selection.value = null
    }
    await reloadConnections()
    runNote.value = `已删除连接 ${row.name}`
  } catch (err) {
    runNote.value = err.message
  }
}

function design(kind, node, objectName = '') {
  const connection = node?.connection || statusConnection.value
  openTool('designer', {
    connectionId: connection?.id || '',
    command: {
      type: objectName ? 'edit' : 'create',
      kind,
      connectionId: connection?.id || '',
      schema: node?.schema || statusSchema.value || '',
      name: objectName || '',
      seq: Date.now()
    }
  })
}

function editorAction(name) {
  const tab = activeTab.value
  if (tab?.kind !== 'sql') {
    return
  }
  editorMap.get(tab.id)?.[name]?.()
}

function createConnection() {
  openTool('connections', { command: { type: 'create', seq: Date.now() } })
}

function editConnection(row) {
  openTool('connections', {
    connectionId: row.id,
    command: { type: 'edit', id: row.id, seq: Date.now() }
  })
}

function syncObjectTabs(info) {
  const connectionId = String(info.connectionId || '')
  const previousSchema = info.previousSchema || ''
  const nextSchema = info.schema || ''
  const previousName = info.previousName || ''
  const nextName = info.name || ''
  if (!connectionId || !previousName || !nextName) {
    return
  }
  const removed = []
  for (const tab of tabs.value) {
    if (tab.kind !== 'object' || String(tab.connectionId) !== connectionId) {
      continue
    }
    const onOld = (tab.schema || '') === previousSchema && tab.table === previousName
    const onNew = (tab.schema || '') === nextSchema && tab.table === nextName
    if (!onOld && !onNew) {
      continue
    }
    if (onOld && (tab.table !== nextName || (tab.schema || '') !== nextSchema)) {
      const nextId = `object-${connectionId}-${nextSchema}-${nextName}`
      const clash = tabs.value.find((item) => item !== tab && item.id === nextId)
      if (activeId.value === tab.id) {
        activeId.value = clash ? clash.id : nextId
      }
      if (clash) {
        clash.stamp = (clash.stamp || 0) + 1
        removed.push(tab)
        continue
      }
      tab.id = nextId
      tab.schema = nextSchema
      tab.table = nextName
      tab.title = nextName
    }
    tab.stamp = (tab.stamp || 0) + 1
  }
  if (removed.length) {
    tabs.value = tabs.value.filter((item) => !removed.includes(item))
  }
}

async function onApplied(info) {
  const node = await navigatorRef.value?.refresh(undefined, info)
  if (node) {
    selection.value = node
  }
  if (!info?.name) {
    return
  }
  structureRefresh.value = { ...info, seq: Date.now() }
  syncObjectTabs(info)
}

async function reloadConnections() {
  connections.value = await listConnections()
  revision.value += 1
  for (const tab of tabs.value) {
    if (tab.kind === 'sql' && !tab.connectionId && connections.value[0]) {
      tab.connectionId = connections.value[0].id
    }
  }
}

function focusFilter() {
  navOpen.value = true
  nextTick(() => navigatorRef.value?.focusFilter())
}

function clickMenu(id) {
  openMenu.value = openMenu.value === id ? '' : id
  context.value = null
  tabContext.value = null
  tabMenu.value = false
}

function hoverMenu(id) {
  if (openMenu.value) {
    openMenu.value = id
  }
}

function runItem(item) {
  openMenu.value = ''
  if (!item.disabled) {
    item.run()
  }
}

async function onContext(payload) {
  openMenu.value = ''
  tabMenu.value = false
  context.value = { x: payload.x, y: payload.y, node: payload.node }
  await nextTick()
  if (!contextItems.value.length) {
    context.value = null
    return
  }
  const el = contextEl.value
  if (!el || !context.value) {
    return
  }
  const rect = el.getBoundingClientRect()
  context.value = {
    ...context.value,
    x: rect.right > window.innerWidth ? Math.max(8, window.innerWidth - rect.width - 8) : context.value.x,
    y: rect.bottom > window.innerHeight ? Math.max(8, window.innerHeight - rect.height - 8) : context.value.y
  }
}

function runContext(item) {
  context.value = null
  item.run()
}

function startNavResize(event) {
  const startX = event.clientX
  const startW = navWidth.value
  function move(next) {
    navWidth.value = Math.min(560, Math.max(200, startW + next.clientX - startX))
  }
  function up() {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}

function onPointer(event) {
  if (openMenu.value && menuBar.value && !menuBar.value.contains(event.target)) {
    openMenu.value = ''
  }
  if (tabMenu.value && !event.target.closest?.('.tab-menu') && !event.target.closest?.('.tab-more')) {
    tabMenu.value = false
  }
  if (context.value && contextEl.value && !contextEl.value.contains(event.target)) {
    context.value = null
  }
  if (tabContext.value && tabContextEl.value && !tabContextEl.value.contains(event.target)) {
    tabContext.value = null
  }
}

function onKeydown(event) {
  if (event.key === 'Escape') {
    openMenu.value = ''
    context.value = null
    tabContext.value = null
    tabMenu.value = false
  }
}

function openPassword() {
  passwordError.value = ''
  currentPassword.value = ''
  nextPassword.value = ''
  passwordDialog.value?.showModal()
}

async function changePassword() {
  passwordError.value = ''
  try {
    await api('/api/auth/password', {
      current_password: currentPassword.value,
      new_password: nextPassword.value
    })
    session.profile = null
    await router.push('/login')
  } catch (err) {
    passwordError.value = err.message
  }
}

async function logout() {
  await session.logout()
  await router.push('/login')
}

function onUnauthorized() {
  session.profile = null
  if (route.path !== '/login') {
    router.push('/login')
  }
}

watch(() => route.path, (path) => {
  if (!routing) {
    applyRoute(path)
  }
})

onMounted(async () => {
  window.addEventListener('dbgo-unauthorized', onUnauthorized)
  window.addEventListener('pointerdown', onPointer)
  window.addEventListener('keydown', onKeydown)
  if (!restore()) {
    newScript({ skipRoute: true })
  }
  try {
    await reloadConnections()
  } catch (err) {
    runNote.value = err.message
  }
  applyRoute(route.path)
  persist = true
})

onBeforeUnmount(() => {
  window.removeEventListener('dbgo-unauthorized', onUnauthorized)
  window.removeEventListener('pointerdown', onPointer)
  window.removeEventListener('keydown', onKeydown)
  window.clearTimeout(saveTimer)
})
</script>

<template>
  <div class="wb">
    <header ref="menuBar" class="menubar" @mouseleave="openMenu = ''">
      <div v-for="menu in menus" :key="menu.id" class="menu" @mouseenter="hoverMenu(menu.id)">
        <button type="button" class="menu-label" :class="{ open: openMenu === menu.id }" @click="clickMenu(menu.id)">{{ menu.label }}</button>
        <div v-if="openMenu === menu.id" class="menu-pop" role="menu">
          <template v-for="(item, index) in menu.items" :key="menu.id + index">
            <hr v-if="item.type === 'sep'" />
            <button v-else type="button" role="menuitem" :disabled="item.disabled" @click="runItem(item)">
              <span>{{ item.label }}</span>
              <span v-if="item.hint" class="menu-hint">{{ item.hint }}</span>
            </button>
          </template>
        </div>
      </div>
      <div class="menu-spacer"></div>
      <div class="menu user-menu">
        <button type="button" class="menu-label" @click="clickMenu('user')">{{ userName || '账号' }}</button>
        <div v-if="openMenu === 'user'" class="menu-pop user-pop" role="menu">
          <button type="button" role="menuitem" @click="runItem({ run: () => openTool('account') })">个人中心</button>
          <button type="button" role="menuitem" @click="runItem({ run: openPassword })">修改密码</button>
          <button type="button" role="menuitem" @click="runItem({ run: logout })">退出</button>
        </div>
      </div>
    </header>

    <div class="toolbar-main">
      <button class="icon-btn" type="button" title="新建 SQL 脚本" @click="newScript()"><Icon name="sql" /></button>
      <button class="icon-btn" type="button" title="新建连接" @click="createConnection"><Icon name="database" /></button>
      <button class="icon-btn" type="button" title="刷新导航" @click="navigatorRef?.refresh()"><Icon name="refresh" /></button>
      <span class="tb-sep"></span>
      <button class="icon-btn tb-play" type="button" title="执行 SQL（Ctrl+Enter）" :disabled="!sqlActive" @click="runActive"><Icon name="play" /></button>
      <span class="tb-sep"></span>
      <button class="icon-btn" type="button" title="总览" @click="openTool('overview')"><Icon name="overview" /></button>
      <button class="icon-btn" type="button" title="连接" @click="openTool('connections')"><Icon name="database" /></button>
      <button class="icon-btn" type="button" title="结构" @click="openTool('catalog')"><Icon name="catalog" /></button>
      <button class="icon-btn" type="button" title="迁移" @click="openTool('migrations')"><Icon name="migration" /></button>
      <button class="icon-btn" type="button" title="备份" @click="openTool('backups')"><Icon name="backup" /></button>
      <button class="icon-btn" type="button" title="ETL" @click="openTool('etl')"><Icon name="etl" /></button>
      <button class="icon-btn" type="button" title="对象设计" @click="design('table')"><Icon name="add" /></button>
      <button class="icon-btn" type="button" title="性能" @click="openTool('monitor', { connectionId: statusConnection?.id || '' })"><Icon name="overview" /></button>
      <button class="icon-btn" type="button" title="大模型" @click="openTool('models')"><Icon name="sql" /></button>
      <button class="icon-btn" type="button" title="代码段" @click="openTool('snippets')"><Icon name="sql" /></button>
      <span class="tb-sep"></span>
      <button class="icon-btn" type="button" :title="navOpen ? '隐藏数据库导航' : '显示数据库导航'" @click="navOpen = !navOpen"><Icon name="nav" /></button>
    </div>

    <div class="wb-body">
      <Navigator
        v-show="navOpen"
        ref="navigatorRef"
        class="wb-nav"
        :style="{ width: navWidth + 'px' }"
        :connections="connections"
        @select="onSelect"
        @context="onContext"
        @open-data="openData"
        @open-design="(node) => design(node.kind, node, node.object || node.table)"
        @reload="reloadConnections"
        @collapse="navOpen = false"
        @create-connection="createConnection"
      />
      <div v-if="navOpen" class="splitter-v" @mousedown="startNavResize"></div>
      <section class="wb-main">
        <div class="wb-tabs" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            type="button"
            role="tab"
            class="wb-tab"
            :class="{ active: tab.id === activeId }"
            :aria-selected="tab.id === activeId"
            :title="tab.title"
            @click="activate(tab)"
            @contextmenu="openTabMenu($event, tab)"
          >
            <Icon :name="tabIcon(tab)" />
            <span>{{ tab.title }}</span>
            <span v-if="tab.closable" class="tab-x" title="关闭" @click.stop="closeTab(tab)">×</span>
          </button>
          <span class="tab-gap"></span>
          <button class="icon-btn tab-more" type="button" title="打开的编辑器" @click="tabMenu = !tabMenu">▾</button>
          <div v-if="tabMenu" class="menu-pop tab-menu">
            <button v-for="tab in tabs" :key="'list-' + tab.id" type="button" @click="activate(tab)">{{ tab.title }}</button>
            <p v-if="!tabs.length" class="muted">没有打开的标签</p>
          </div>
        </div>
        <div class="wb-stage">
          <div v-if="!activeTab" class="wb-empty">
            <p>没有打开的编辑器</p>
            <button class="btn btn-primary" type="button" @click="newScript()">新建 SQL 脚本</button>
          </div>
          <div v-for="tab in tabs" v-show="tab.id === activeId" :key="tab.id" class="wb-pane">
            <ScriptEditor
              v-if="tab.kind === 'sql'"
              :ref="(el) => bindEditor(tab.id, el)"
              :tab="tab"
              :connections="connections"
              :show-results="showResults"
              @ran="runNote = $event"
              @close-results="showResults = false"
              @manage-snippets="openTool('snippets')"
            />
            <ObjectView
              v-else-if="tab.kind === 'object'"
              :connection-id="tab.connectionId"
              :schema="tab.schema"
              :table="tab.table"
              :driver="tab.driver"
              :stamp="tab.stamp || 0"
              @query="queryObject(tab)"
            />
            <div v-else class="tool-scroll">
              <Overview v-if="tab.tool === 'overview'" />
              <Connections v-else-if="tab.tool === 'connections'" :command="tab.command" :revision="revision" @changed="reloadConnections" />
              <Catalog v-else-if="tab.tool === 'catalog'" :preferred-connection="tab.preferredConnection" :preferred-schema="tab.preferredSchema || ''" :focus-seq="tab.focusSeq || 0" :revision="revision" :structure-refresh="structureRefresh" @schema="onCatalogSchema" />
              <Migrations v-else-if="tab.tool === 'migrations'" :preferred-connection="tab.preferredConnection" :revision="revision" />
              <Backups v-else-if="tab.tool === 'backups'" :preferred-connection="tab.preferredConnection" :revision="revision" />
              <Etl v-else-if="tab.tool === 'etl'" :preferred-connection="tab.preferredConnection" :revision="revision" />
              <Designer v-else-if="tab.tool === 'designer'" :command="tab.command" :preferred-connection="tab.preferredConnection" :connections="connections" @applied="onApplied" />
              <Monitor v-else-if="tab.tool === 'monitor'" :preferred-connection="tab.preferredConnection" :connections="connections" :revision="revision" />
              <Models v-else-if="tab.tool === 'models'" />
              <Snippets v-else-if="tab.tool === 'snippets'" @insert="insertSnippet" />
              <Account v-else-if="tab.tool === 'account'" />
            </div>
          </div>
        </div>
      </section>
    </div>

    <footer class="statusbar">
      <span>{{ statusConnection ? '数据库: ' + statusConnection.name : '数据库: 未选择' }}</span>
      <span v-if="statusConnection" class="status-target">{{ connectionTarget(statusConnection) }}</span>
      <span class="status-note">{{ runNote }}</span>
      <span class="status-right">
        <span v-if="statusSchema">{{ statusSchema }}</span>
        <span v-if="statusConnection">{{ driverLabel(statusConnection.driver) }}</span>
        <span>{{ zone }}</span>
        <span>zh_CN</span>
      </span>
    </footer>

    <div v-if="tabContext" ref="tabContextEl" class="ctx" :style="{ left: tabContext.x + 'px', top: tabContext.y + 'px' }" role="menu">
      <button v-for="(item, index) in tabContext.items" :key="index" type="button" :disabled="item.disabled" @click="runTabContext(item)">{{ item.label }}</button>
    </div>

    <div v-if="context" ref="contextEl" class="ctx" :style="{ left: context.x + 'px', top: context.y + 'px' }" role="menu">
      <template v-for="(item, index) in contextItems" :key="index">
        <hr v-if="item.type === 'sep'" />
        <button v-else type="button" :class="{ danger: item.danger }" @click="runContext(item)">{{ item.label }}</button>
      </template>
    </div>

    <dialog ref="passwordDialog" class="modal">
      <form class="stack" @submit.prevent="changePassword">
        <h2>修改密码</h2>
        <p class="muted">保存后当前会话会失效，需要重新登录。</p>
        <p v-if="passwordError" class="notice">{{ passwordError }}</p>
        <label class="field"><span>当前密码</span><input v-model="currentPassword" type="password" required /></label>
        <label class="field"><span>新密码</span><input v-model="nextPassword" type="password" minlength="8" required /></label>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存并重新登录</button>
          <button class="btn" type="button" @click="passwordDialog.close()">取消</button>
        </div>
      </form>
    </dialog>

    <router-view />

    <dialog ref="aboutDialog" class="modal">
      <form class="stack" @submit.prevent="aboutDialog.close()">
        <h2>关于 DBGo</h2>
        <p>网页版数据库工作台。可以管理连接、设计表和例程、看性能、并用 OpenAI 协议的模型优化 SQL。</p>
        <p class="muted">支持 SQLite、MySQL、PostgreSQL 和 SQL Server。查询默认只读，写入前取消只读。每个账号只能看到自己创建的连接。</p>
        <div class="actions">
          <button class="btn" type="submit">关闭</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
