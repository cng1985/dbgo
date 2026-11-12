<script setup>
import { computed, nextTick, provide, reactive, ref, watch } from 'vue'
import { api, connectionTarget } from '../api'
import { nodeShown } from '../navTree.js'
import { markMatches, mergeTreeChildren, nodeMark, planTreeRefresh } from '../treeState.js'
import Icon from './Icon.vue'
import TreeNode from './TreeNode.vue'

const props = defineProps({
  connections: { type: Array, default: () => [] }
})
const emit = defineEmits(['select', 'context', 'open-data', 'open-design', 'reload', 'collapse', 'create-connection'])

const filter = ref('')
const filterInput = ref(null)
const treeBox = ref(null)
const selectedId = ref('')
const selectedConnectionId = ref('')
const roots = ref([])
const filterMiss = computed(() => {
  const query = filter.value.trim()
  return !!query && !!roots.value.length && !roots.value.some((node) => nodeShown(node, query))
})

function folder(id, label, children, place = {}, open = false) {
  return {
    id,
    kind: 'folder',
    folder: place.folder || '',
    label,
    meta: String(children.length),
    open,
    loaded: true,
    children,
    connection: place.connection,
    schema: place.schema || '',
    table: place.table || ''
  }
}

function tableNode(connection, schema, table) {
  const view = /view/i.test(table.type || '')
  return {
    id: `t-${connection.id}-${schema}-${table.name}`,
    kind: view ? 'view' : 'table',
    label: table.name,
    connection,
    schema,
    table: table.name,
    open: false,
    loaded: false,
    children: [],
    async load() {
      const body = { connection_id: connection.id, schema, table: table.name }
      const columns = (await api('/api/catalog/columns', body)).data.columns || []
      const indexes = (await api('/api/catalog/indexes', body)).data.indexes || []
      const cols = columns.map((column) => ({
        id: `col-${connection.id}-${schema}-${table.name}-${column.name}`,
        kind: column.primary ? 'key' : 'column',
        label: column.name,
        meta: [column.type, column.comment].filter(Boolean).join(' · '),
        leaf: true,
        connection,
        schema,
        table: table.name
      }))
      const indexNodes = indexes.map((item) => ({
        id: `idx-${connection.id}-${schema}-${table.name}-${item.name}`,
        kind: 'index',
        label: item.name,
        meta: item.unique ? '唯一' : (item.columns || ''),
        leaf: true,
        connection,
        schema,
        table: table.name
      }))
      const place = { connection, schema, table: table.name }
      return [
        folder(`cols-${connection.id}-${schema}-${table.name}`, '列', cols, { ...place, folder: 'columns' }),
        folder(`idxs-${connection.id}-${schema}-${table.name}`, '索引', indexNodes, { ...place, folder: 'indexes' })
      ]
    }
  }
}

function routineNode(connection, schema, item) {
  return {
    id: `r-${connection.id}-${schema}-${item.kind}-${item.name}`,
    kind: item.kind,
    label: item.name,
    connection,
    schema,
    object: item.name,
    leaf: true
  }
}

function schemaNode(connection, name) {
  return {
    id: `s-${connection.id}-${name}`,
    kind: 'schema',
    label: name,
    connection,
    schema: name,
    open: false,
    loaded: false,
    children: [],
    async load() {
      const tables = (await api('/api/catalog/tables', {
        connection_id: connection.id,
        schema: name
      })).data.tables || []
      const base = tables.filter((item) => !/view/i.test(item.type || ''))
      const views = tables.filter((item) => /view/i.test(item.type || ''))
      let routines = []
      let routineSupported = false
      try {
        const body = (await api('/api/catalog/objects', {
          connection_id: connection.id,
          schema: name,
          kind: 'routine'
        })).data
        routines = body.objects || []
        routineSupported = body.supported !== false
      } catch {
        routines = []
      }
      const place = { connection, schema: name }
      const children = [
        folder(`tables-${connection.id}-${name}`, '表', base.map((item) => tableNode(connection, name, item)), { ...place, folder: 'tables' }),
        folder(`views-${connection.id}-${name}`, '视图', views.map((item) => tableNode(connection, name, item)), { ...place, folder: 'views' })
      ]
      if (routineSupported) {
        children.push(
          folder(`procs-${connection.id}-${name}`, '存储过程', routines.filter((item) => item.kind === 'procedure').map((item) => routineNode(connection, name, item)), { ...place, folder: 'procedures' }),
          folder(`funcs-${connection.id}-${name}`, '函数', routines.filter((item) => item.kind === 'function').map((item) => routineNode(connection, name, item)), { ...place, folder: 'functions' })
        )
      }
      return children
    }
  }
}

