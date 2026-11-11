<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { api, formatTime, statusText } from '../api'

const props = defineProps({
  preferredConnection: { type: String, default: '' },
  revision: { type: Number, default: 0 }
})

const connections = ref([])
const rows = ref([])
const runs = ref([])
const error = ref('')
const notice = ref('')
const formError = ref('')
const formNote = ref('')
const dialog = ref(null)
const runsDialog = ref(null)
const runsTask = ref(null)
const runningID = ref('')
const abortingID = ref('')
const sourceColumns = ref([])
const targetColumns = ref([])
const previewRows = ref([])
const form = reactive(blank())
let ready = false

const modeText = { append: '追加', replace: '清空后写入' }

function blank() {
  return {
    id: '',
    version: 0,
    name: '',
    source_connection_id: '',
    source_sql: '',
    target_connection_id: '',
    target_schema: '',
    target_table: '',
    write_mode: 'append',
    mappings: []
  }
}

async function loadConnections() {
  const page = await api('/api/connection/search', { page: 1, page_size: 100, sort: 'name', direction: 'asc' })
  connections.value = page.data || []
}

async function load() {
  const page = await api('/api/etl/search', { page: 1, page_size: 100 })
  rows.value = page.data || []
}

function openCreate() {
  const source = props.preferredConnection || connections.value[0]?.id || ''
  Object.assign(form, blank(), { source_connection_id: source, target_connection_id: source })
  sourceColumns.value = []
  targetColumns.value = []
  previewRows.value = []
  formError.value = ''
  formNote.value = ''
  dialog.value?.showModal()
}

