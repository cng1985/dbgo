<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api, driverLabel, formatTime, statusText } from '../api'
import { commentSQL, copyText, explainSQL, filterSQL, formatSQL, insertSQL } from '../sqlActions.js'
import { buildEditFields, fieldChanged, querySource, rowKeys, updateRowSQL } from '../rowEdit.js'
import { lineNumberAt, sqlErrorLine, translateErrorLine } from '../sqlError.js'
import { cellPreview, cycleSort, formatCopiedCells, sortResultRows } from '../resultGrid.js'
import SqlEditor from './SqlEditor.vue'
import { readSuggestEnabled, writeSuggestEnabled } from '../suggestPref.js'

const props = defineProps({
  tab: { type: Object, required: true },
  connections: { type: Array, default: () => [] },
  showResults: { type: Boolean, default: true }
})
const emit = defineEmits(['ran', 'close-results', 'manage-snippets'])

const schemas = ref([])
const hintTables = ref([])
const hintPending = ref(false)
const hintError = ref('')
const truncated = ref(false)
const result = ref(null)
const history = ref([])
const error = ref('')
const errorLine = computed(() => sqlErrorLine(error.value))
const sorts = ref({})
const pending = ref(false)
const panel = ref('result')
const resultsHeight = ref(240)
const editorRef = ref(null)
const menu = ref(null)
const menuEl = ref(null)
const advice = ref(null)
const optimizing = ref(false)
const snippets = ref([])
const snippetID = ref('')
const suggestOn = ref(readSuggestEnabled())
const editDialog = ref(null)
const editFields = ref([])
const editError = ref('')
const editWarn = ref('')
const editPending = ref(false)
const editTitle = ref('')
const editExact = ref(true)
const editContext = ref(null)
const editDirty = computed(() => editFields.value.some((field) => fieldChanged(field)))
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
const outputLines = computed(() => (result.value?.statements || []).flatMap((item) => item.messages || []))
const summary = computed(() => {
  if (pending.value) {
    return '正在执行'
  }
  if (!result.value) {
    return ''
  }
  const text = `耗时 ${result.value.duration_ms} ms，影响或返回 ${result.value.row_count} 行`
  if (!outputLines.value.length) {
    return text
  }
  return `${text}，${outputLines.value.length} 条输出`
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

function onSQL(value) {
  if (value !== props.tab.sql) {
    error.value = ''
  }
  props.tab.sql = value
}

function pickConnection(id) {
  props.tab.schema = ''
  props.tab.connectionId = id
}

function displayRows(statement, index) {
  const sort = sorts.value[index]
  return sortResultRows(statement.rows || [], sort?.column, sort?.direction)
}

function toggleSort(index, column) {
  const next = { ...sorts.value }
  const sorted = cycleSort(next[index], column)
  if (sorted) {
    next[index] = sorted
  } else {
    delete next[index]
  }
  sorts.value = next
}

function sortState(index, column) {
  const sort = sorts.value[index]
  if (!sort || sort.column !== column) {
    return 'none'
  }
  return sort.direction === 'asc' ? 'ascending' : 'descending'
}

function selectedResultCells(grid) {
  const selection = window.getSelection()
  if (!selection || selection.isCollapsed) {
    return []
  }
  const all = [...grid.querySelectorAll('td.result-cell')]
  const nodeCell = (node) => {
    const element = node?.nodeType === 1 ? node : node?.parentElement
    return element?.closest?.('td.result-cell') || null
  }
  const start = nodeCell(selection.anchorNode)
  const end = nodeCell(selection.focusNode)
  if (start && end) {
    const startAt = all.indexOf(start)
    const endAt = all.indexOf(end)
    if (startAt >= 0 && endAt >= 0) {
      const from = Math.min(startAt, endAt)
      const to = Math.max(startAt, endAt)
      return all.slice(from, to + 1)
    }
  }
  return all.filter((cell) => selection.containsNode(cell, true))
}

function onResultCopy(event, statementIndex) {
  const cells = selectedResultCells(event.currentTarget)
  if (!cells.some((cell) => cell.dataset.long === '1')) {
    return
  }
  const statement = result.value?.statements?.[statementIndex]
  const payload = formatCopiedCells(cells.map((cell) => ({
    row: cell.dataset.row,
    value: statement?.rows?.[Number(cell.dataset.row)]?.[Number(cell.dataset.col)]
  })))
  if (event.clipboardData) {
    event.preventDefault()
    event.clipboardData.setData('text/plain', payload)
    return
  }
  copyText(payload)
}

async function run(sqlText, originLine) {
  error.value = ''
  sorts.value = {}
  result.value = null
  panel.value = 'result'
  pending.value = true
  props.tab.autorun = 0
  const selected = typeof sqlText === 'string'
  const sql = selected ? sqlText : props.tab.sql
  const origin = selected && Number.isInteger(originLine) && originLine > 0 ? originLine : 1
  try {
    result.value = (await api('/api/query/execute', {
      connection_id: props.tab.connectionId,
      sql,
      schema: props.tab.schema || '',
      read_only: props.tab.readOnly,
      max_rows: Number(props.tab.maxRows || 100)
    })).data
    const statements = result.value?.statements || []
    const hasMessages = statements.some((item) => (item.messages || []).length)
    const hasGrid = statements.some((item) => item.columns?.length)
    panel.value = hasMessages && !hasGrid ? 'output' : 'result'
    emit('ran', `${props.tab.title} 完成，${result.value.duration_ms} ms，${result.value.row_count} 行`)
    await loadHistory()
    loadHints(props.tab.schema)
  } catch (err) {
    const message = translateErrorLine(err.message, origin)
    error.value = message
    emit('ran', message)
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
    props.tab.schema = one.schema || ''
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

async function loadSnippets() {
  try {
    const all = []
    let page = 1
    for (;;) {
      const result = await api('/api/snippet/search', { page, page_size: 200, sort: 'name', direction: 'asc' })
      const batch = result.data || []
      all.push(...batch)
      if (!batch.length || all.length >= (result.total || 0)) {
        break
      }
      page += 1
      if (page > 5) {
        break
      }
    }
    snippets.value = all
  } catch (err) {
    error.value = err.message
  }
}

async function insertSnippet() {
  if (!snippetID.value) {
    return
  }
  error.value = ''
  try {
    const one = (await api('/api/snippet/one', { id: snippetID.value })).data
    const body = one.body || ''
    if (!body) {
      return
    }
    if (editorRef.value?.insertAtCaret) {
      editorRef.value.insertAtCaret(body)
    } else {
      props.tab.sql = props.tab.sql ? `${props.tab.sql.replace(/\s*$/, '')}\n${body}` : body
    }
  } catch (err) {
    error.value = err.message
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
  showMenu({
    x: payload.x,
    y: payload.y,
    items: [
      { label: selected ? '执行选中' : '执行', run: () => run(selected ? payload.text : undefined, selected ? lineNumberAt(props.tab.sql, payload.start) : 1) },
      { label: '格式化', run: () => format(payload) },
      { label: '解释', run: () => explain(payload) },
      { label: 'AI 优化', disabled: !props.tab.connectionId, run: () => optimize(payload) },
      { label: '注释 / 取消注释', run: () => comment(payload) },
      { label: '复制', run: () => copyText(chosenText(payload)) },
      { label: (suggestOn.value ? '✓ ' : '') + '智能提示', run: () => writeSuggestEnabled(!suggestOn.value) }
    ]
  })
}

function showMenu(next) {
  menu.value = next
  nextTick(() => {
    const el = menuEl.value
    if (!el || !menu.value) {
      return
    }
    const rect = el.getBoundingClientRect()
    const x = rect.right > window.innerWidth ? Math.max(8, window.innerWidth - rect.width - 8) : menu.value.x
    const y = rect.bottom > window.innerHeight ? Math.max(8, window.innerHeight - rect.height - 8) : menu.value.y
    if (x !== menu.value.x || y !== menu.value.y) {
      menu.value = { ...menu.value, x, y }
    }
  })
}

function openRowMenu(event, statement, row) {
  event.preventDefault()
  const cellIndex = cellFromEvent(event)
  const column = cellIndex >= 0 ? statement.columns?.[cellIndex] : null
  const value = cellIndex >= 0 ? row[cellIndex] : null
  const items = []
  if (column) {
    items.push(
      { label: '复制单元格', run: () => copyText(value === null ? 'NULL' : String(value)) },
      { label: '复制列名', run: () => copyText(column.name || '') },
      { label: '用此值筛选', run: () => { props.tab.sql = filterSQL(driver.value, props.tab.sql, column.name || 'col', value); props.tab.readOnly = true } }
    )
  }
  items.push(
    { label: '复制行', run: () => copyText(row.map((cell) => (cell === null ? 'NULL' : cell)).join('\t')) },
    { label: '复制行为 JSON', run: () => copyText(JSON.stringify(Object.fromEntries((statement.columns || []).map((item, index) => [item.name, row[index]])), null, 2)) },
    { label: '复制为 INSERT', run: () => copyText(insertSQL(statement.columns, row)) },
    { label: '编辑行数据', run: () => editRow(statement, row) }
  )
  showMenu({ x: event.clientX, y: event.clientY, items })
}

function cellFromEvent(event) {
  const node = event.target?.nodeType === 1 ? event.target : event.target?.parentElement
  const cell = node?.closest?.('td')
  if (!cell?.parentElement) {
    return -1
  }
  return [...cell.parentElement.children].indexOf(cell) - 1
}

async function editRow(statement, row) {
  editError.value = ''
  editWarn.value = ''
  const sourceSQL = statement.sql || ((result.value?.statements || []).length === 1 ? result.value?.sql : '')
  const source = querySource(sourceSQL)
  if (!source) {
    error.value = '只能编辑单表查询的结果。请改成 select … from 表。'
    return
  }
  if (!props.tab.connectionId) {
    error.value = '请先选择连接。'
    return
  }
  const schema = source.schema || props.tab.schema || ''
  try {
    const body = { connection_id: props.tab.connectionId, schema, table: source.table }
    const columns = (await api('/api/catalog/columns', body)).data.columns || []
    const indexes = (await api('/api/catalog/indexes', body)).data.indexes || []
    if (!columns.length) {
      error.value = '没有读到这张表的字段，不能编辑行。'
      return
    }
    const keys = rowKeys(columns, indexes, statement.columns || [])
    if (!keys.names.length) {
      error.value = '结果里没有这张表的字段，不能编辑行。'
      return
    }
    editExact.value = keys.exact
    editWarn.value = keys.exact ? '' : '没有可用的主键或唯一索引，保存时会用当前行的全部字段去匹配，可能更新到多行。'
    editTitle.value = schema ? `${schema}.${source.table}` : source.table
    editFields.value = buildEditFields(columns, statement.columns || [], row, keys.names)
    editContext.value = { driver: driver.value, schema, table: source.table, statement, row }
    await nextTick()
    editDialog.value?.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function saveRow() {
  const current = editContext.value
  if (!current || editPending.value || !editDirty.value) {
    return
  }
  const sql = updateRowSQL(current.driver, current.schema, current.table, editFields.value)
  if (!sql) {
    editError.value = '没有要保存的修改。'
    return
  }
  if (!editExact.value && !window.confirm('没有主键或唯一索引，可能一次更新多行。确定保存？')) {
    return
  }
  editPending.value = true
  editError.value = ''
  try {
    const data = (await api('/api/query/execute', {
      connection_id: props.tab.connectionId,
      sql,
      schema: current.schema || props.tab.schema || '',
      read_only: false,
      max_rows: 1
    })).data
    const affected = Number(data.row_count ?? 0)
    if (!affected) {
      editError.value = '没有更新到行。数据可能已经变化，请重新查询。'
      return
    }
    const columns = current.statement.columns || []
    for (const field of editFields.value) {
      if (!fieldChanged(field)) {
        continue
      }
      const index = columns.findIndex((column) => String(column.name || '').toLowerCase() === field.name.toLowerCase())
      if (index >= 0) {
        current.row[index] = field.isNull ? null : field.value
      }
    }
    editDialog.value?.close()
    emit('ran', affected > 1 ? `已更新 ${affected} 行，结果里先改了当前这一行` : '已更新 1 行')
    loadHistory().catch(() => {})
  } catch (err) {
    editError.value = err.message
  } finally {
    editPending.value = false
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

function onSuggestPref(event) {
  suggestOn.value = !!event.detail
}

onMounted(() => {
  loadHints(props.tab.schema || '')
  loadHistory()
  window.addEventListener('pointerdown', closeMenu)
  window.addEventListener('dbgo-suggest', onSuggestPref)
  if (props.tab.autorun) {
    run()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('pointerdown', closeMenu)
  window.removeEventListener('dbgo-suggest', onSuggestPref)
})

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
      <label class="check"><input :checked="suggestOn" type="checkbox" aria-label="智能提示" @change="writeSuggestEnabled($event.target.checked)" /> 智能提示</label>
      <label class="check">最大行数 <input v-model.number="tab.maxRows" class="rows" type="number" min="1" max="1000" /></label>
      <button class="btn" type="button" :disabled="!tab.connectionId || hintPending" @click="loadHints(tab.schema)">刷新结构</button>
      <button class="btn" type="button" @click="format()">格式化</button>
      <button class="btn" type="button" :disabled="!tab.sql.trim()" @click="explain()">解释</button>
      <button class="btn" type="button" :disabled="!tab.connectionId || optimizing" @click="optimize()">AI 优化</button>
      <select v-model="snippetID" aria-label="代码段" @focus="loadSnippets">
        <option value="">代码段</option>
        <option v-for="item in snippets" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
      <button class="btn" type="button" :disabled="!snippetID" @click="insertSnippet">插入</button>
      <button class="btn" type="button" @click="emit('manage-snippets')">管理</button>
      <span class="script-hint">{{ driver ? driverLabel(driver) + ' · ' : '' }}{{ hintText }}</span>
    </div>
    <p v-if="hintError" class="notice script-alert">结构提示不可用：{{ hintError }}。关键字提示仍然可用。</p>
    <div class="script-editor">
      <SqlEditor ref="editorRef" :model-value="tab.sql" :error-line="errorLine" fill :driver="driver" :schema="tab.schema" :schemas="schemaNames" :tables="hintTables" @update:model-value="onSQL" @run="run" @menu="openEditorMenu" />
    </div>
    <template v-if="showResults">
      <div class="splitter-h" @mousedown="startDrag"></div>
      <section class="script-results" :style="{ height: resultsHeight + 'px' }">
        <div class="result-tabs">
          <button type="button" :class="{ on: panel === 'result' }" @click="panel = 'result'">结果</button>
          <button type="button" :class="{ on: panel === 'output' }" @click="panel = 'output'">输出<span v-if="outputLines.length" class="count">{{ outputLines.length }}</span></button>
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
          <template v-else-if="panel === 'output'">
            <p v-if="error" class="notice">{{ error }}</p>
            <div v-else-if="!result" class="empty">执行后在这里显示 PRINT 等消息。</div>
            <pre v-else-if="outputLines.length" class="output-console">{{ outputLines.join('\n') }}</pre>
            <div v-else class="empty">没有输出。</div>
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
              <div v-if="statement.columns?.length" class="table-wrap result-grid" @copy="onResultCopy($event, index)">
                <table>
                  <thead>
                    <tr>
                      <th class="rownum">#</th>
                      <th v-for="(column, columnIndex) in statement.columns" :key="columnIndex" :aria-sort="sortState(index, columnIndex)">
                        <button type="button" class="col-sort" :title="'按 ' + (column.name || '此列') + ' 排序'" @click="toggleSort(index, columnIndex)">
                          <span>{{ column.name }}</span>
                          <span v-if="sortState(index, columnIndex) === 'ascending'" class="sort-mark">↑</span>
                          <span v-else-if="sortState(index, columnIndex) === 'descending'" class="sort-mark">↓</span>
                        </button>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="item in displayRows(statement, index)" :key="item.index" @contextmenu="openRowMenu($event, statement, item.row)">
                      <td class="rownum">{{ item.index + 1 }}</td>
                      <td v-for="(cell, cellIndex) in item.row" :key="cellIndex" class="result-cell" :class="{ null: cell === null }" :data-row="item.index" :data-col="cellIndex" :data-long="cellPreview(cell).long ? '1' : undefined">{{ cellPreview(cell).text }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </article>
          </template>
        </div>
      </section>
    </template>
    <div v-if="menu" ref="menuEl" class="ctx" :style="{ left: menu.x + 'px', top: menu.y + 'px' }" role="menu">
      <button v-for="(item, index) in menu.items" :key="index" type="button" :disabled="item.disabled" @click="runMenu(item)">{{ item.label }}</button>
    </div>
    <dialog ref="editDialog" class="modal wide">
      <form @submit.prevent="saveRow">
        <h2>编辑行数据</h2>
        <p class="muted">{{ editTitle }}</p>
        <p v-if="editWarn" class="notice">{{ editWarn }}</p>
        <p v-if="editError" class="notice">{{ editError }}</p>
        <div class="edit-fields">
          <div v-for="field in editFields" :key="field.name" class="edit-field">
            <span class="edit-name">{{ field.name }} <small class="muted">{{ field.type }}{{ field.key ? ' · 定位' : '' }}{{ field.locked ? ' · 只读' : '' }}</small></span>
            <input v-model="field.value" :disabled="field.isNull || field.locked" :aria-label="field.name" />
            <label class="check"><input v-model="field.isNull" type="checkbox" :disabled="field.locked" /> NULL</label>
          </div>
        </div>
        <div class="row">
          <button class="btn" type="button" @click="editDialog?.close()">取消</button>
          <button class="btn btn-primary" type="submit" :disabled="editPending || !editDirty">保存</button>
        </div>
      </form>
    </dialog>
  </section>
</template>
