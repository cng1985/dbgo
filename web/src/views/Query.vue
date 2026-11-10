<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api, driverLabel, formatTime, statusText } from '../api'
import SqlEditor from '../components/SqlEditor.vue'

const connections = ref([])
const connectionID = ref('')
const schemas = ref([])
const schema = ref('')
const hintTables = ref([])
const hintPending = ref(false)
const hintError = ref('')
const truncated = ref(false)
const sql = ref('select 1 as ok')
const readOnly = ref(true)
const maxRows = ref(100)
const result = ref(null)
const history = ref([])
const error = ref('')
const pending = ref(false)
let hintSeq = 0

const driver = computed(() => connections.value.find((item) => item.id === connectionID.value)?.driver || '')
const schemaNames = computed(() => schemas.value.map((item) => item.name))
const columnCount = computed(() => hintTables.value.reduce((sum, table) => sum + (table.columns?.length || 0), 0))
const hintText = computed(() => {
  const bits = []
  if (driver.value) {
    bits.push(driverLabel(driver.value))
  }
  if (schema.value) {
    bits.push(schema.value)
  }
  if (hintPending.value && !hintTables.value.length) {
    bits.push('正在读取表和字段')
    return bits.join(' · ')
  }
  if (!connectionID.value) {
    bits.push('关键字可用，选择连接后提示表和字段')
    return bits.join(' · ')
  }
  let body = `关键字、${hintTables.value.length} 张表、${columnCount.value} 个字段`
  if (!hintTables.value.length && !hintError.value) {
    body = '关键字可用，当前模式还没有表'
  }
  if (truncated.value) {
    body += '（已截断）'
  }
  bits.push(body)
  return bits.join(' · ')
})

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

async function loadHints(preferred) {
  const seq = ++hintSeq
  const current = connectionID.value
  hintError.value = ''
  if (!current) {
    schemas.value = []
    hintTables.value = []
    schema.value = ''
    truncated.value = false
    hintPending.value = false
    return
  }
  hintPending.value = true
  try {
    const resultBody = (await api('/api/catalog/suggest', {
      connection_id: current,
      schema: preferred || ''
    })).data
    if (seq !== hintSeq || current !== connectionID.value) {
      return
    }
    schemas.value = resultBody.schemas || []
    hintTables.value = resultBody.tables || []
    truncated.value = !!resultBody.truncated
    const nextSchema = resultBody.schema || ''
    if (schema.value !== nextSchema) {
      schema.value = nextSchema
    }
  } catch (err) {
    if (seq !== hintSeq) {
      return
    }
    hintError.value = err.message
  } finally {
    if (seq === hintSeq) {
      hintPending.value = false
    }
  }
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
    loadHints(schema.value)
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

watch(connectionID, () => {
  hintTables.value = []
  schemas.value = []
  schema.value = ''
  truncated.value = false
  hintError.value = ''
  loadHints('')
})

watch(schema, (value) => {
  if (!connectionID.value || !value) {
    return
  }
  const loaded = hintTables.value.find((table) => table.schema)?.schema
  if (loaded && loaded !== value) {
    loadHints(value)
  }
})

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
    <p v-if="hintError" class="notice">结构提示不可用：{{ hintError }}。关键字提示仍然可用。</p>
    <div class="split split-wide">
      <section class="card stack">
        <div class="toolbar">
          <div class="row">
            <select v-model="connectionID">
              <option value="" disabled>选择连接</option>
              <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
            <select v-model="schema" :disabled="!schemas.length" aria-label="默认模式">
              <option v-if="!schemas.length" value="">默认模式</option>
              <option v-for="item in schemas" :key="item.name" :value="item.name">{{ item.name }}</option>
            </select>
            <button class="btn" type="button" :disabled="!connectionID || hintPending" @click="loadHints(schema)">刷新结构</button>
          </div>
          <div class="row">
            <label class="check"><input v-model="readOnly" type="checkbox" /> 只读</label>
            <label class="field field-inline"><span>最大行数</span><input v-model.number="maxRows" type="number" min="1" max="1000" /></label>
            <button class="btn btn-primary" type="button" :disabled="pending || !connectionID" title="Ctrl 或 ⌘ + Enter" @click="run">执行</button>
          </div>
        </div>
        <SqlEditor v-model="sql" :driver="driver" :schema="schema" :schemas="schemaNames" :tables="hintTables" @run="run" />
        <p class="muted editor-note">{{ hintText }}。输入时提示，Ctrl + Space 打开提示，Ctrl/⌘ + Enter 执行。</p>
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
