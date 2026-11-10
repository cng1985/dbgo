<script setup>
import { provide, reactive, ref, watch } from 'vue'
import { api, connectionTarget } from '../api'
import Icon from './Icon.vue'
import TreeNode from './TreeNode.vue'

const props = defineProps({
  connections: { type: Array, default: () => [] }
})
const emit = defineEmits(['select', 'context', 'open-data', 'open-design', 'reload', 'collapse', 'create-connection'])

const filter = ref('')
const filterInput = ref(null)
const selectedId = ref('')
const roots = ref([])

function folder(id, label, children, open = false) {
  return {
    id,
    kind: 'folder',
    label,
    meta: String(children.length),
    open,
    loaded: true,
    children
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
        meta: column.type || '',
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
        leaf: true
      }))
      return [
        folder(`cols-${connection.id}-${schema}-${table.name}`, '列', cols),
        folder(`idxs-${connection.id}-${schema}-${table.name}`, '索引', indexNodes)
      ]
    }
  }
}

function routineNode(connection, schema, item) {
  return {
    id: `r-${connection.id}-${schema}-${item.kind}-${item.name}`,
    kind: item.kind,
    label: item.name,
    meta: item.detail || '',
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
      const children = [
        folder(`tables-${connection.id}-${name}`, '表', base.map((item) => tableNode(connection, name, item))),
        folder(`views-${connection.id}-${name}`, '视图', views.map((item) => tableNode(connection, name, item)))
      ]
      if (routineSupported) {
        children.push(
          folder(`procs-${connection.id}-${name}`, '存储过程', routines.filter((item) => item.kind === 'procedure').map((item) => routineNode(connection, name, item))),
          folder(`funcs-${connection.id}-${name}`, '函数', routines.filter((item) => item.kind === 'function').map((item) => routineNode(connection, name, item)))
        )
      }
      return children
    }
  }
}

function connectionNode(row, previous) {
  if (previous) {
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
  if (selectedId.value && ![...walk(roots.value)].some((node) => node.id === selectedId.value)) {
    selectedId.value = ''
  }
  if (!seen.size) {
    selectedId.value = ''
  }
}, { immediate: true })

function* walk(nodes) {
  for (const node of nodes || []) {
    yield node
    yield* walk(node.children)
  }
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
  } catch (err) {
    node.error = err.message
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

async function refresh(node) {
  const target = node && node.load ? node : null
  if (!target) {
    emit('reload')
    const opened = roots.value.filter((item) => item.open)
    for (const item of opened) {
      item.loaded = false
      item.children = []
      await ensure(item)
    }
    return
  }
  target.loaded = false
  target.children = []
  target.open = true
  await ensure(target)
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
        placeholder="按名称过滤对象（表、过程、用户等）"
        aria-label="按名称过滤对象"
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
    <div class="nav-tree" role="tree">
      <p v-if="!roots.length" class="nav-empty">还没有连接。可以新建一个 SQLite 文件或网络库。</p>
      <TreeNode v-for="node in roots" :key="node.id" :node="node" :depth="0" />
    </div>
  </section>
</template>
