<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { api, driverLabel, drivers, formatTime } from '../api'

const rows = ref([])
const total = ref(0)
const keyword = ref('')
const driver = ref('')
const error = ref('')
const notice = ref('')
const dialog = ref(null)
const form = reactive(blank())
const props = defineProps({
  command: { type: Object, default: null },
  revision: { type: Number, default: 0 }
})
const emit = defineEmits(['changed'])
const network = computed(() => form.driver !== 'sqlite')

function blank() {
  return {
    id: '',
    name: '',
    driver: 'sqlite',
    host: '127.0.0.1',
    port: 0,
    database_name: '',
    username: '',
    password: '',
    sqlite_path: '',
    ssl_mode: 'disable',
    remark: '',
    version: 0
  }
}

async function load() {
  error.value = ''
  const page = await api('/api/connection/search', {
    keyword: keyword.value,
    driver: driver.value,
    page: 1,
    page_size: 50,
    sort: 'id',
    direction: 'desc'
  })
  rows.value = page.data
  total.value = page.total
}

function openCreate() {
  Object.assign(form, blank())
  notice.value = ''
  dialog.value?.showModal()
}

async function openEdit(row) {
  error.value = ''
  try {
    const one = (await api('/api/connection/one', { id: row.id })).data
    Object.assign(form, blank(), one, { password: '' })
    dialog.value.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function save() {
  error.value = ''
  const body = {
    name: form.name,
    driver: form.driver,
    host: form.host,
    port: Number(form.port || 0),
    database_name: form.database_name,
    username: form.username,
    password: form.password,
    sqlite_path: form.sqlite_path,
    ssl_mode: form.driver === 'postgres' ? form.ssl_mode : '',
    remark: form.remark
  }
  try {
    if (form.id) {
      body.id = form.id
      body.version = form.version
      await api('/api/connection/update', body)
    } else {
      await api('/api/connection/create', body)
    }
    dialog.value.close()
    await load()
    emit('changed')
  } catch (err) {
    error.value = err.message
  }
}

async function test(row) {
  error.value = ''
  notice.value = ''
  try {
    const result = (await api('/api/connection/test', { id: row.id })).data
    notice.value = `${result.name} 可连接，版本 ${result.server_version}，耗时 ${result.latency_ms ?? 0} ms`
  } catch (err) {
    error.value = err.message
  }
}

async function remove(row) {
  if (!window.confirm(`删除连接「${row.name}」？已保存的查询历史和备份记录会保留。`)) {
    return
  }
  error.value = ''
  try {
    await api('/api/connection/delete', { id: row.id })
    await load()
    emit('changed')
  } catch (err) {
    error.value = err.message
  }
}

function target(row) {
  if (row.driver === 'sqlite') {
    return row.sqlite_path
  }
  return `${row.host}:${row.port}/${row.database_name}`
}

watch(() => props.command, async (command) => {
  if (!command?.seq) {
    return
  }
  await nextTick()
  if (!dialog.value) {
    await nextTick()
  }
  if (command.type === 'create') {
    openCreate()
  } else if (command.type === 'edit' && command.id) {
    await openEdit({ id: command.id })
  }
}, { immediate: true, flush: 'post' })

watch(() => props.revision, (value, previous) => {
  if (previous === undefined || value === previous) {
    return
  }
  load()
})

onMounted(load)
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-if="notice" class="notice ok">{{ notice }}</p>
    <div class="toolbar">
      <div class="row grow">
        <input v-model="keyword" class="grow" placeholder="按名称搜索" @keyup.enter="load" />
        <select v-model="driver">
          <option value="">全部驱动</option>
          <option v-for="item in drivers" :key="item.value" :value="item.value">{{ item.label }}</option>
        </select>
        <button class="btn" type="button" @click="load">搜索</button>
      </div>
      <button class="btn btn-primary" type="button" @click="openCreate">新建连接</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr><th>名称</th><th>驱动</th><th>目标</th><th>版本</th><th>更新时间</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="6" class="empty">没有连接。可以先建一个 SQLite 文件。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}</td>
            <td><span class="tag">{{ driverLabel(row.driver) }}</span></td>
            <td class="wrap">{{ target(row) }}</td>
            <td>{{ row.version }}</td>
            <td>{{ formatTime(row.updated_at) }}</td>
            <td class="actions">
              <button class="btn" type="button" @click="test(row)">测试</button>
              <button class="btn" type="button" @click="openEdit(row)">编辑</button>
              <button class="btn btn-danger" type="button" @click="remove(row)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="muted">共 {{ total }} 条</p>
    <dialog ref="dialog" class="modal">
      <form class="stack" @submit.prevent="save">
        <h2>{{ form.id ? '编辑连接' : '新建连接' }}</h2>
        <div class="form-grid">
          <label class="field"><span>名称</span><input v-model="form.name" required /></label>
          <label class="field">
            <span>驱动</span>
            <select v-model="form.driver">
              <option v-for="item in drivers" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label v-if="!network" class="field span-2"><span>SQLite 文件</span><input v-model="form.sqlite_path" placeholder="相对数据目录，例如 demo.db" required /></label>
          <template v-else>
            <label class="field"><span>主机</span><input v-model="form.host" required /></label>
            <label class="field"><span>端口</span><input v-model.number="form.port" type="number" min="0" /></label>
            <label class="field"><span>数据库</span><input v-model="form.database_name" required /></label>
            <label class="field"><span>用户名</span><input v-model="form.username" required /></label>
            <label class="field span-2"><span>密码</span><input v-model="form.password" type="password" :placeholder="form.id ? '留空则保持原密码' : ''" /></label>
            <label v-if="form.driver === 'postgres'" class="field">
              <span>SSL</span>
              <select v-model="form.ssl_mode">
                <option value="disable">disable</option>
                <option value="require">require</option>
                <option value="verify-ca">verify-ca</option>
                <option value="verify-full">verify-full</option>
              </select>
            </label>
          </template>
          <label class="field span-2"><span>备注</span><input v-model="form.remark" /></label>
        </div>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存</button>
          <button class="btn" type="button" @click="dialog.close()">取消</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
