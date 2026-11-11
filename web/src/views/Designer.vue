<script setup>
import { computed, ref, watch } from 'vue'
import { api, driverLabel } from '../api'
import { columnTypeFor, columnTypes, defaultColumnType } from '../columnTypes.js'

const props = defineProps({
  command: { type: Object, default: null },
  preferredConnection: { type: String, default: '' },
  connections: { type: Array, default: () => [] }
})
const emit = defineEmits(['applied'])

const connectionID = ref('')
const schema = ref('')
const kind = ref('table')
const name = ref('')
const replace = ref(false)
const selectSQL = ref('select 1 as id')
const returns = ref('integer')
const language = ref('sql')
const body = ref('return 1')
const sql = ref('')
const note = ref('')
const error = ref('')
const pending = ref(false)
const driver = computed(() => props.connections.find((item) => item.id === connectionID.value)?.driver || 'sqlite')
const typeOptions = computed(() => columnTypes(driver.value))
const isRoutine = computed(() => kind.value === 'procedure' || kind.value === 'function')
const sqliteRoutine = computed(() => driver.value === 'sqlite' && isRoutine.value)

function blankColumn(primary = false) {
  return { name: primary ? 'id' : '', type: defaultColumnType(driver.value), nullable: !primary, primary, identity: primary, default: '' }
}

function blankParam() {
  return { name: 'p_id', type: 'integer', mode: 'in' }
}

const columns = ref([blankColumn(true)])
const params = ref([])

watch(driver, (value) => {
  for (const column of columns.value) {
    column.type = columnTypeFor(value, column.type)
  }
})

watch(() => props.connections, (list) => {
  if (!connectionID.value && list?.length) {
    connectionID.value = props.preferredConnection || list[0].id
  }
}, { immediate: true })

watch(() => props.command, async (command) => {
  if (!command?.seq) {
    return
  }
  kind.value = command.kind || 'table'
  if (command.connectionId) {
    connectionID.value = command.connectionId
  }
  schema.value = command.schema || ''
  name.value = command.name || ''
  replace.value = command.type === 'edit'
  error.value = ''
  note.value = ''
  if (command.type === 'edit' && command.name) {
    await loadDefinition()
  }
}, { immediate: true })

function payload() {
  return {
    driver: driver.value,
    schema: schema.value,
    kind: kind.value,
    name: name.value,
    replace: replace.value,
    columns: columns.value.filter((item) => item.name.trim()),
    select_sql: selectSQL.value,
    params: params.value.filter((item) => item.name.trim()),
    returns: returns.value,
    language: language.value,
    body: body.value
  }
}

