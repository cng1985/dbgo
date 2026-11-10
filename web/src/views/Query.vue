<script setup>
import { onMounted, ref } from 'vue'
import { api, formatTime, statusText } from '../api'

const connections = ref([])
const connectionID = ref('')
const sql = ref('select 1 as ok')
const readOnly = ref(true)
const maxRows = ref(100)
const result = ref(null)
const history = ref([])
const error = ref('')
const pending = ref(false)

async function loadConnections() {
  const page = await api('/api/connection/search', { page: 1, page_size: 100, sort: 'name', direction: 'asc' })
  connections.value = page.data
  if (!connectionID.value && page.data.length) {
    connectionID.value = page.data[0].id
  }
}

async function loadHistory() {
  const page = await api('/api/query/search', {
    connection_id: connectionID.value || '0',
    page: 1,
    page_size: 20
  })
  history.value = page.data
}

async function run() {
  error.value = ''
  result.value = null
  pending.value = true
  try {
    result.value = (await api('/api/query/execute', {
      connection_id: connectionID.value,
      sql: sql.value,
      read_only: readOnly.value,
      max_rows: Number(maxRows.value || 100)
    })).data
    await loadHistory()
  } catch (err) {
    error.value = err.message
    await loadHistory()
  } finally {
    pending.value = false
  }
}

async function openHistory(item) {
  try {
    const one = (await api('/api/query/one', { id: item.id })).data
    sql.value = one.sql
    readOnly.value = one.read_only
    connectionID.value = one.connection_id
    error.value = one.error_message || ''
  } catch (err) {
    error.value = err.message
  }
}

onMounted(async () => {
  try {
    await loadConnections()
    await loadHistory()
  } catch (err) {
    error.value = err.message
  }
})
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <div class="split split-wide">
      <section class="card stack">
        <div class="toolbar">
          <select v-model="connectionID">
            <option value="" disabled>选择连接</option>
            <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
          <label class="check"><input v-model="readOnly" type="checkbox" /> 只读</label>
          <label class="field"><span>最大行数</span><input v-model.number="maxRows" type="number" min="1" max="1000" /></label>
          <button class="btn btn-primary" type="button" :disabled="pending || !connectionID" @click="run">执行</button>
        </div>
        <textarea v-model="sql" class="sql-box" spellcheck="false" />
        <div v-if="result" class="stack">
          <p class="muted">耗时 {{ result.duration_ms }} ms，影响或返回 {{ result.row_count }} 行</p>
          <article v-for="(statement, index) in result.statements" :key="index" class="stack">
            <div class="row">
              <strong>{{ statement.command || '语句 ' + (index + 1) }}</strong>
              <span v-if="statement.truncated" class="tag">结果已截断</span>
              <span class="muted">影响 {{ statement.rows_affected }} 行</span>
            </div>
            <div v-if="statement.columns?.length" class="table-wrap result">
              <table>
                <thead>
                  <tr><th v-for="column in statement.columns" :key="column.name">{{ column.name }}</th></tr>
                </thead>
                <tbody>
                  <tr v-for="(row, rowIndex) in statement.rows" :key="rowIndex">
                    <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell === null ? 'NULL' : cell }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </article>
        </div>
      </section>
      <aside class="card">
        <h2>历史</h2>
        <div v-if="!history.length" class="empty">还没有查询。</div>
        <button v-for="item in history" :key="item.id" class="history" type="button" @click="openHistory(item)">
          <span class="tag" :class="item.status === 'failed' ? 'bad' : 'good'">{{ statusText[item.status] || item.status }}</span>
          <span>{{ item.connection_name }}</span>
          <small class="muted">{{ formatTime(item.created_at) }} · {{ item.duration_ms }} ms</small>
        </button>
      </aside>
    </div>
  </div>
</template>
