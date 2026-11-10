<script setup>
import { onMounted, reactive, ref } from 'vue'
import { api, formatTime } from '../api'

const rows = ref([])
const total = ref(0)
const error = ref('')
const notice = ref('')
const dialog = ref(null)
const form = reactive(blank())

function blank() {
  return {
    id: '',
    name: '',
    base_url: 'https://api.openai.com/v1',
    api_key: '',
    model: 'gpt-4o-mini',
    temperature: 0.2,
    max_tokens: 1200,
    is_default: true,
    remark: '',
    version: 0
  }
}

async function load() {
  error.value = ''
  const page = await api('/api/llm/search', { page: 1, page_size: 50, sort: 'name', direction: 'asc' })
  rows.value = page.data || []
  total.value = page.total
}

function openCreate() {
  Object.assign(form, blank(), { is_default: rows.value.length === 0 })
  dialog.value?.showModal()
}

async function openEdit(row) {
  error.value = ''
  try {
    const one = (await api('/api/llm/one', { id: row.id })).data
    Object.assign(form, blank(), one, { api_key: '' })
    dialog.value?.showModal()
  } catch (err) {
    error.value = err.message
  }
}

async function save() {
  error.value = ''
  const body = {
    name: form.name,
    base_url: form.base_url,
    api_key: form.api_key,
    model: form.model,
    temperature: Number(form.temperature),
    max_tokens: Number(form.max_tokens),
    is_default: form.is_default,
    remark: form.remark
  }
  try {
    if (form.id) {
      body.id = form.id
      body.version = form.version
      await api('/api/llm/update', body)
    } else {
      await api('/api/llm/create', body)
    }
    dialog.value?.close()
    await load()
  } catch (err) {
    error.value = err.message
  }
}

async function test(row) {
  error.value = ''
  notice.value = `正在测试 ${row.name}`
  try {
    const result = (await api('/api/llm/test', { id: row.id })).data
    notice.value = `${row.name}：${result.detail}，耗时 ${result.latency_ms} ms`
  } catch (err) {
    error.value = err.message
    notice.value = ''
  }
}

async function remove(row) {
  if (!window.confirm(`删除模型「${row.name}」？`)) {
    return
  }
  error.value = ''
  try {
    await api('/api/llm/delete', { id: row.id })
    await load()
  } catch (err) {
    error.value = err.message
  }
}

onMounted(async () => {
  try {
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
    <div class="toolbar">
      <p class="muted">使用 OpenAI 协议。地址填到 <code>/v1</code>，例如 <code>https://api.openai.com/v1</code> 或本地兼容服务。</p>
      <button class="btn btn-primary" type="button" @click="openCreate">新建模型</button>
    </div>
    <div class="table-wrap">
      <table>
        <thead><tr><th>名称</th><th>模型</th><th>地址</th><th>默认</th><th>密钥</th><th>更新时间</th><th></th></tr></thead>
        <tbody>
          <tr v-if="!rows.length"><td colspan="7" class="empty">还没有模型。配好之后，SQL 编辑器就可以做优化。</td></tr>
          <tr v-for="row in rows" :key="row.id">
            <td>{{ row.name }}</td>
            <td>{{ row.model }}</td>
            <td class="wrap">{{ row.base_url }}</td>
            <td>{{ row.is_default ? '是' : '' }}</td>
            <td>{{ row.api_key_set ? '已保存' : '无' }}</td>
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
    <p class="muted">共 {{ total }} 条。密钥只加密保存在本机，接口不回传原文。</p>
    <dialog ref="dialog" class="modal">
      <form class="stack" @submit.prevent="save">
        <h2>{{ form.id ? '编辑模型' : '新建模型' }}</h2>
        <div class="form-grid">
          <label class="field"><span>名称</span><input v-model="form.name" required /></label>
          <label class="field"><span>模型</span><input v-model="form.model" required /></label>
          <label class="field span-2"><span>服务地址</span><input v-model="form.base_url" required /></label>
          <label class="field span-2"><span>API Key</span><input v-model="form.api_key" type="password" :placeholder="form.id ? '留空则保持原密钥' : '本地服务可以留空'" /></label>
          <label class="field"><span>温度</span><input v-model.number="form.temperature" type="number" min="0" max="2" step="0.1" /></label>
          <label class="field"><span>最大令牌</span><input v-model.number="form.max_tokens" type="number" min="1" max="8192" /></label>
          <label class="field span-2"><span>备注</span><input v-model="form.remark" /></label>
          <label class="check span-2"><input v-model="form.is_default" type="checkbox" /> 作为默认模型</label>
        </div>
        <div class="actions">
          <button class="btn btn-primary" type="submit">保存</button>
          <button class="btn" type="button" @click="dialog.close()">取消</button>
        </div>
      </form>
    </dialog>
  </div>
</template>
