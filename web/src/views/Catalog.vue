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

async function loadDetail() {
  columns.value = []
  indexes.value = []
  if (!connectionID.value || !table.value) {
    return
  }
  const body = { connection_id: connectionID.value, schema: schema.value, table: table.value }
  const columnResult = (await api('/api/catalog/columns', body)).data
  const indexResult = (await api('/api/catalog/indexes', body)).data
  columns.value = columnResult.columns || []
  indexes.value = indexResult.indexes || []
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
  try {
    await loadTables()
  } catch (err) {
    error.value = err.message
  }
})
watch(table, async () => {
  try {
    await loadDetail()
  } catch (err) {
    error.value = err.message
  }
})

onMounted(async () => {
  try {
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
              <thead><tr><th>名称</th><th>类型</th><th>可空</th><th>主键</th><th>默认值</th></tr></thead>
              <tbody>
                <tr v-if="!columns.length"><td colspan="5" class="empty">选择一张表。</td></tr>
                <tr v-for="item in columns" :key="item.name">
                  <td>{{ item.name }}</td>
                  <td>{{ item.type }}</td>
                  <td>{{ item.nullable ? '是' : '否' }}</td>
                  <td>{{ item.primary ? '是' : '' }}</td>
                  <td>{{ item.default || '' }}</td>
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
