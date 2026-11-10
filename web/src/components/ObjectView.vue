<script setup>
import { onMounted, ref, watch } from 'vue'
import { api, driverLabel } from '../api'

const props = defineProps({
  connectionId: { type: String, default: '' },
  schema: { type: String, default: '' },
  table: { type: String, default: '' },
  driver: { type: String, default: '' }
})
const emit = defineEmits(['query'])

const columns = ref([])
const indexes = ref([])
const error = ref('')

async function load() {
  columns.value = []
  indexes.value = []
  error.value = ''
  if (!props.connectionId || !props.table) {
    return
  }
  try {
    const body = { connection_id: props.connectionId, schema: props.schema, table: props.table }
    columns.value = (await api('/api/catalog/columns', body)).data.columns || []
    indexes.value = (await api('/api/catalog/indexes', body)).data.indexes || []
  } catch (err) {
    error.value = err.message
  }
}

watch(() => [props.connectionId, props.schema, props.table], load)
onMounted(load)
</script>

<template>
  <div class="page object-view">
    <div class="toolbar">
      <div>
        <h2>{{ schema ? schema + '.' + table : table }}</h2>
        <p class="muted">{{ driverLabel(driver) }} · 字段和索引</p>
      </div>
      <button class="btn btn-primary" type="button" @click="emit('query')">查询数据</button>
    </div>
    <p v-if="error" class="notice">{{ error }}</p>
    <section class="card">
      <h2>字段</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>名称</th><th>类型</th><th>可空</th><th>主键</th><th>默认值</th></tr></thead>
          <tbody>
            <tr v-if="!columns.length"><td colspan="5" class="empty">没有字段。</td></tr>
            <tr v-for="item in columns" :key="item.name">
              <td>{{ item.name }}</td>
              <td>{{ item.type }}</td>
              <td>{{ item.nullable ? '是' : '否' }}</td>
              <td>{{ item.primary ? '是' : '' }}</td>
              <td>{{ item.default || '' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
    <section class="card">
      <h2>索引</h2>
      <div class="table-wrap">
        <table>
          <thead><tr><th>名称</th><th>唯一</th><th>列</th></tr></thead>
          <tbody>
            <tr v-if="!indexes.length"><td colspan="3" class="empty">没有索引。</td></tr>
            <tr v-for="item in indexes" :key="item.name">
              <td>{{ item.name }}</td>
              <td>{{ item.unique ? '是' : '否' }}</td>
              <td>{{ item.columns }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
