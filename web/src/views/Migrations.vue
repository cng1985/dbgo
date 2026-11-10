<script setup>
import { onMounted, reactive, ref, watch } from 'vue'
import { api, formatTime, statusText } from '../api'

const connections = ref([])
const connectionID = ref('')
const rows = ref([])
const error = ref('')
const dialog = ref(null)
const form = reactive(blank())
const props = defineProps({
  preferredConnection: { type: String, default: '' },
  revision: { type: Number, default: 0 }
})
let ready = false

function blank() {
  return { id: '', connection_id: '', version_no: 1, name: '', up_sql: '', down_sql: '' }
}

async function loadConnections() {
  const page = await api('/api/connection/search', { page: 1, page_size: 100, sort: 'name', direction: 'asc' })
  connections.value = page.data
  if (!connectionID.value && page.data.length) {
    connectionID.value = page.data[0].id
  }
}

async function load() {
  if (!connectionID.value) {
    rows.value = []
    return
  }
  const page = await api('/api/migration/search', { connection_id: connectionID.value, page: 1, page_size: 100 })
  rows.value = page.data
}

function openCreate() {
  Object.assign(form, blank(), { connection_id: connectionID.value, version_no: (rows.value.at(-1)?.version_no || 0) + 1 })
  dialog.value.showModal()
}

async function openEdit(row) {
  const one = (await api('/api/migration/one', { id: row.id })).data
  Object.assign(form, one)
  dialog.value.showModal()
}

async function save() {
  error.value = ''
  const body = {
    connection_id: form.connection_id || connectionID.value,
    version_no: Number(form.version_no),
    name: form.name,
    up_sql: form.up_sql,
    down_sql: form.down_sql
  }
  try {
    if (form.id) {
      body.id = form.id
      await api('/api/migration/update', body)
    } else {
      await api('/api/migration/create', body)
    }
    dialog.value.close()
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function act(path, row, extra) {
  error.value = ''
  try {
    await api(path, { id: row.id, connection_id: row.connection_id, ...extra })
    await load()
  } catch (err) {
    error.value = err.message
    await load()
  }
}

watch(() => props.preferredConnection, (id) => {
  if (ready && id && id !== connectionID.value) {
    connectionID.value = id
  }
})

watch(connectionID, (id, previous) => {
  if (!ready || id === previous) {
    return
  }
  load()
})

watch(() => props.revision, (value, previous) => {
  if (!ready || previous === undefined || value === previous) {
    return
  }
  loadConnections()
})

onMounted(async () => {
  try {
    if (props.preferredConnection) {
      connectionID.value = props.preferredConnection
    }
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
    <div class="toolbar">
      <div class="row">
        <select v-model="connectionID" @change="load">
          <option value="" disabled>选择连接</option>
          <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
      </div>
      <button class="btn btn-primary" type="button" :disabled="!connectionID" @click="openCreate">新建迁移</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>版本</th><th>名称</th><th>状态</th><th>执行时间</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="5" class="empty">这个连接还没有迁移。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.version_no }}</td>
            <td>{{ row.name }}</td>
            <td><span class="tag" :class="row.status === 'failed' ? 'bad' : row.status === 'applied' ? 'good' : ''">{{ statusText[row.status] || row.status }}</span></td>
            <td>{{ formatTime(row.applied_at) }}</td>
            <td class="actions">
              <button v-if="row.status === 'draft' || row.status === 'failed'" class="btn" type="button" @click="openEdit(row)">编辑</button>
              <button v-if="row.status === 'draft'" class="btn btn-primary" type="button" @click="act('/api/migration/apply', row)">执行</button>
              <button v-if="row.status === 'applied'" class="btn" type="button" @click="act('/api/migration/rollback', row)">回滚</button>
              <button v-if="row.status === 'applying' || row.status === 'reverting'" class="btn" type="button" @click="act('/api/migration/resolve', row, { action: 'abort' })">中止</button>
              <button v-if="row.status === 'applying' || row.status === 'reverting'" class="btn" type="button" @click="act('/api/migration/resolve', row, { action: 'confirm' })">确认</button>
              <button v-if="row.status === 'draft' || row.status === 'failed'" class="btn btn-danger" type="button" @click="act('/api/migration/delete', row)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <dialog ref="dialog" class="modal">
      <form class="stack" @submit.prevent="save">
        <h2>{{ form.id ? '编辑迁移' : '新建迁移' }}</h2>
        <div class="form-grid">
          <label class="field"><span>版本号</span><input v-model.number="form.version_no" type="number" min="1" required :disabled="!!form.id" /></label>
          <label class="field"><span>名称</span><input v-model="form.name" required /></label>
          <label class="field span-2"><span>升级 SQL</span><textarea v-model="form.up_sql" required /></label>
          <label class="field span-2"><span>回滚 SQL</span><textarea v-model="form.down_sql" /></label>
        </div>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存</button>
          <button class="btn" type="button" @click="dialog.close()">取消</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
