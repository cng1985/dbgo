<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { highlightSQL, suggestSQL } from '../sqlEditor.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  driver: { type: String, default: '' },
  schema: { type: String, default: '' },
  schemas: { type: Array, default: () => [] },
  tables: { type: Array, default: () => [] },
  fill: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'run'])

const root = ref(null)
const input = ref(null)
const backdrop = ref(null)
const gutter = ref(null)
const menu = ref(null)
const text = ref(props.modelValue || '')
const items = ref([])
const suggestion = ref({ from: 0, to: 0, prefix: '', mode: 'any', autoOpen: false })
const open = ref(false)
const active = ref(0)
const menuStyle = ref({})
const kindLabel = { keyword: '关键字', table: '表', column: '字段', schema: '模式' }
let held = false

const html = computed(() => highlightSQL(text.value, props.driver, props.tables))
const lineNumbers = computed(() => {
  const count = Math.max(1, text.value.split('\n').length)
  return Array.from({ length: count }, (_, index) => String(index + 1)).join('\n')
})
const rows = computed(() => {
  const list = []
  let section = ''
  items.value.forEach((item, itemIndex) => {
    if (item.section !== section) {
      section = item.section
      list.push({ header: section, key: 'h-' + section })
    }
    list.push({
      item,
      itemIndex,
      key: item.kind + ':' + item.label + ':' + item.detail + ':' + itemIndex
    })
  })
  return list
})

watch(() => props.modelValue, (value) => {
  const next = value ?? ''
  if (!input.value || input.value.value === next) {
    text.value = next
    return
  }
  input.value.value = next
  text.value = next
  open.value = false
  nextTick(() => {
    if (!input.value) {
      return
    }
    input.value.selectionStart = input.value.selectionEnd = next.length
    syncScroll()
  })
})

watch(html, () => nextTick(syncScroll))
watch(() => [props.driver, props.schema, props.schemas, props.tables], () => {
  if (open.value) {
    refreshSuggest(false)
  }
}, { deep: true })
watch(active, () => {
  nextTick(() => menu.value?.querySelector('.active')?.scrollIntoView({ block: 'nearest' }))
})

function syncScroll() {
  if (!backdrop.value || !input.value) {
    return
  }
  backdrop.value.scrollTop = input.value.scrollTop
  backdrop.value.scrollLeft = input.value.scrollLeft
  if (gutter.value) {
    gutter.value.scrollTop = input.value.scrollTop
  }
}

function refreshSuggest(force) {
  if (!input.value) {
    return
  }
  const cursor = input.value.selectionStart ?? 0
  const result = suggestSQL({
    sql: input.value.value,
    cursor,
    driver: props.driver,
    tables: props.tables,
    schemas: props.schemas,
    schema: props.schema
  })
  const prefixChanged = result.prefix !== suggestion.value.prefix || result.from !== suggestion.value.from
  suggestion.value = result
  items.value = result.items
  const show = result.items.length > 0 && (force || result.autoOpen || result.prefix.length > 0)
  open.value = show
  if (!show) {
    return
  }
  active.value = force || prefixChanged ? 0 : Math.min(active.value, result.items.length - 1)
  nextTick(placeMenu)
}

function onInput() {
  text.value = input.value.value
  emit('update:modelValue', text.value)
  syncScroll()
  refreshSuggest(false)
}

function onKeyup(event) {
  if (held) {
    held = false
    return
  }
  if (open.value && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
    return
  }
  refreshSuggest(false)
}

function onKeydown(event) {
  if (event.isComposing || event.key === 'Process') {
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    open.value = false
    emit('run')
    return
  }
  if (event.ctrlKey && event.key === ' ' && !event.metaKey && !event.altKey) {
    event.preventDefault()
    active.value = 0
    refreshSuggest(true)
    return
  }
  if (open.value && items.value.length) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      active.value = (active.value + 1) % items.value.length
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      active.value = (active.value - 1 + items.value.length) % items.value.length
      return
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      accept(items.value[active.value])
      return
    }
    if (event.key === 'Tab') {
      event.preventDefault()
      accept(items.value[active.value])
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      open.value = false
      return
    }
  }
  if (event.key === 'Tab') {
    event.preventDefault()
    insertAtCaret('  ')
  }
}

function insertAtCaret(value) {
  const area = input.value
  const from = area.selectionStart ?? 0
  const to = area.selectionEnd ?? from
  const next = area.value.slice(0, from) + value + area.value.slice(to)
  const caret = from + value.length
  area.value = next
  text.value = next
  emit('update:modelValue', next)
  area.selectionStart = area.selectionEnd = caret
  syncScroll()
}

