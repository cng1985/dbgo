<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api, listConnections } from '../api'
import { filterTables, pickSchema } from '../catalogView.js'

const connections = ref([])
const connectionID = ref('')
const schemas = ref([])
const schema = ref('')
const tables = ref([])
const keyword = ref('')
const table = ref('')
const columns = ref([])
const indexes = ref([])
const error = ref('')
const props = defineProps({
  preferredConnection: { type: String, default: '' },
  preferredSchema: { type: String, default: '' },
  focusSeq: { type: Number, default: 0 },
  revision: { type: Number, default: 0 },
  structureRefresh: { type: Object, default: null }
})
const emit = defineEmits(['schema'])
const selected = computed(() => connections.value.find((item) => item.id === connectionID.value))
const visibleTables = computed(() => filterTables(tables.value, keyword.value))
let structureHold = 0
let appliedFocus = 0
let connectionsReady = false
let mounted = false
let schemaSerial = 0
let tableSerial = 0
let detailSerial = 0

function sameID(left, right) {
  return String(left || '') === String(right || '')
}

async function loadConnections() {
  connections.value = await listConnections()
  connectionsReady = true
  if (!connectionID.value && connections.value.length) {
    connectionID.value = connections.value[0].id
  }
}

function requestedSchema() {
  if (props.focusSeq && appliedFocus !== props.focusSeq) {
    return props.preferredSchema || ''
  }
  return schema.value || props.preferredSchema || ''
}

async function loadSchemas() {
  const requested = requestedSchema()
  const connection = connectionID.value
  const serial = ++schemaSerial
  schemas.value = []
  schema.value = ''
  tables.value = []
  keyword.value = ''
  if (!connection) {
    appliedFocus = props.focusSeq || appliedFocus
    return
  }
  const result = (await api('/api/catalog/schemas', { connection_id: connection })).data
  if (serial !== schemaSerial || !sameID(connectionID.value, connection)) {
    return
  }
  schemas.value = result.schemas || []
  schema.value = pickSchema(selected.value?.driver, schemas.value, requested, selected.value?.database_name || '')
  appliedFocus = props.focusSeq || appliedFocus
}

async function loadTables() {
  const connection = connectionID.value
  const currentSchema = schema.value
  const serial = ++tableSerial
  tables.value = []
  columns.value = []
  indexes.value = []
  table.value = ''
  if (!connection) {
    return
  }
  const result = (await api('/api/catalog/tables', { connection_id: connection, schema: currentSchema })).data
  if (serial !== tableSerial || !sameID(connectionID.value, connection) || schema.value !== currentSchema) {
    return
  }
  tables.value = result.tables || []
  table.value = tables.value[0]?.name || ''
}

async function loadDetail(options = {}) {
  const connection = connectionID.value
  const currentSchema = schema.value
  const currentTable = table.value
  const serial = ++detailSerial
  if (!connection || !currentTable) {
    columns.value = []
    indexes.value = []
    return
  }
  if (!options.keep) {
    columns.value = []
    indexes.value = []
  }
  const body = { connection_id: connection, schema: currentSchema, table: currentTable }
  const columnResult = (await api('/api/catalog/columns', body)).data
  const indexResult = (await api('/api/catalog/indexes', body)).data
  if (serial !== detailSerial || !sameID(connectionID.value, connection) || schema.value !== currentSchema || table.value !== currentTable) {
    return
  }
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
  const connection = connectionID.value
  const serial = ++schemaSerial
  structureHold += 1
  try {
    const schemaResult = (await api('/api/catalog/schemas', { connection_id: connection })).data
    if (serial !== schemaSerial || !sameID(connectionID.value, connection)) {
      return
    }
    schemas.value = schemaResult.schemas || []
    const schemaNames = new Set(schemas.value.map((item) => item.name))
    schema.value = schemaNames.has(keepSchema) || keepSchema === ''
      ? keepSchema
      : pickSchema(selected.value?.driver, schemas.value, '', selected.value?.database_name || '')
    const tableResult = (await api('/api/catalog/tables', {
      connection_id: connection,
      schema: schema.value
    })).data
    if (serial !== schemaSerial || !sameID(connectionID.value, connection)) {
      return
    }
    tableSerial += 1
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
  if (!connectionsReady || !mounted) {
    return
  }
  error.value = ''
  try {
    await loadSchemas()
  } catch (err) {
    error.value = err.message
  }
})
watch(schema, async (value) => {
  emit('schema', value || '')
  if (structureHold) {
    return
  }
  keyword.value = ''
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

watch(() => props.focusSeq, () => {
  if (!props.focusSeq) {
    return
  }
  keyword.value = ''
  if (props.preferredConnection && props.preferredConnection !== connectionID.value) {
    connectionID.value = props.preferredConnection
    return
  }
  if (props.preferredSchema && schemas.value.some((item) => item.name === props.preferredSchema)) {
    schema.value = props.preferredSchema
    appliedFocus = props.focusSeq
    return
  }
  loadSchemas().catch((err) => {
    error.value = err.message
  })
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
    mounted = true
    if (connectionID.value) {
      await loadSchemas()
    }
  } catch (err) {
    error.value = err.message
  }
})
</script>

<template>
  <div class="page catalog-page">
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
      <section class="card catalog-tables">
        <div class="catalog-table-head">
          <h2>表</h2>
          <span v-if="tables.length" class="muted">{{ visibleTables.length }}/{{ tables.length }}</span>
        </div>
        <input v-model="keyword" class="catalog-filter" type="search" placeholder="搜索表" aria-label="搜索表" />
        <div class="catalog-table-list">
          <div v-if="!tables.length" class="empty">这个连接下还没有表。</div>
          <div v-else-if="!visibleTables.length" class="empty">没有匹配的表。</div>
          <button v-for="item in visibleTables" :key="item.schema + item.name" class="history" type="button" :class="{ picked: item.name === table }" @click="table = item.name">
            <strong>{{ item.name }}</strong>
            <small class="muted">{{ item.type }}</small>
          </button>
        </div>
      </section>
      <section class="stack catalog-detail">
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
