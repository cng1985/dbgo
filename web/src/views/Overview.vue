<script setup>
import { onMounted, ref } from 'vue'
import { api, formatTime, statusText } from '../api'

const data = ref(null)
const error = ref('')

const cards = [
  { key: 'connection_count', label: '连接' },
  { key: 'query_count', label: '查询' },
  { key: 'failed_query_count', label: '失败查询' },
  { key: 'applied_migration_count', label: '已执行迁移' },
  { key: 'backup_count', label: '备份' }
]

onMounted(async () => {
  try {
    data.value = (await api('/api/overview/one', {})).data
  } catch (err) {
    error.value = err.message
  }
})
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <div v-if="data" class="cards">
      <article v-for="card in cards" :key="card.key" class="stat">
        <span class="muted">{{ card.label }}</span>
        <b>{{ data[card.key] }}</b>
      </article>
    </div>
    <section class="card">
      <h2>最近查询</h2>
      <div v-if="!data?.recent_queries?.length" class="empty">还没有查询记录。</div>
      <div v-else class="table-wrap">
        <table>
          <thead>
            <tr><th>连接</th><th>状态</th><th>行数</th><th>耗时</th><th>时间</th></tr>
          </thead>
          <tbody>
            <tr v-for="item in data.recent_queries" :key="item.id">
              <td>{{ item.connection_name }}</td>
              <td><span class="tag" :class="item.status === 'failed' ? 'bad' : 'good'">{{ statusText[item.status] || item.status }}</span></td>
              <td>{{ item.row_count }}</td>
              <td>{{ item.duration_ms }} ms</td>
              <td>{{ formatTime(item.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