function accept(item) {
  if (!item || !input.value) {
    return
  }
  const area = input.value
  const from = suggestion.value.from ?? 0
  const to = suggestion.value.to ?? from
  const next = area.value.slice(0, from) + item.insert + area.value.slice(to)
  const caret = from + item.insert.length
  area.value = next
  text.value = next
  emit('update:modelValue', next)
  open.value = false
  held = true
  nextTick(() => {
    area.focus()
    area.selectionStart = area.selectionEnd = caret
    syncScroll()
  })
}

function placeMenu() {
  const area = input.value
  const box = root.value
  if (!area || !box || !open.value) {
    return
  }
  const coords = caretOffset(area)
  const areaRect = area.getBoundingClientRect()
  const menuHeight = 280
  const menuWidth = 320
  let top = areaRect.top + coords.top + coords.line + 4
  let left = areaRect.left + coords.left
  if (top + menuHeight > window.innerHeight - 8) {
    top = Math.max(8, areaRect.top + coords.top - menuHeight - 4)
  }
  if (left + menuWidth > window.innerWidth - 8) {
    left = Math.max(8, window.innerWidth - menuWidth - 8)
  }
  menuStyle.value = { top: top + 'px', left: left + 'px' }
}

function caretOffset(area) {
  const position = area.selectionStart ?? 0
  const style = getComputedStyle(area)
  const mirror = document.createElement('div')
  mirror.style.position = 'absolute'
  mirror.style.visibility = 'hidden'
  mirror.style.whiteSpace = 'pre'
  mirror.style.overflow = 'hidden'
  mirror.style.fontFamily = style.fontFamily
  mirror.style.fontSize = style.fontSize
  mirror.style.fontWeight = style.fontWeight
  mirror.style.lineHeight = style.lineHeight
  mirror.style.letterSpacing = style.letterSpacing
  mirror.style.padding = style.padding
  mirror.style.border = style.border
  mirror.style.tabSize = style.tabSize
  mirror.style.width = area.clientWidth + 'px'
  mirror.style.left = '-9999px'
  mirror.style.top = '0'
  const before = area.value.slice(0, position)
  mirror.textContent = before.endsWith('\n') ? before + '\u200b' : before
  const marker = document.createElement('span')
  marker.textContent = '\u200b'
  mirror.appendChild(marker)
  document.body.appendChild(mirror)
  const top = marker.offsetTop - area.scrollTop
  const left = marker.offsetLeft - area.scrollLeft
  const line = marker.offsetHeight || Number.parseFloat(style.lineHeight) || 20
  mirror.remove()
  return { top, left, line }
}

function onBlur() {
  setTimeout(() => {
    if (menu.value?.contains(document.activeElement)) {
      return
    }
    open.value = false
  }, 140)
}

function onViewport() {
  if (open.value) {
    placeMenu()
  }
}

onMounted(() => {
  if (input.value) {
    input.value.value = props.modelValue || ''
  }
  text.value = props.modelValue || ''
  window.addEventListener('scroll', onViewport, true)
  window.addEventListener('resize', onViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onViewport, true)
  window.removeEventListener('resize', onViewport)
})
</script>

<template>
  <div ref="root" class="sql-editor" :class="{ 'sql-editor-fill': fill }" translate="no">
    <div ref="gutter" class="sql-gutter" aria-hidden="true">{{ lineNumbers }}</div>
    <div class="sql-surface">
      <pre ref="backdrop" class="sql-backdrop" aria-hidden="true" v-html="html"></pre>
      <textarea
        ref="input"
        class="sql-input"
        spellcheck="false"
        autocomplete="off"
        autocapitalize="off"
        autocorrect="off"
        wrap="off"
        aria-label="SQL"
        aria-autocomplete="list"
        :aria-expanded="open"
        @input="onInput"
        @keydown="onKeydown"
        @keyup="onKeyup"
        @click="refreshSuggest(false)"
        @scroll="syncScroll"
        @blur="onBlur"
      ></textarea>
      <div v-if="!text" class="sql-placeholder">输入 SQL，Ctrl+Space 提示</div>
    </div>
    <Teleport to="body">
    <ul v-if="open && items.length" ref="menu" class="sql-suggest" role="listbox" :style="menuStyle">
      <template v-for="entry in rows" :key="entry.key">
        <li v-if="entry.header" class="sql-suggest-section">{{ entry.header }}</li>
        <li
          v-else
          role="option"
          :aria-selected="entry.itemIndex === active"
          :class="{ active: entry.itemIndex === active }"
          @mousedown.prevent="accept(entry.item)"
          @mouseenter="active = entry.itemIndex"
        >
          <span class="sql-kind" :data-kind="entry.item.kind">{{ kindLabel[entry.item.kind] || '提示' }}</span>
          <span class="sql-label">{{ entry.item.label }}</span>
          <span v-if="entry.item.detail" class="sql-detail">{{ entry.item.detail }}</span>
        </li>
      </template>
      <li class="sql-suggest-foot">Tab 或 Enter 填入，Esc 关闭</li>
    </ul>
    </Teleport>
  </div>
</template>
