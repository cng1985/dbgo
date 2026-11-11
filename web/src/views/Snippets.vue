<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { api, driverLabel, drivers, formatTime } from '../api'

const rows = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = 50
const keyword = ref('')
const error = ref('')
const notice = ref('')
const dialog = ref(null)
const fileInput = ref(null)
const conflict = ref('rename')
const form = reactive(blank())
const emit = defineEmits(['insert'])

function blank() {
  return { id: '', name: '', description: '', body: '', driver: '', version: 0 }
}

const pageCount = computed(() => Math.max(1, Math.ceil((total.value || 0) / pageSize)))

async function load() {
  error.value = ''
  const result = await api('/api/snippet/search', {
    keyword: keyword.value,
    page: page.value,
    page_size: pageSize,
    sort: 'name',
    direction: 'asc'
  })
  const data = result.data || []
  if (!data.length && page.value > 1) {
    page.value -= 1
    return load()
  }
  rows.value = data
  total.value = result.total || 0
}

function search() {
  page.value = 1
  return load()
}

function go(next) {
  page.value = next
  return load()
}

function openCreate() {
  Object.assign(form, blank())
  notice.value = ''
  dialog.value?.showModal()
}

async function openEdit(row) {
  error.value = ''
  try {
    const one = (await api('/api/snippet/one', { id: row.id })).data
    Object.assign(form, blank(), one)
    dialog.value?.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function save() {
  error.value = ''
  const body = {
    name: form.name,
    description: form.description,
    body: form.body,
    driver: form.driver
  }
  try {
    if (form.id) {
      body.id = form.id
      body.version = form.version
      await api('/api/snippet/update', body)
    } else {
      await api('/api/snippet/create', body)
    }
    dialog.value?.close()
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function remove(row) {
  if (!window.confirm(`删除代码段「${row.name}」？`)) {
    return
  }
  error.value = ''
  try {
    await api('/api/snippet/delete', { id: row.id })
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function insert(row) {
  error.value = ''
  try {
    const one = (await api('/api/snippet/one', { id: row.id })).data
    emit('insert', one.body || '')
  } catch (err) {
    error.value = err.message
  }
}

async function exportSnippets() {
  error.value = ''
  notice.value = ''
  try {
    const document = (await api('/api/snippet/export', {})).data
    if (!document.snippets?.length) {
      error.value = '没有可导出的代码段'
      return
    }
    const blob = new Blob([JSON.stringify(document, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = window.document.createElement('a')
    link.href = url
    link.download = 'dbgo-snippets.json'
    window.document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    notice.value = `已导出 ${document.snippets?.length || 0} 个代码段`
  } catch (err) {
    error.value = err.message
  }
}

async function onImport(event) {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) {
    return
  }
  error.value = ''
  notice.value = ''
  try {
    const document = JSON.parse(await file.text())
    const result = (await api('/api/snippet/import', {
      document,
      on_conflict: conflict.value
    })).data
    notice.value = `导入 ${result.created} 个，跳过 ${result.skipped} 个`
    await load()
  } catch (err) {
    error.value = err.message === 'Unexpected token' || err instanceof SyntaxError ? '代码段文件不是 JSON' : err.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <p v-if="error" class="notice">{{ error }}</p>
    <p v-if="notice" class="notice ok">{{ notice }}</p>
    <div class="toolbar">
      <div class="row grow">
        <input v-model="keyword" class="grow" placeholder="按名称或说明搜索" @keyup.enter="search" />
        <button class="btn" type="button" @click="search">搜索</button>
      </div>
      <select v-model="conflict" aria-label="重名策略">
        <option value="rename">重名则改名</option>
        <option value="skip">重名则跳过</option>
        <option value="error">重名则失败</option>
      </select>
      <button class="btn" type="button" @click="exportSnippets">导出</button>
      <button class="btn" type="button" @click="fileInput?.click()">导入</button>
      <input ref="fileInput" class="file-input" type="file" accept="application/json,.json" @change="onImport" />
      <button class="btn btn-primary" type="button" @click="openCreate">新建代码段</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead>
          <tr><th>名称</th><th>说明</th><th>数据库</th><th>更新时间</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="5" class="empty">还没有代码段。可以保存常用 SQL，也可以导入 JSON。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}</td>
            <td class="wrap">{{ row.description }}</td>
            <td>{{ row.driver ? driverLabel(row.driver) : '不限' }}</td>
            <td>{{ formatTime(row.updated_at) }}</td>
            <td class="actions">
              <button class="btn" type="button" @click="insert(row)">插入</button>
              <button class="btn" type="button" @click="openEdit(row)">编辑</button>
              <button class="btn btn-danger" type="button" @click="remove(row)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="row">
      <p class="muted grow">共 {{ total }} 条，第 {{ page }} / {{ pageCount }} 页。每个账号只能看到自己的代码段。</p>
      <button class="btn" type="button" :disabled="page <= 1" @click="go(page - 1)">上一页</button>
      <button class="btn" type="button" :disabled="page >= pageCount" @click="go(page + 1)">下一页</button>
    </div>
    <dialog ref="dialog" class="modal">
      <form class="stack" @submit.prevent="save">
        <h2>{{ form.id ? '编辑代码段' : '新建代码段' }}</h2>
        <div class="form-grid">
          <label class="field"><span>名称</span><input v-model="form.name" required /></label>
          <label class="field">
            <span>数据库</span>
            <select v-model="form.driver">
              <option value="">不限</option>
              <option v-for="item in drivers" :key="item.value" :value="item.value">{{ item.label }}</option>
            </select>
          </label>
          <label class="field span-2"><span>说明</span><input v-model="form.description" /></label>
          <label class="field span-2"><span>内容</span><textarea v-model="form.body" class="sql-box" required spellcheck="false" /></label>
        </div>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存</button>
          <button class="btn" type="button" @click="dialog.close()">取消</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