async function openEdit(row) {
  formError.value = ''
  try {
    const one = (await api('/api/etl/one', { id: row.id })).data
    Object.assign(form, blank(), one, {
      mappings: (one.mappings || []).map((item) => ({ source: item.source, target: item.target }))
    })
    sourceColumns.value = []
    targetColumns.value = []
    previewRows.value = []
    formNote.value = ''
    dialog.value?.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function save() {
  formError.value = ''
  const mappings = form.mappings.filter((item) => item.source.trim() || item.target.trim())
  const body = {
    name: form.name,
    source_connection_id: form.source_connection_id,
    source_sql: form.source_sql,
    target_connection_id: form.target_connection_id,
    target_schema: form.target_schema,
    target_table: form.target_table,
    write_mode: form.write_mode,
    mappings
  }
  try {
    if (form.id) {
      body.id = form.id
      body.version = form.version
      await api('/api/etl/update', body)
    } else {
      await api('/api/etl/create', body)
    }
    dialog.value?.close()
    notice.value = '任务已保存'
    await load()
  } catch (err) {
    formError.value = err.message
  }
}

async function readSource() {
  formError.value = ''
  try {
    const data = (await api('/api/etl/preview', {
      source_connection_id: form.source_connection_id,
      source_sql: form.source_sql
    })).data
    sourceColumns.value = data.columns || []
    previewRows.value = data.rows || []
    if (!form.mappings.length) {
      form.mappings = sourceColumns.value.map((name) => ({ source: name, target: name }))
    }
    formNote.value = data.truncated ? '只显示了前 20 行，执行时会继续抽取。' : ''
  } catch (err) {
    formError.value = err.message
  }
}

async function readTarget() {
  formError.value = ''
  try {
    const data = (await api('/api/catalog/columns', {
      connection_id: form.target_connection_id,
      schema: form.target_schema,
      table: form.target_table
    })).data
    targetColumns.value = (data.columns || []).map((item) => item.name)
  } catch (err) {
    formError.value = err.message
  }
}

function findName(names, name) {
  return names.find((item) => item === name) || names.find((item) => item.toLowerCase() === name.toLowerCase())
}

function matchNames() {
  formError.value = ''
  if (!sourceColumns.value.length) {
    formError.value = '请先读取源字段'
    return
  }
  const targets = targetColumns.value
  const matched = sourceColumns.value.filter((name) => !targets.length || findName(targets, name))
  form.mappings = matched.map((name) => ({
    source: name,
    target: findName(targets, name) || name
  }))
  if (!form.mappings.length) {
    formError.value = '没有同名字段'
  }
}

function pending(row) {
  return runningID.value === row.id || row.last_status === 'running'
}

async function run(row) {
  if (row.write_mode === 'replace' && !window.confirm(`清空后写入会先删除「${row.target_table}」里的全部行。确定执行？`)) {
    return
  }
  error.value = ''
  notice.value = ''
  runningID.value = row.id
  try {
    const result = (await api('/api/etl/run', { id: row.id })).data
    notice.value = `已写入 ${result.rows_written} 行`
  } catch (err) {
    if (abortingID.value !== row.id) {
      error.value = err.message || '执行没有返回结果'
    }
  } finally {
    runningID.value = ''
    abortingID.value = ''
    try {
      await load()
    } catch (err) {
      error.value = err.message
    }
  }
}

async function abort(row) {
  error.value = ''
  abortingID.value = row.id
  try {
    await api('/api/etl/abort', { id: row.id })
    notice.value = '已中止'
    await load()
    if (runsTask.value?.id === row.id) {
      await loadRuns(row)
    }
  } catch (err) {
    error.value = err.message
  }
}

async function remove(row) {
  if (!window.confirm(`删除任务「${row.name}」？执行记录会一起删除，目标表里的数据保留。`)) {
    return
  }
  error.value = ''
  try {
    await api('/api/etl/delete', { id: row.id })
    notice.value = '任务已删除'
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function openRuns(row) {
  runsTask.value = row
  error.value = ''
  try {
    await loadRuns(row)
    runsDialog.value?.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function loadRuns(row) {
  const page = await api('/api/etl/runs', { task_id: row.id, page: 1, page_size: 50 })
  runs.value = page.data || []
}

function targetLabel(row) {
  return row.target_schema ? `${row.target_schema}.${row.target_table}` : row.target_table
}

function lastText(row) {
  if (runningID.value === row.id) {
    return '进行中'
  }
  if (!row.last_status) {
    return '还没执行'
  }
  const label = statusText[row.last_status] || row.last_status
  if (row.last_status === 'succeeded') {
    return `${label}，写入 ${row.last_rows_written} 行`
  }
  if (row.last_error) {
    return `${label}：${row.last_error}`
  }
  return label
}

watch(() => props.revision, (value, previous) => {
  if (!ready || previous === undefined || value === previous) {
    return
  }
  loadConnections().catch((err) => { error.value = err.message })
})

onMounted(async () => {
  try {
    await loadConnections()
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    ready = true
  }
})
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-else-if="notice" class="muted">{{ notice }}</p>
    <div class="toolbar">
      <div>
        <h2>ETL</h2>
        <p class="muted">用一条查询抽取数据，按字段对应写入另一张表。转换写在抽取语句里。</p>
      </div>
      <button class="btn btn-primary" type="button" @click="openCreate">新建任务</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr><th>名称</th><th>来源</th><th>目标</th><th>写入</th><th>最近执行</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="6" class="empty">还没有 ETL 任务。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}</td>
            <td>{{ row.source_connection_name }}</td>
            <td>{{ row.target_connection_name }} · {{ targetLabel(row) }}</td>
            <td>{{ modeText[row.write_mode] || row.write_mode }}</td>
            <td>
              <span class="tag" :class="runningID === row.id ? '' : row.last_status === 'failed' ? 'bad' : row.last_status === 'succeeded' ? 'good' : ''">{{ lastText(row) }}</span>
            </td>
            <td class="actions">
              <button class="btn" type="button" @click="openEdit(row)">编辑</button>
              <button class="btn btn-primary" type="button" :disabled="runningID === row.id" @click="run(row)">执行</button>
              <button v-if="pending(row)" class="btn" type="button" @click="abort(row)">中止</button>
              <button class="btn" type="button" @click="openRuns(row)">记录</button>
              <button class="btn btn-danger" type="button" @click="remove(row)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <dialog ref="dialog" class="modal wide">
      <form class="stack" @submit.prevent="save">
        <h2>{{ form.id ? '编辑任务' : '新建任务' }}</h2>
        <p v-if="formError" class="notice">{{ formError }}</p>
        <p v-else-if="formNote" class="muted">{{ formNote }}</p>
        <div class="form-grid">
          <label class="field span-2"><span>名称</span><input v-model="form.name" required maxlength="80" /></label>
          <label class="field"><span>来源连接</span>
            <select v-model="form.source_connection_id" required>
              <option value="" disabled>选择连接</option>
              <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label class="field"><span>目标连接</span>
            <select v-model="form.target_connection_id" required>
              <option value="" disabled>选择连接</option>
              <option v-for="item in connections" :key="'t-' + item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label class="field span-2"><span>抽取语句</span>
            <textarea v-model="form.source_sql" required placeholder="select id, name from source_table" />
          </label>
          <div class="span-2 row">
            <button class="btn" type="button" @click="readSource">读取源字段</button>
            <span v-if="sourceColumns.length" class="muted">{{ sourceColumns.length }} 个字段</span>
          </div>
          <div v-if="previewRows.length" class="span-2 table-wrap">
            <table>
              <thead><tr><th v-for="name in sourceColumns" :key="name">{{ name }}</th></tr></thead>
              <tbody>
                <tr v-for="(row, index) in previewRows.slice(0, 5)" :key="index">
                  <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell === null ? 'NULL' : cell }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <label class="field"><span>目标模式</span><input v-model="form.target_schema" placeholder="留空使用默认模式" /></label>
          <label class="field"><span>目标表</span><input v-model="form.target_table" required /></label>
          <label class="field"><span>写入方式</span>
            <select v-model="form.write_mode">
              <option value="append">追加</option>
              <option value="replace">清空后写入</option>
            </select>
          </label>
          <div class="row">
            <button class="btn" type="button" @click="readTarget">读取目标字段</button>
            <button class="btn" type="button" @click="matchNames">按同名对应</button>
          </div>
          <div class="span-2">
            <div class="map-list">
              <div v-for="(item, index) in form.mappings" :key="index" class="map-row">
                <input v-model="item.source" list="etl-source-columns" aria-label="来源字段" placeholder="来源字段" />
                <span class="map-arrow">→</span>
                <input v-model="item.target" list="etl-target-columns" aria-label="目标字段" placeholder="目标字段" />
                <button class="btn" type="button" @click="form.mappings.splice(index, 1)">删除</button>
              </div>
            </div>
            <datalist id="etl-source-columns">
              <option v-for="name in sourceColumns" :key="name" :value="name" />
            </datalist>
            <datalist id="etl-target-columns">
              <option v-for="name in targetColumns" :key="name" :value="name" />
            </datalist>
            <button class="btn" type="button" @click="form.mappings.push({ source: '', target: '' })">添加字段</button>
            <p class="muted">不填字段对应时，按抽取结果的字段名写入。追加会接在现有行后面；清空后写入失败会整段回滚。</p>
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存</button>
          <button class="btn" type="button" @click="dialog.close()">取消</button>
        </div>
      </form>
    </dialog>

    <dialog ref="runsDialog" class="modal wide">
      <form class="stack" @submit.prevent="runsDialog.close()">
        <h2>{{ runsTask?.name }} 的执行记录</h2>
        <div class="table-wrap">
          <table>
            <thead><tr><th>开始</th><th>状态</th><th>读取</th><th>写入</th><th>说明</th></tr></thead>
            <tbody>
              <tr v-if="!runs.length"><td colspan="5" class="empty">还没有执行记录。</td></tr>
              <tr v-for="item in runs" :key="item.id">
                <td>{{ formatTime(item.started_at) }}</td>
                <td><span class="tag" :class="item.status === 'failed' ? 'bad' : item.status === 'succeeded' ? 'good' : ''">{{ statusText[item.status] || item.status }}</span></td>
                <td>{{ item.rows_read }}</td>
                <td>{{ item.rows_written }}</td>
                <td>{{ item.error_message || '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="actions">
          <button class="btn" type="submit">关闭</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
