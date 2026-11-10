<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api, driverLabel, formatTime, statusText } from '../api'
import SqlEditor from './SqlEditor.vue'

const props = defineProps({
  tab: { type: Object, required: true },
  connections: { type: Array, default: () => [] },
  showResults: { type: Boolean, default: true }
})
const emit = defineEmits(['ran', 'close-results'])

const schemas = ref([])
const hintTables = ref([])
const hintPending = ref(false)
const hintError = ref('')
const truncated = ref(false)
const result = ref(null)
const history = ref([])
const error = ref('')
const pending = ref(false)
const panel = ref('result')
const resultsHeight = ref(240)
let hintSeq = 0

const driver = computed(() => props.connections.find((item) => item.id === props.tab.connectionId)?.driver || '')
const schemaNames = computed(() => schemas.value.map((item) => item.name))
const columnCount = computed(() => hintTables.value.reduce((sum, table) => sum + (table.columns?.length || 0), 0))
const hintText = computed(() => {
  if (hintPending.value && !hintTables.value.length) {
    return '正在读取表和字段'
  }
  if (!props.tab.connectionId) {
    return '关键字可用，选择连接后提示表和字段'
  }
  if (!hintTables.value.length && !hintError.value) {
    return '关键字可用，当前模式还没有表'
  }
  let body = `关键字、${hintTables.value.length} 张表、${columnCount.value} 个字段`
  if (truncated.value) {
    body += '（已截断）'
  }
  return body
})
const summary = computed(() => {
  if (pending.value) {
    return '正在执行'
  }
  if (!result.value) {
    return ''
  }
  return `耗时 ${result.value.duration_ms} ms，影响或返回 ${result.value.row_count} 行`
})

async function loadHistory() {
  const page = await api('/api/query/search', {
    connection_id: props.tab.connectionId || '0',
    page: 1,
    page_size: 20
  })
  history.value = page.data || []
}

