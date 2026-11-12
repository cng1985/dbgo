<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api, driverLabel, formatBytes, formatTime } from '../api'
import { schemaPlaceholder } from '../catalogView.js'

const props = defineProps({
  preferredConnection: { type: String, default: '' },
  connections: { type: Array, default: () => [] },
  revision: { type: Number, default: 0 }
})

const connectionID = ref('')
const schema = ref('')
const snapshot = ref(null)
const error = ref('')
const pending = ref(false)
const schemaHint = computed(() => {
  const row = props.connections.find((item) => item.id === connectionID.value)
  return row ? schemaPlaceholder(row.driver, row.database_name) : '模式，留空用默认'
})

watch(() => props.connections, (list) => {
  if (!connectionID.value && list?.length) {
    connectionID.value = props.preferredConnection || list[0].id
  }
}, { immediate: true })

watch(() => props.preferredConnection, (value) => {
  if (value) {
    connectionID.value = value
    load()
  }
})

async function load() {
  error.value = ''
  if (!connectionID.value) {
    return
  }
  pending.value = true
  try {
    snapshot.value = (await api('/api/monitor/snapshot', {
      connection_id: connectionID.value,
      schema: schema.value
    })).data
    if (!schema.value && snapshot.value.schema) {
      schema.value = snapshot.value.schema
    }
  } catch (err) {
    error.value = err.message
  } finally {
    pending.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <div class="toolbar">
      <div class="row grow">
        <select v-model="connectionID" aria-label="连接">
          <option value="" disabled>选择连接</option>
          <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }} · {{ driverLabel(item.driver) }}</option>
        </select>
        <input v-model="schema" :placeholder="schemaHint" aria-label="模式" @keyup.enter="load" />
        <button class="btn btn-primary" type="button" :disabled="pending || !connectionID" @click="load">采集</button>
      </div>
      <span v-if="snapshot" class="muted">{{ snapshot.server_version }} · {{ formatTime(snapshot.collected_at) }}</span>
    </div>
    <div v-if="snapshot" class="cards">
      <article v-for="item in snapshot.metrics" :key="item.key" class="stat">
        <span class="muted">{{ item.label }}</span>
        <b>{{ item.unit === 'bytes' ? formatBytes(item.value) : item.value }}</b>
        <small v-if="item.hint" class="muted">{{ item.hint }}</small>
      </article>
    </div>
    <p v-for="(item, index) in snapshot?.notes || []" :key="index" class="muted">{{ item }}</p>
    <section class="card">
      <h2>会话</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>ID</th><th>用户</th><th>库</th><th>状态</th><th>耗时</th><th>SQL</th></tr></thead>
          <tbody>
            <tr v-if="!snapshot?.sessions?.length"><td colspan="6" class="empty">没有会话，或当前驱动不提供会话列表。</td></tr>
            <tr v-for="item in snapshot?.sessions || []" :key="item.id + item.sql">
              <td>{{ item.id }}</td>
              <td>{{ item.user }}</td>
              <td>{{ item.database }}</td>
              <td>{{ item.state }}</td>
              <td>{{ item.duration }}</td>
              <td class="wrap">{{ item.sql }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <section class="card">
      <h2>对象大小</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>模式</th><th>名称</th><th>行数</th><th>大小</th></tr></thead>
          <tbody>
            <tr v-if="!snapshot?.sizes?.length"><td colspan="4" class="empty">没有大小数据。</td></tr>
            <tr v-for="item in snapshot?.sizes || []" :key="item.schema + item.name">
              <td>{{ item.schema }}</td>
              <td>{{ item.name }}</td>
              <td>{{ item.rows }}</td>
              <td>{{ item.bytes ? formatBytes(item.bytes) : '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