function connectionSignature(row) {
  return [row.driver, row.host, row.port, row.database_name, row.username, row.sqlite_path, row.ssl_mode].join('\n')
}

function connectionNode(row, previous) {
  if (previous) {
    const signature = connectionSignature(row)
    if (previous.signature !== signature) {
      previous.offline = false
      previous.failure = ''
      previous.loaded = false
      previous.open = false
      previous.children = []
      previous.error = ''
    }
    previous.signature = signature
    previous.label = row.name
    previous.meta = connectionTarget(row)
    previous.connection = row
    return previous
  }
  return {
    id: `c-${row.id}`,
    kind: 'connection',
    label: row.name,
    meta: connectionTarget(row),
    connection: row,
    signature: connectionSignature(row),
    offline: false,
    failure: '',
    open: false,
    loaded: false,
    children: [],
    async load() {
      const schemas = (await api('/api/catalog/schemas', { connection_id: row.id })).data.schemas || []
      return [
        folder(
          `dbs-${row.id}`,
          '数据库',
          schemas.map((item) => schemaNode(row, item.name)),
          { folder: 'databases', connection: row },
          true
        )
      ]
    }
  }
}

watch(() => props.connections, (list) => {
  const previous = new Map(roots.value.map((node) => [node.connection?.id, node]))
  const seen = new Set((list || []).map((row) => row.id))
  roots.value = (list || []).map((row) => connectionNode(row, previous.get(row.id)))
  const alive = new Set([...seen].map((id) => String(id)))
  if (!alive.size || (selectedConnectionId.value && !alive.has(selectedConnectionId.value))) {
    selectedId.value = ''
    selectedConnectionId.value = ''
  }
}, { immediate: true })

function* walk(nodes) {
  for (const node of nodes || []) {
    yield node
    yield* walk(node.children)
  }
}

function markOffline(node) {
  node.offline = true
  node.failure = '无法连接'
  node.open = false
  node.loaded = false
  node.children = []
  node.error = ''
}

function failLoad(node, err) {
  if (node.kind === 'connection') {
    markOffline(node)
    return
  }
  node.error = err.message
}

async function ensure(node) {
  if (!node || node.leaf || node.loaded || !node.load) {
    return
  }
  node.loading = true
  node.error = ''
  try {
    node.children = await node.load()
    node.loaded = true
    node.offline = false
    node.failure = ''
  } catch (err) {
    failLoad(node, err)
  } finally {
    node.loading = false
  }
}

async function toggle(node) {
  if (!node || node.leaf) {
    return
  }
  select(node)
  node.open = !node.open
  if (node.open) {
    await ensure(node)
  }
}

function select(node) {
  selectedId.value = node.id
  if (node.connection?.id != null && node.connection.id !== '') {
    selectedConnectionId.value = String(node.connection.id)
  }
  emit('select', node)
}

function openNode(node) {
  if (node.kind === 'table' || node.kind === 'view') {
    emit('open-data', node)
    return
  }
  if (node.kind === 'procedure' || node.kind === 'function') {
    emit('open-design', node)
    return
  }
  toggle(node)
}

function context(event, node) {
  event.preventDefault()
  select(node)
  emit('context', { x: event.clientX, y: event.clientY, node })
}

function applyOpen(nodes, plan) {
  for (const child of nodes || []) {
    if (child.leaf) {
      continue
    }
    child.open = !!plan?.open?.some((mark) => markMatches(child, mark))
    if (!child.open) {
      continue
    }
    if (child.load) {
      child.loading = true
    }
    if (child.children?.length) {
      applyOpen(child.children, plan)
    }
  }
}