async function preview() {
  error.value = ''
  note.value = ''
  pending.value = true
  try {
    const result = (await api('/api/catalog/preview', payload())).data
    sql.value = result.sql || ''
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function loadDefinition() {
  error.value = ''
  if (!connectionID.value || !name.value) {
    error.value = '请选择连接并填写对象名'
    return
  }
  pending.value = true
  try {
    const result = (await api('/api/catalog/definition', {
      connection_id: connectionID.value,
      schema: schema.value,
      name: name.value,
      kind: kind.value
    })).data
    sql.value = result.sql || ''
    replace.value = kind.value !== 'table'
    const hints = [result.note || '']
    if (kind.value === 'table') {
      hints.push('这是当前定义。勾选“替换已有对象”后重新生成 SQL 会先删除这张表。')
    } else if (driver.value === 'sqlite' && kind.value === 'view') {
      hints.push('SQLite 需要先删除同名视图。勾选替换后重新生成，或在语句前写 DROP VIEW IF EXISTS。')
    }
    note.value = hints.filter(Boolean).join(' ')
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function apply() {
  error.value = ''
  if (!connectionID.value) {
    error.value = '请选择连接'
    return
  }
  if (!sql.value.trim()) {
    await preview()
    if (!sql.value.trim()) {
      return
    }
  }
  if (!window.confirm('将在目标库执行编辑器中的语句。表的“替换”会先删除同名表。')) {
    return
  }
  pending.value = true
  try {
    const result = (await api('/api/catalog/apply', {
      connection_id: connectionID.value,
      sql: sql.value
    })).data
    note.value = `已执行 ${result.statements} 条语句，耗时 ${result.duration_ms} ms`
    emit('applied')
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-if="note" class="notice ok">{{ note }}</p>
    <p v-if="sqliteRoutine" class="notice">SQLite 不支持在控制台创建存储过程或函数。</p>
    <div class="toolbar">
      <div class="row grow">
        <select v-model="connectionID" aria-label="连接">
          <option value="" disabled>选择连接</option>
          <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
        <input v-model="schema" placeholder="模式，可空" aria-label="模式" />
        <select v-model="kind" aria-label="对象类型">
          <option value="table">表</option>
          <option value="view">视图</option>
          <option value="procedure">存储过程</option>
          <option value="function">函数</option>
        </select>
        <input v-model="name" placeholder="对象名" aria-label="对象名" />
        <label class="check"><input v-model="replace" type="checkbox" /> 替换已有对象</label>
      </div>
      <div class="row">
        <button class="btn" type="button" :disabled="pending" @click="loadDefinition">读取定义</button>
        <button class="btn" type="button" :disabled="pending" @click="preview">生成 SQL</button>
        <button class="btn btn-primary" type="button" :disabled="pending" @click="apply">执行</button>
      </div>
    </div>

    <div v-if="kind === 'table'" class="card">
      <div class="toolbar">
        <div class="row">
          <h2>列</h2>
          <span class="muted">{{ driverLabel(driver) }}</span>
        </div>
        <button class="btn" type="button" @click="columns.push(blankColumn(false))">添加列</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>名称</th><th>类型</th><th>可空</th><th>主键</th><th>自增</th><th>默认值</th><th></th></tr></thead>
          <tbody>
            <tr v-for="(column, index) in columns" :key="index">
              <td><input v-model="column.name" aria-label="列名" /></td>
              <td>
                <select v-model="column.type" aria-label="类型">
                  <option v-for="item in typeOptions" :key="item" :value="item">{{ item }}</option>
                </select>
              </td>
              <td><input v-model="column.nullable" type="checkbox" aria-label="可空" /></td>
              <td><input v-model="column.primary" type="checkbox" aria-label="主键" /></td>
              <td><input v-model="column.identity" type="checkbox" aria-label="自增" /></td>
              <td><input v-model="column.default" aria-label="默认值" /></td>
              <td><button class="btn" type="button" @click="columns.splice(index, 1)">删除</button></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="kind === 'view'" class="card">
      <h2>视图查询</h2>
      <label class="field"><span>SELECT</span><textarea v-model="selectSQL" class="sql-box" /></label>
    </div>

    <div v-else class="card stack">
      <div class="toolbar">
        <h2>参数</h2>
        <button class="btn" type="button" @click="params.push(blankParam())">添加参数</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>名称</th><th>类型</th><th>方向</th><th></th></tr></thead>
          <tbody>
            <tr v-if="!params.length"><td colspan="4" class="empty">可以没有参数。</td></tr>
            <tr v-for="(param, index) in params" :key="index">
              <td><input v-model="param.name" aria-label="参数名" /></td>
              <td><input v-model="param.type" aria-label="参数类型" /></td>
              <td>
                <select v-model="param.mode" aria-label="参数方向">
                  <option value="in">in</option>
                  <option value="out">out</option>
                  <option value="inout">inout</option>
                </select>
              </td>
              <td><button class="btn" type="button" @click="params.splice(index, 1)">删除</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="form-grid">
        <label v-if="kind === 'function'" class="field"><span>返回类型</span><input v-model="returns" /></label>
        <label class="field"><span>语言</span>
          <select v-model="language">
            <option value="sql">sql</option>
            <option value="plpgsql">plpgsql</option>
          </select>
        </label>
      </div>
      <label class="field"><span>主体</span><textarea v-model="body" class="sql-box" /></label>
    </div>

    <label class="field">
      <span>将要执行的 SQL。可以改完再执行。</span>
      <textarea v-model="sql" class="sql-box" spellcheck="false" />
    </label>
  </div>
</template>
