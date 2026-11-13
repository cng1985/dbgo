<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { normalizeNewlines } from '../sqlError.js'
import { highlightSQL, suggestSQL } from '../sqlEditor.js'
import { readSuggestEnabled } from '../suggestPref.js'

const props = defineProps({
  modelValue: { type: String, default: '' },
  driver: { type: String, default: '' },
  schema: { type: String, default: '' },
  schemas: { type: Array, default: () => [] },
  tables: { type: Array, default: () => [] },
  fill: { type: Boolean, default: false },
  errorLine: { type: Number, default: 0 }
})
const emit = defineEmits(['update:modelValue', 'run', 'menu'])

const root = ref(null)
const input = ref(null)
const backdrop = ref(null)
const gutter = ref(null)
const menu = ref(null)
const text = ref(normalizeNewlines(props.modelValue))
const items = ref([])
const suggestion = ref({ from: 0, to: 0, prefix: '', mode: 'any', autoOpen: false })
const open = ref(false)
const active = ref(0)
const menuStyle = ref({})
const kindLabel = { keyword: '关键字', table: '表', column: '字段', schema: '模式' }
const suggestOn = ref(readSuggestEnabled())
let held = false

const html = computed(() => highlightSQL(text.value, props.driver, props.tables))
const lineCount = computed(() => Math.max(1, text.value.split('\n').length))
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
  const next = normalizeNewlines(value)
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
watch(() => props.errorLine, (line) => nextTick(() => revealLine(line)))
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

function onSuggestPref(event) {
  suggestOn.value = !!event.detail
  if (!suggestOn.value) {
    open.value = false
    items.value = []
  }
}

function refreshSuggest(force) {
  if (!suggestOn.value) {
    open.value = false
    items.value = []
    return
  }
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
  const next = normalizeNewlines(input.value.value)
  if (input.value.value !== next) {
    const caret = input.value.selectionStart
    input.value.value = next
    input.value.selectionStart = input.value.selectionEnd = caret
  }
  text.value = next
  emit('update:modelValue', next)
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

function selection() {
  const area = input.value
  if (!area) {
    return { text: '', start: 0, end: 0 }
  }
  const start = area.selectionStart ?? 0
  const end = area.selectionEnd ?? start
  const from = Math.min(start, end)
  const to = Math.max(start, end)
  return { text: area.value.slice(from, to), start: from, end: to }
}

function replaceRange(start, end, value) {
  const area = input.value
  if (!area) {
    return
  }
  const next = area.value.slice(0, start) + value + area.value.slice(end)
  area.value = next
  text.value = next
  emit('update:modelValue', next)
  const caret = start + String(value).length
  area.focus()
  area.selectionStart = area.selectionEnd = caret
  syncScroll()
}

function revealLine(line) {
  const area = input.value
  if (!area || !line || line < 1) {
    syncScroll()
    return
  }
  const lines = area.value.split('\n')
  const index = Math.min(lines.length, line) - 1
  let start = 0
  for (let i = 0; i < index; i++) {
    start += lines[i].length + 1
  }
  area.focus()
  area.selectionStart = start
  area.selectionEnd = start + (lines[index] || '').length
  const style = getComputedStyle(area)
  const lineHeight = Number.parseFloat(style.lineHeight) || 20
  const pad = Number.parseFloat(style.paddingTop) || 0
  const target = pad + index * lineHeight
  const view = area.clientHeight || 0
  if (target < area.scrollTop || target > area.scrollTop + view - lineHeight) {
    area.scrollTop = Math.max(0, target - view / 3)
  }
  syncScroll()
}

function onContextMenu(event) {
  event.preventDefault()
  open.value = false
  emit('menu', { x: event.clientX, y: event.clientY, ...selection() })
}

defineExpose({ selection, replaceRange, insertAtCaret, revealLine })

onMounted(() => {
  const next = normalizeNewlines(props.modelValue)
  if (input.value) {
    input.value.value = next
  }
  text.value = next
  window.addEventListener('scroll', onViewport, true)
  window.addEventListener('resize', onViewport)
  window.addEventListener('dbgo-suggest', onSuggestPref)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onViewport, true)
  window.removeEventListener('resize', onViewport)
  window.removeEventListener('dbgo-suggest', onSuggestPref)
})
</script>

<template>
  <div ref="root" class="sql-editor" :class="{ 'sql-editor-fill': fill }" translate="no">
    <div ref="gutter" class="sql-gutter" aria-hidden="true">
      <span v-for="n in lineCount" :key="n" :class="{ 'sql-line-bad': n === errorLine }">{{ n }}</span>
    </div>
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
        @contextmenu="onContextMenu"
      ></textarea>
      <div v-if="!text" class="sql-placeholder">{{ suggestOn ? '输入 SQL，Ctrl+Space 提示' : '输入 SQL' }}</div>
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
