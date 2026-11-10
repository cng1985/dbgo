<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, driverLabel, formatTime, statusText } from '../api'
import { commentSQL, copyText, explainSQL, filterSQL, formatSQL, insertSQL } from '../sqlActions.js'
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
const editorRef = ref(null)
const menu = ref(null)
const advice = ref(null)
const optimizing = ref(false)
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

async function run(sqlText) {
  error.value = ''
  result.value = null
  panel.value = 'result'
  pending.value = true
  props.tab.autorun = 0
  const sql = typeof sqlText === 'string' ? sqlText : props.tab.sql
  try {
    result.value = (await api('/api/query/execute', {
      connection_id: props.tab.connectionId,
      sql,
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

function chosenText(payload) {
  if (payload?.text && payload.text.trim()) {
    return payload.text
  }
  return props.tab.sql
}

function replaceChosen(payload, value) {
  if (payload?.text && payload.text.trim() && editorRef.value) {
    editorRef.value.replaceRange(payload.start, payload.end, value)
    return
  }
  props.tab.sql = value
}

function format(payload) {
  menu.value = null
  replaceChosen(payload, formatSQL(chosenText(payload)))
}

function explain(payload) {
  menu.value = null
  if (driver.value === 'sqlserver') {
    error.value = 'SQL Server 没有 EXPLAIN。可以用 AI 优化查看改写建议。'
    return
  }
  props.tab.sql = explainSQL(driver.value, chosenText(payload))
  props.tab.readOnly = true
  run()
}

async function optimize(payload) {
  menu.value = null
  error.value = ''
  optimizing.value = true
  panel.value = 'ai'
  try {
    advice.value = (await api('/api/llm/optimize', {
      connection_id: props.tab.connectionId,
      schema: props.tab.schema,
      sql: chosenText(payload)
    })).data
  } catch (err) {
    error.value = err.message
  } finally {
    optimizing.value = false
  }
}

function applyAdvice() {
  if (advice.value?.sql) {
    props.tab.sql = advice.value.sql
    props.tab.readOnly = false
  }
}

function comment(payload) {
  menu.value = null
  replaceChosen(payload, commentSQL(chosenText(payload)))
}

function openEditorMenu(payload) {
  const selected = payload.text && payload.text.trim()
  menu.value = {
    x: payload.x,
    y: payload.y,
    items: [
      { label: selected ? '执行选中' : '执行', run: () => run(selected ? payload.text : undefined) },
      { label: '格式化', run: () => format(payload) },
      { label: '解释', run: () => explain(payload) },
      { label: 'AI 优化', disabled: !props.tab.connectionId, run: () => optimize(payload) },
      { label: '注释 / 取消注释', run: () => comment(payload) },
      { label: '复制', run: () => copyText(chosenText(payload)) }
    ]
  }
}

function openCellMenu(event, statement, row, cellIndex) {
  event.preventDefault()
  const column = statement.columns[cellIndex]
  const value = row[cellIndex]
  menu.value = {
    x: event.clientX,
    y: event.clientY,
    items: [
      { label: '复制单元格', run: () => copyText(value === null ? 'NULL' : String(value)) },
      { label: '复制行', run: () => copyText(row.map((cell) => (cell === null ? 'NULL' : cell)).join('\t')) },
      { label: '复制行为 JSON', run: () => copyText(JSON.stringify(Object.fromEntries(statement.columns.map((item, index) => [item.name, row[index]])), null, 2)) },
      { label: '复制为 INSERT', run: () => copyText(insertSQL(statement.columns, row)) },
      { label: '复制列名', run: () => copyText(column?.name || '') },
      { label: '用此值筛选', run: () => { props.tab.sql = filterSQL(driver.value, props.tab.sql, column?.name || 'col', value); props.tab.readOnly = true } }
    ]
  }
}

function runMenu(item) {
  const action = item.run
  menu.value = null
  action()
}

function closeMenu(event) {
  if (!menu.value) {
    return
  }
  if (event?.target?.closest?.('.ctx')) {
    return
  }
  menu.value = null
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
  window.addEventListener('pointerdown', closeMenu)
  if (props.tab.autorun) {
    run()
  }
})

onBeforeUnmount(() => window.removeEventListener('pointerdown', closeMenu))

defineExpose({ run, format, explain, optimize })
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
      <button class="btn" type="button" @click="format()">格式化</button>
      <button class="btn" type="button" :disabled="!tab.sql.trim()" @click="explain()">解释</button>
      <button class="btn" type="button" :disabled="!tab.connectionId || optimizing" @click="optimize()">AI 优化</button>
      <span class="script-hint">{{ driver ? driverLabel(driver) + ' · ' : '' }}{{ hintText }}</span>
    </div>
    <p v-if="hintError" class="notice script-alert">结构提示不可用：{{ hintError }}。关键字提示仍然可用。</p>
    <div class="script-editor">
      <SqlEditor ref="editorRef" v-model="tab.sql" fill :driver="driver" :schema="tab.schema" :schemas="schemaNames" :tables="hintTables" @run="run" @menu="openEditorMenu" />
    </div>
    <template v-if="showResults">
      <div class="splitter-h" @mousedown="startDrag"></div>
      <section class="script-results" :style="{ height: resultsHeight + 'px' }">
        <div class="result-tabs">
          <button type="button" :class="{ on: panel === 'result' }" @click="panel = 'result'">结果</button>
          <button type="button" :class="{ on: panel === 'history' }" @click="panel = 'history'">历史</button>
          <button type="button" :class="{ on: panel === 'ai' }" @click="panel = 'ai'">优化</button>
          <span class="result-summary">{{ summary }}</span>
          <button class="icon-btn" type="button" title="隐藏结果" @click="emit('close-results')">×</button>
        </div>
        <div class="result-body">
          <template v-if="panel === 'ai'">
            <div v-if="optimizing" class="empty">正在请模型阅读语句和表结构。</div>
            <div v-else-if="!advice" class="empty">在编辑器里右键，或点 AI 优化。需要先在大模型里配置 OpenAI 协议服务。</div>
            <div v-else class="stack ai-box">
              <p>{{ advice.summary || '模型没有给出摘要' }}</p>
              <p v-for="(note, index) in advice.notes || []" :key="index" class="muted">{{ note }}</p>
              <pre v-if="advice.sql" class="sql-preview">{{ advice.sql }}</pre>
              <button v-if="advice.sql" class="btn btn-primary" type="button" @click="applyAdvice">替换到编辑器</button>
            </div>
          </template>
          <template v-else-if="panel === 'history'">
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
                      <td v-for="(cell, cellIndex) in row" :key="cellIndex" :class="{ null: cell === null }" @contextmenu="openCellMenu($event, statement, row, cellIndex)">{{ cell === null ? 'NULL' : cell }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>
          </template>
        </div>
      </section>
    </template>
    <div v-if="menu" class="ctx" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" role="menu">
      <button v-for="(item, index) in menu.items" :key="index" type="button" :disabled="item.disabled" @click="runMenu(item)">{{ item.label }}</button>
    </div>
  </section>
</template>
