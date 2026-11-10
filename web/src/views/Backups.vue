<script setup>
import { onMounted, ref } from 'vue'
import { api, downloadBackup, formatBytes, formatTime, statusText } from '../api'

const connections = ref([])
const connectionID = ref('')
const name = ref('')
const format = ref('sql')
const rows = ref([])
const error = ref('')
const notice = ref('')
const pending = ref(false)

async function loadConnections() {
  const page = await api('/api/connection/search', { page: 1, page_size: 100, sort: 'name', direction: 'asc' })
  connections.value = page.data
  if (!connectionID.value && page.data.length) {
    connectionID.value = page.data[0].id
  }
}

async function load() {
  const page = await api('/api/backup/search', {
    connection_id: connectionID.value || '0',
    page: 1,
    page_size: 50
  })
  rows.value = page.data
}

async function create() {
  error.value = ''
  notice.value = ''
  pending.value = true
  try {
    const created = (await api('/api/backup/create', {
      connection_id: connectionID.value,
      name: name.value,
      format: format.value
    })).data
    notice.value = `备份已完成，${created.statement_count} 条语句，${formatBytes(created.size_bytes)}`
    name.value = ''
    await load()
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

async function restore(row) {
  if (!window.confirm(`把「${row.name}」恢复到原连接？这会执行备份中的 SQL。`)) {
    return
  }
  error.value = ''
  try {
    const result = (await api('/api/backup/restore', { id: row.id })).data
    notice.value = result.detail || '已恢复'
  } catch (err) {
    error.value = err.message
  }
}

async function download(row) {
  error.value = ''
  try {
    await downloadBackup(row.id, `${row.name}.${row.format === 'sqlite' ? 'db' : 'sql'}`)
  } catch (err) {
    error.value = err.message
  }
}

async function remove(row) {
  error.value = ''
  try {
    await api('/api/backup/delete', { id: row.id })
    await load()
  } catch (err) {
    error.value = err.message
  }
}

onMounted(async () => {
  try {
    await loadConnections()
    await load()
  } catch (err) {
    error.value = err.message
  }
})
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-if="notice" class="notice ok">{{ notice }}</p>
    <form class="card form-grid" @submit.prevent="create">
      <label class="field">
        <span>连接</span>
        <select v-model="connectionID" required>
          <option value="" disabled>选择连接</option>
          <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
      </label>
      <label class="field"><span>名称</span><input v-model="name" required placeholder="例如 上线前备份" /></label>
      <label class="field">
        <span>格式</span>
        <select v-model="format">
          <option value="sql">SQL 逻辑备份</option>
          <option value="sqlite">SQLite 文件副本</option>
        </select>
      </label>
      <div class="field">
        <span>操作</span>
        <button class="btn btn-primary" type="submit" :disabled="pending">创建备份</button>
      </div>
    </form>
    <div class="table-wrap">
      <table>
        <thead><tr><th>名称</th><th>连接</th><th>格式</th><th>状态</th><th>大小</th><th>时间</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="7" class="empty">还没有备份。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}<span v-if="row.truncated" class="tag">已截断</span></td>
            <td>{{ row.connection_name }}</td>
            <td>{{ row.format }}</td>
            <td><span class="tag" :class="row.status === 'failed' ? 'bad' : 'good'">{{ statusText[row.status] || row.status }}</span></td>
            <td>{{ formatBytes(row.size_bytes) }}</td>
            <td>{{ formatTime(row.created_at) }}</td>
            <td class="actions">
              <button class="btn" type="button" :disabled="row.status !== 'ready'" @click="download(row)">下载</button>
              <button class="btn" type="button" :disabled="row.status !== 'ready' || row.format !== 'sql'" @click="restore(row)">恢复</button>
              <button class="btn btn-danger" type="button" :disabled="row.status === 'running'" @click="remove(row)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
