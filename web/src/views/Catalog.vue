<script setup>
import { onMounted, ref, watch } from 'vue'
import { api } from '../api'

const connections = ref([])
const connectionID = ref('')
const schemas = ref([])
const schema = ref('')
const tables = ref([])
const table = ref('')
const columns = ref([])
const indexes = ref([])
const error = ref('')
const props = defineProps({
  preferredConnection: { type: String, default: '' },
  revision: { type: Number, default: 0 },
  structureRefresh: { type: Object, default: null }
})
let structureHold = 0

async function loadConnections() {
  const page = await api('/api/connection/search', { page: 1, page_size: 100, sort: 'name', direction: 'asc' })
  connections.value = page.data
  if (!connectionID.value && page.data.length) {
    connectionID.value = page.data[0].id
  }
}

async function loadSchemas() {
  schemas.value = []
  schema.value = ''
  tables.value = []
  if (!connectionID.value) {
    return
  }
  const result = (await api('/api/catalog/schemas', { connection_id: connectionID.value })).data
  schemas.value = result.schemas || []
  schema.value = schemas.value[0]?.name || ''
}

async function loadTables() {
  tables.value = []
  columns.value = []
  indexes.value = []
  table.value = ''
  if (!connectionID.value) {
    return
  }
  const result = (await api('/api/catalog/tables', { connection_id: connectionID.value, schema: schema.value })).data
  tables.value = result.tables || []
  table.value = tables.value[0]?.name || ''
}

async function loadDetail(options = {}) {
  if (!connectionID.value || !table.value) {
    columns.value = []
    indexes.value = []
    return
  }
  if (!options.keep) {
    columns.value = []
    indexes.value = []
  }
  const body = { connection_id: connectionID.value, schema: schema.value, table: table.value }
  const columnResult = (await api('/api/catalog/columns', body)).data
  const indexResult = (await api('/api/catalog/indexes', body)).data
  columns.value = columnResult.columns || []
  indexes.value = indexResult.indexes || []
}

async function refreshStructure(info) {
  if (!info?.name || !connectionID.value || String(connectionID.value) !== String(info.connectionId || '')) {
    return
  }
  const currentSchema = schema.value || ''
  const previousSchema = info.previousSchema || ''
  const nextSchema = info.schema || ''
  const viewingEdited = !!info.previousName && currentSchema === previousSchema && table.value === info.previousName
  const schemaInView = currentSchema === previousSchema || currentSchema === nextSchema
  if (!viewingEdited && !schemaInView) {
    return
  }
  const keepSchema = viewingEdited ? nextSchema : currentSchema
  const keepTable = viewingEdited ? info.name : table.value
  structureHold += 1
  try {
    const schemaResult = (await api('/api/catalog/schemas', { connection_id: connectionID.value })).data
    schemas.value = schemaResult.schemas || []
    const schemaNames = new Set(schemas.value.map((item) => item.name))
    schema.value = schemaNames.has(keepSchema) || keepSchema === '' ? keepSchema : (schemas.value[0]?.name || '')
    const tableResult = (await api('/api/catalog/tables', {
      connection_id: connectionID.value,
      schema: schema.value
    })).data
    tables.value = tableResult.tables || []
    if (tables.value.some((item) => item.name === keepTable)) {
      table.value = keepTable
    } else if (!tables.value.some((item) => item.name === table.value)) {
      table.value = tables.value[0]?.name || ''
    }
    await loadDetail({ keep: true })
  } catch (err) {
    error.value = err.message
  } finally {
    structureHold -= 1
  }
}

watch(connectionID, async () => {
  error.value = ''
  try {
    await loadSchemas()
  } catch (err) {
    error.value = err.message
  }
})
watch(schema, async () => {
  if (structureHold) {
    return
  }
  try {
    await loadTables()
  } catch (err) {
    error.value = err.message
  }
})
watch(table, async () => {
  if (structureHold) {
    return
  }
  try {
    await loadDetail()
  } catch (err) {
    error.value = err.message
  }
})

watch(() => props.preferredConnection, (id) => {
  if (id) {
    connectionID.value = id
  }
})

watch(() => props.revision, (value, previous) => {
  if (previous === undefined || value === previous) {
    return
  }
  loadConnections()
})

watch(() => props.structureRefresh?.seq, (value, previous) => {
  if (!value || value === previous) {
    return
  }
  refreshStructure(props.structureRefresh)
})

onMounted(async () => {
  try {
    if (props.preferredConnection) {
      connectionID.value = props.preferredConnection
    }
    await loadConnections()
  } catch (err) {
    error.value = err.message
  }
})
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <div class="toolbar">
      <select v-model="connectionID">
        <option value="" disabled>选择连接</option>
        <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
      <select v-model="schema">
        <option value="">默认模式</option>
        <option v-for="item in schemas" :key="item.name" :value="item.name">{{ item.name }}</option>
      </select>
    </div>
    <div class="split">
      <section class="card">
        <h2>表</h2>
        <div v-if="!tables.length" class="empty">这个连接下还没有表。</div>
        <button v-for="item in tables" :key="item.schema + item.name" class="history" type="button" :class="{ picked: item.name === table }" @click="table = item.name">
          <strong>{{ item.name }}</strong>
          <small class="muted">{{ item.type }}</small>
        </button>
      </section>
      <section class="stack">
        <div class="card">
          <h2>字段</h2>
          <div class="table-wrap">
            <table>
              <thead><tr><th>名称</th><th>类型</th><th>可空</th><th>主键</th><th>默认值</th><th>注释</th></tr></thead>
              <tbody>
                <tr v-if="!columns.length"><td colspan="6" class="empty">选择一张表。</td></tr>
                <tr v-for="item in columns" :key="item.name">
                  <td>{{ item.name }}</td>
                  <td>{{ item.type }}</td>
                  <td>{{ item.nullable ? '是' : '否' }}</td>
                  <td>{{ item.primary ? '是' : '' }}</td>
                  <td>{{ item.default || '' }}</td>
                  <td>{{ item.comment || '' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div class="card">
          <h2>索引</h2>
          <div class="table-wrap">
            <table>
              <thead><tr><th>名称</th><th>唯一</th><th>列</th></tr></thead>
              <tbody>
                <tr v-if="!indexes.length"><td colspan="3" class="empty">没有索引。</td></tr>
                <tr v-for="item in indexes" :key="item.name">
                  <td>{{ item.name }}</td>
                  <td>{{ item.unique ? '是' : '否' }}</td>
                  <td>{{ item.columns }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>
