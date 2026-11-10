<script>
export default { name: 'TreeNode' }
</script>

<script setup>
import { computed, inject } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  force: { type: Boolean, default: false }
})

const tree = inject('tree')

const query = computed(() => (tree.filter || '').trim().toLowerCase())

function ownMatch(node, q) {
  if (!q) {
    return false
  }
  return (node.label || '').toLowerCase().includes(q) || (node.meta || '').toLowerCase().includes(q)
}

function subtreeMatch(node, q) {
  if (!q) {
    return true
  }
  if (ownMatch(node, q)) {
    return true
  }
  if (!node.loaded && !node.leaf && (node.kind === 'connection' || node.kind === 'schema')) {
    return true
  }
  return (node.children || []).some((child) => subtreeMatch(child, q))
}

const visible = computed(() => !query.value || props.force || subtreeMatch(props.node, query.value))
const childForce = computed(() => props.force || ownMatch(props.node, query.value))
const selected = computed(() => tree.selectedId === props.node.id)
const icon = computed(() => {
  const kind = props.node.kind
  if (kind === 'connection') return 'database'
  if (kind === 'schema') return 'schema'
  if (kind === 'table') return 'table'
  if (kind === 'view') return 'view'
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
      :class="{ selected, loading: node.loading }"
      :style="{ paddingLeft: (depth * 14 + 2) + 'px' }"
      @click="tree.select(node)"
      @dblclick="tree.open(node)"
      @contextmenu="tree.context($event, node)"
    >
      <button v-if="!node.leaf" class="twisty" type="button" :aria-label="node.open ? '折叠' : '展开'" @click="onToggle">
        <Icon name="chevron" :class="{ open: node.open }" />
      </button>
      <span v-else class="twisty"></span>
      <span class="node-icon" :data-kind="node.kind" :data-driver="node.connection?.driver || ''">
        <Icon :name="icon" />
      </span>
      <span class="tree-label">{{ node.label }}</span>
      <span v-if="node.meta" class="tree-meta">{{ node.meta }}</span>
    </div>
    <div v-if="node.open">
      <div v-if="node.loading" class="tree-row hint" :style="{ paddingLeft: ((depth + 1) * 14 + 22) + 'px' }">正在读取…</div>
      <div v-else-if="node.error" class="tree-row hint bad" :style="{ paddingLeft: ((depth + 1) * 14 + 22) + 'px' }">{{ node.error }}</div>
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
