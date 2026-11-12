<script setup>
import { ref } from 'vue'
import Icon from './Icon.vue'
import SqlEditor from './SqlEditor.vue'
import { procedureParams } from '../procedureScript.js'

const props = defineProps({
  connections: { type: Array, default: () => [] },
  connectionId: { type: String, default: '' },
  schema: { type: String, default: '' },
  schemaHint: { type: String, default: '' },
  driver: { type: String, default: '' },
  name: { type: String, default: '' },
  description: { type: String, default: '' },
  objectId: { type: String, default: '' },
  sql: { type: String, default: '' },
  pending: { type: Boolean, default: false },
  error: { type: String, default: '' },
  note: { type: String, default: '' },
  sqlite: { type: Boolean, default: false }
})
const emit = defineEmits([
  'connection', 'update:schema', 'update:name', 'update:description', 'update:sql', 'update:params', 'refresh', 'save'
])

const pane = ref('source')
const params = ref([])

function readParams() {
  params.value = procedureParams(props.sql).map((item) => ({ ...item }))
}

function showSource() {
  pane.value = 'source'
}

function showParams() {
  readParams()
  pane.value = 'params'
}

function blankParam() {
  return {
    name: '',
    type: props.driver === 'sqlserver' ? 'varchar(50)' : 'integer',
    mode: 'in',
    comment: ''
  }
}

function changeParam(index, field, value) {
  const next = params.value.map((item, itemIndex) => itemIndex === index ? { ...item, [field]: value } : item)
  params.value = next
  emit('update:params', next)
}

function addParam() {
  params.value = [...params.value, blankParam()]
}

function removeParam(index) {
  const next = params.value.filter((_, itemIndex) => itemIndex !== index)
  params.value = next
  emit('update:params', next)
}

defineExpose({ showParams, readParams })
</script>

<template>
  <div class="procedure-studio">
    <div class="procedure-meta">
      <label>连接
        <select :value="connectionId" aria-label="连接" @change="emit('connection', $event)">
          <option value="" disabled>选择连接</option>
          <option v-for="item in connections" :key="item.id" :value="item.id">{{ item.name }}</option>
        </select>
      </label>
      <label>模式
        <input :value="schema" :placeholder="schemaHint" aria-label="模式" @input="emit('update:schema', $event.target.value)" />
      </label>
      <label>存储过程名称
        <input :value="name" aria-label="存储过程名称" spellcheck="false" @input="emit('update:name', $event.target.value)" />
      </label>
      <label class="grow">存储过程描述
        <input :value="description" aria-label="存储过程描述" @input="emit('update:description', $event.target.value)" />
      </label>
      <label>ID
        <input :value="objectId" aria-label="ID" readonly />
      </label>
      <label>类型
        <select aria-label="类型" value="PROCEDURE">
          <option value="PROCEDURE">PROCEDURE</option>
        </select>
      </label>
    </div>
    <p v-if="sqlite" class="notice procedure-note">SQLite 不支持在控制台创建存储过程。</p>
    <p v-if="error" class="notice procedure-note">{{ error }}</p>
    <p v-if="note" class="notice ok procedure-note">{{ note }}</p>
    <div class="procedure-body">
      <nav class="procedure-side" aria-label="存储过程视图">
        <button type="button" :class="{ active: pane === 'params' }" :aria-pressed="pane === 'params'" @click="showParams">
          <Icon name="column" />
          <span>存储过程参数</span>
        </button>
        <button type="button" :class="{ active: pane === 'source' }" :aria-pressed="pane === 'source'" @click="showSource">
          <Icon name="sql" />
          <span>来源</span>
        </button>
      </nav>
      <div class="procedure-main">
        <div v-if="pane === 'params'" class="procedure-params">
          <div class="toolbar">
            <h2>存储过程参数</h2>
            <button class="btn" type="button" @click="addParam">添加参数</button>
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>名称</th><th>类型</th><th>方向</th><th>说明</th><th></th></tr></thead>
              <tbody>
                <tr v-if="!params.length"><td colspan="5" class="empty">这个过程还没有参数。</td></tr>
                <tr v-for="(param, index) in params" :key="index">
                  <td><input :value="param.name" aria-label="参数名" @input="changeParam(index, 'name', $event.target.value)" /></td>
                  <td><input :value="param.type" aria-label="参数类型" @input="changeParam(index, 'type', $event.target.value)" /></td>
                  <td>
                    <select :value="param.mode" aria-label="参数方向" @change="changeParam(index, 'mode', $event.target.value)">
                      <option value="in">in</option>
                      <option value="out">out</option>
                      <option value="inout">inout</option>
                    </select>
                  </td>
                  <td><input :value="param.comment" aria-label="参数说明" @input="changeParam(index, 'comment', $event.target.value)" /></td>
                  <td><button class="btn" type="button" @click="removeParam(index)">删除</button></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <SqlEditor v-else :model-value="sql" :driver="driver" :schema="schema" fill @update:model-value="emit('update:sql', $event)" />
      </div>
    </div>
    <footer class="procedure-foot">
      <button class="btn" type="button" :disabled="pending" @click="emit('refresh')">
        <Icon name="refresh" />
        刷新
      </button>
      <button class="btn btn-primary" type="button" :disabled="pending || sqlite" @click="emit('save')">保存</button>
      <span class="procedure-source">{{ pane === 'source' ? 'Source' : '参数' }}</span>
    </footer>
  </div>
</template>
