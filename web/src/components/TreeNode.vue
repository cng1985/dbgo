<script>
export default { name: 'TreeNode' }
</script>

<script setup>
import { computed, inject } from 'vue'
import { matchesText, nodeShown } from '../navTree.js'
import Icon from './Icon.vue'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  force: { type: Boolean, default: false }
})

const tree = inject('tree')

const query = computed(() => (tree.filter || '').trim().toLowerCase())
const visible = computed(() => nodeShown(props.node, query.value, props.force))
const childForce = computed(() => props.force || matchesText(props.node, query.value))
const rowTitle = computed(() => {
  if (props.node.kind === 'connection') {
    const parts = [props.node.label, props.node.meta]
    if (props.node.offline) {
      parts.push(props.node.failure || '无法连接')
    }
    return parts.filter(Boolean).join(' · ')
  }
  return props.node.offline ? (props.node.failure || '无法连接') : undefined
})
const selected = computed(() => tree.selectedId === props.node.id)
const icon = computed(() => {
  const kind = props.node.kind
  if (kind === 'connection') return 'database'
  if (kind === 'schema') return 'schema'
  if (kind === 'table') return 'table'
  if (kind === 'view') return 'view'
  if (kind === 'procedure' || kind === 'function') return 'sql'
  if (kind === 'key') return 'key'
  if (kind === 'column') return 'column'
  if (kind === 'index') return 'index'
  return 'folder'
})

function onToggle(event) {
  event.stopPropagation()
  tree.toggle(props.node)
}
</script>

<template>
  <div v-if="visible" class="tree-branch">
    <div
      class="tree-row"
      :class="{ selected, loading: node.loading, offline: node.offline, connection: node.kind === 'connection' }"
      :style="{ paddingLeft: (depth * 14 + 2) + 'px' }"
      :title="rowTitle"
      @click="tree.select(node)"
      @dblclick="tree.open(node)"
      @contextmenu="tree.context($event, node)"
    >
      <button v-if="!node.leaf" class="twisty" type="button" :aria-label="node.open ? '折叠' : '展开'" @click="onToggle">
        <Icon name="chevron" :class="{ open: node.open }" />
      </button>
      <span v-else class="twisty"></span>
      <span class="node-icon" :class="{ offline: node.offline }" :data-kind="node.kind" :data-driver="node.connection?.driver || ''">
        <Icon :name="icon" />
      </span>
      <span class="tree-label">{{ node.label }}</span>
      <span v-if="node.meta" class="tree-meta">{{ node.meta }}</span>
    </div>
    <div v-if="node.open">
      <div v-if="node.loading && !(node.children || []).length" class="tree-row hint" :style="{ paddingLeft: ((depth + 1) * 14 + 22) + 'px' }">正在读取…</div>
      <div v-else-if="node.error && !(node.children || []).length" class="tree-row hint bad" :style="{ paddingLeft: ((depth + 1) * 14 + 22) + 'px' }">{{ node.error }}</div>
      <div v-else-if="node.loaded && !(node.children || []).length" class="tree-row hint" :style="{ paddingLeft: ((depth + 1) * 14 + 22) + 'px' }">空</div>
      <TreeNode
        v-for="child in node.children || []"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :force="childForce"
      />
    </div>
  </div>
</template>