async function reloadNode(node, plan) {
  if (!node?.load) {
    return true
  }
  const quiet = !!node.loaded
  if (!quiet) {
    node.loading = true
    node.error = ''
  }
  try {
    const children = await node.load()
    applyOpen(children, plan)
    node.children = quiet ? mergeTreeChildren(node.children, children) : children
    node.loaded = true
    node.offline = false
    node.failure = ''
    node.error = ''
    return true
  } catch (err) {
    failLoad(node, err)
    return false
  } finally {
    node.loading = false
  }
}

function findMarked(mark) {
  if (!mark) {
    return null
  }
  for (const item of walk(roots.value)) {
    if (markMatches(item, mark)) {
      return item
    }
  }
  return null
}

async function restore(nodes, plan) {
  const pending = []
  for (const node of nodes || []) {
    if (node.leaf) {
      continue
    }
    const shouldOpen = plan.open.some((mark) => markMatches(node, mark))
    if (shouldOpen) {
      node.open = true
      pending.push(node)
      continue
    }
    if (node.open) {
      node.open = false
    }
    if (node.load && node.loaded) {
      node.loaded = false
    }
  }
  for (const node of pending) {
    if (node.load) {
      const ok = await reloadNode(node, plan)
      if (!ok) {
        continue
      }
    }
    await restore(node.children, plan)
  }
}

async function refresh(node, change) {
  const scroller = treeBox.value
  const scrollTop = scroller ? scroller.scrollTop : 0
  const open = []
  let selected = null
  for (const item of walk(roots.value)) {
    if (item.open) {
      open.push(nodeMark(item))
    }
    if (item.id === selectedId.value) {
      selected = nodeMark(item)
    }
  }
  const plan = planTreeRefresh(open, selected, change)
  const target = node && node.load ? node : null
  if (target) {
    if (!plan.open.some((mark) => markMatches(target, mark))) {
      plan.open.push(nodeMark(target))
    }
    await restore([target], plan)
  } else {
    emit('reload')
    await restore(roots.value, plan)
  }
  const selectedNode = findMarked(plan.selected)
  if (selectedNode) {
    selectedId.value = selectedNode.id
    if (selectedNode.connection?.id != null && selectedNode.connection.id !== '') {
      selectedConnectionId.value = String(selectedNode.connection.id)
    }
  }
  await nextTick()
  if (scroller) {
    scroller.scrollTop = scrollTop
  }
  return selectedNode
}

function collapseAll() {
  for (const node of walk(roots.value)) {
    node.open = false
  }
}

function focusFilter() {
  filterInput.value?.focus()
  filterInput.value?.select()
}

const tree = reactive({
  selectedId,
  filter,
  toggle,
  select,
  open: openNode,
  context
})
provide('tree', tree)

defineExpose({ refresh, collapseAll, focusFilter, selectedId })
</script>

<template>
  <section class="navigator">
    <header class="nav-head">
      <span>数据库导航</span>
      <button class="icon-btn" type="button" title="隐藏导航" @click="$emit('collapse')">
        <Icon name="close" />
      </button>
    </header>
    <div class="nav-filter">
      <input
        ref="filterInput"
        v-model="filter"
        type="search"
        placeholder="过滤连接名、地址或对象名"
        aria-label="过滤连接名、地址或对象名"
      />
      <button class="icon-btn" type="button" title="过滤" @click="focusFilter">
        <Icon name="search" />
      </button>
    </div>
    <div class="nav-tools">
      <button class="icon-btn" type="button" title="全部折叠" @click="collapseAll">
        <Icon name="chevron" class="turn" />
      </button>
      <button class="icon-btn" type="button" title="刷新" @click="refresh()">
        <Icon name="refresh" />
      </button>
      <button class="icon-btn" type="button" title="新建连接" @click="$emit('create-connection')">
        <Icon name="add" />
      </button>
    </div>
    <div ref="treeBox" class="nav-tree" role="tree">
      <p v-if="!roots.length" class="nav-empty">还没有连接。可以新建一个 SQLite 文件或网络库。</p>
      <p v-else-if="filterMiss" class="nav-empty">没有匹配的连接。</p>
      <TreeNode v-for="node in roots" :key="node.id" :node="node" :depth="0" />
    </div>
  </section>
</template>
