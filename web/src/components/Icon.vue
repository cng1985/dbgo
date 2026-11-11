<script setup>
defineProps({
  name: { type: String, default: '' }
})
</script>

<template>
  <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
    <template v-if="name === 'chevron'">
      <path d="M6 3.5 10.5 8 6 12.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" />
    </template>
    <template v-else-if="name === 'sql'">
      <path d="M3 2.5h7.2L13 5.2V13.5H3z" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M10 2.6V5.4H13" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M5.2 8.2h5.6M5.2 10.6h3.8" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'database'">
      <ellipse cx="8" cy="3.6" rx="5.2" ry="1.8" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M2.8 3.6v8.6c0 1 2.3 1.8 5.2 1.8s5.2-.8 5.2-1.8V3.6" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M2.8 8c0 1 2.3 1.8 5.2 1.8s5.2-.8 5.2-1.8" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'add'">
      <path d="M8 3.2v9.6M3.2 8h9.6" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'refresh'">
      <path d="M13 8a5 5 0 1 1-1.4-3.4" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
      <path d="M13 2.8V5.6H10.2" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" />
    </template>
    <template v-else-if="name === 'play'">
      <path d="M4.2 2.6v10.8L13 8z" fill="currentColor" />
    </template>
    <template v-else-if="name === 'overview'">
      <rect x="2.2" y="2.2" width="5" height="5" fill="none" stroke="currentColor" stroke-width="1.3" />
      <rect x="8.8" y="2.2" width="5" height="5" fill="none" stroke="currentColor" stroke-width="1.3" />
      <rect x="2.2" y="8.8" width="5" height="5" fill="none" stroke="currentColor" stroke-width="1.3" />
      <rect x="8.8" y="8.8" width="5" height="5" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'migration'">
      <path d="M3 11.5h10M3 4.5h10" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
      <path d="M10.2 2.2 13 4.5 10.2 6.8M5.8 9.2 3 11.5l2.8 2.3" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" />
    </template>
    <template v-else-if="name === 'backup'">
      <path d="M3 6.5h10v6.2H3z" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M5.2 6.5V4.2h5.6v2.3M8 8.2v3" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'catalog'">
      <path d="M3 2.6h4.2v10.8H3zM8.2 4.4H13v9H8.2z" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'nav'">
      <path d="M2.4 3.2h11.2M2.4 8h11.2M2.4 12.8h7" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'folder'">
      <path d="M2 4.2h4l1.2 1.4H14V13H2z" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round" />
    </template>
    <template v-else-if="name === 'schema'">
      <rect x="2.2" y="3" width="11.6" height="10" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M2.2 6.2h11.6M6 6.2V13" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'table'">
      <rect x="2" y="2.4" width="12" height="11.2" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M2 6h12M2 9.4h12M6 6v7.6" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'view'">
      <path d="M1.6 8s2.4-3.6 6.4-3.6S14.4 8 14.4 8s-2.4 3.6-6.4 3.6S1.6 8 1.6 8z" fill="none" stroke="currentColor" stroke-width="1.3" />
      <circle cx="8" cy="8" r="1.6" fill="none" stroke="currentColor" stroke-width="1.3" />
    </template>
    <template v-else-if="name === 'column'">
      <path d="M8 2.2v11.6M5.2 4.4h5.6M5.2 8h5.6M5.2 11.6h5.6" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'key'">
      <circle cx="6" cy="8" r="2.4" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M8.2 8H14M12 8v2.2M10.2 8v1.6" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'index'">
      <path d="M3.2 3.2h3.2v3.2H3.2zM9.6 3.2h3.2v3.2H9.6zM3.2 9.6h3.2v3.2H3.2z" fill="none" stroke="currentColor" stroke-width="1.2" />
      <path d="M9.6 11.2h3.2" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'search'">
      <circle cx="7" cy="7" r="3.4" fill="none" stroke="currentColor" stroke-width="1.3" />
      <path d="M9.6 9.6 13 13" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
    <template v-else-if="name === 'etl'">
      <rect x="1.5" y="3.2" width="4.6" height="4.2" fill="none" stroke="currentColor" stroke-width="1.2" />
      <path d="M6.4 5.2h2.2M7.4 3.8 8.8 5.2 7.4 6.6" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" />
      <rect x="9.9" y="8.6" width="4.6" height="4.2" fill="none" stroke="currentColor" stroke-width="1.2" />
    </template>
    <template v-else-if="name === 'close'">
      <path d="M4 4l8 8M12 4 4 12" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" />
    </template>
  </svg>
</template>