async function loadHints(preferred) {
  const seq = ++hintSeq
  const current = props.tab.connectionId
  hintError.value = ''
  if (!current) {
    schemas.value = []
    hintTables.value = []
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
    if (seq !== hintSeq || current !== props.tab.connectionId) {
      return
    }
    schemas.value = resultBody.schemas || []
    hintTables.value = resultBody.tables || []
    truncated.value = !!resultBody.truncated
    const nextSchema = resultBody.schema || ''
    if (props.tab.schema !== nextSchema) {
      props.tab.schema = nextSchema
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

function pickConnection(id) {
  props.tab.schema = ''
  props.tab.connectionId = id
}

async function run() {
  error.value = ''
  result.value = null
  panel.value = 'result'
  pending.value = true
  props.tab.autorun = 0
  try {
    result.value = (await api('/api/query/execute', {
      connection_id: props.tab.connectionId,
      sql: props.tab.sql,
      read_only: props.tab.readOnly,
      max_rows: Number(props.tab.maxRows || 100)
    })).data
    emit('ran', `${props.tab.title} 完成，${result.value.duration_ms} ms，${result.value.row_count} 行`)
    await loadHistory()
    loadHints(props.tab.schema)
  } catch (err) {
    error.value = err.message
    emit('ran', err.message)
    await loadHistory()
  } finally {
    pending.value = false
  }
}

async function openHistory(item) {
  try {
    const one = (await api('/api/query/one', { id: item.id })).data
    props.tab.sql = one.sql
    props.tab.readOnly = one.read_only
    props.tab.connectionId = one.connection_id
    error.value = one.error_message || ''
    panel.value = 'result'
  } catch (err) {
    error.value = err.message
  }
}

function startDrag(event) {
  const startY = event.clientY
  const startH = resultsHeight.value
  function move(next) {
    resultsHeight.value = Math.min(520, Math.max(96, startH + startY - next.clientY))
  }
  function up() {
    window.removeEventListener('mousemove', move)
    window.removeEventListener('mouseup', up)
  }
  window.addEventListener('mousemove', move)
  window.addEventListener('mouseup', up)
}

watch(() => props.tab.connectionId, () => {
  loadHints(props.tab.schema || '')
  loadHistory()
})

watch(() => props.tab.schema, (value) => {
  if (!props.tab.connectionId || !value) {
    return
  }
  const loaded = hintTables.value.find((table) => table.schema)?.schema
  if (loaded && loaded !== value) {
    loadHints(value)
  }
})

onMounted(() => {
  loadHints(props.tab.schema || '')
  loadHistory()
  if (props.tab.autorun) {
    run()
  }
})

defineExpose({ run })
</script>

<template>
  <section class="script">
    <div class="script-bar">
      <button class="btn btn-primary script-run" type="button" :disabled="pending || !tab.connectionId" title="Ctrl 或 ⌘ + Enter" @click="run">
        执行
      </button>
      <select :value="tab.connectionId" aria-label="连接" @change="pickConnection($event.target.value)">
        <option value="" disabled>选择连接</option>
        <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
      <select v-model="tab.schema" :disabled="!schemas.length" aria-label="默认模式">
        <option v-if="!schemas.length" value="">默认模式</option>
        <option v-for="item in schemas" :key="item.name" :value="item.name">{{ item.name }}</option>
      </select>
      <label class="check"><input v-model="tab.readOnly" type="checkbox" /> 只读</label>
      <label class="check">最大行数 <input v-model.number="tab.maxRows" class="rows" type="number" min="1" max="1000" /></label>
      <button class="btn" type="button" :disabled="!tab.connectionId || hintPending" @click="loadHints(tab.schema)">刷新结构</button>
      <span class="script-hint">{{ driver ? driverLabel(driver) + ' · ' : '' }}{{ hintText }}</span>
    </div>
    <p v-if="hintError" class="notice script-alert">结构提示不可用：{{ hintError }}。关键字提示仍然可用。</p>
    <div class="script-editor">
      <SqlEditor v-model="tab.sql" fill :driver="driver" :schema="tab.schema" :schemas="schemaNames" :tables="hintTables" @run="run" />
    </div>
    <template v-if="showResults">
      <div class="splitter-h" @mousedown="startDrag"></div>
      <section class="script-results" :style="{ height: resultsHeight + 'px' }">
        <div class="result-tabs">
          <button type="button" :class="{ on: panel === 'result' }" @click="panel = 'result'">结果</button>
          <button type="button" :class="{ on: panel === 'history' }" @click="panel = 'history'">历史</button>
          <span class="result-summary">{{ summary }}</span>
          <button class="icon-btn" type="button" title="隐藏结果" @click="emit('close-results')">×</button>
        </div>
        <div class="result-body">
          <template v-if="panel === 'history'">
            <div v-if="!history.length" class="empty">还没有查询。</div>
            <button v-for="item in history" :key="item.id" class="history" type="button" @click="openHistory(item)">
              <span class="tag" :class="item.status === 'failed' ? 'bad' : 'good'">{{ statusText[item.status] || item.status }}</span>
              <span>{{ item.connection_name }}</span>
              <small class="muted">{{ formatTime(item.created_at) }} · {{ item.duration_ms }} ms</small>
            </button>
          </template>
          <template v-else>
            <p v-if="error" class="notice">{{ error }}</p>
            <div v-else-if="!result" class="empty">执行 SQL 后在这里显示结果。Ctrl 或 ⌘ + Enter 执行，Ctrl + Space 提示。</div>
            <article v-for="(statement, index) in result?.statements || []" :key="index" class="statement">
              <div class="row">
                <strong>{{ statement.command || '语句 ' + (index + 1) }}</strong>
                <span v-if="statement.truncated" class="tag">结果已截断</span>
                <span class="muted">影响 {{ statement.rows_affected }} 行</span>
              </div>
              <div v-if="statement.columns?.length" class="table-wrap result-grid">
                <table>
                  <thead>
                    <tr>
                      <th class="rownum">#</th>
                      <th v-for="column in statement.columns" :key="column.name">{{ column.name }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(row, rowIndex) in statement.rows" :key="rowIndex">
                      <td class="rownum">{{ rowIndex + 1 }}</td>
                      <td v-for="(cell, cellIndex) in row" :key="cellIndex" :class="{ null: cell === null }">{{ cell === null ? 'NULL' : cell }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>
          </template>
        </div>
      </section>
    </template>
  </section>
</template>
