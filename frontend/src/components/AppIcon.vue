<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: number; filled?: boolean }>(), {
  size: 24,
  filled: false,
})
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    :fill="filled && (name === 'heart' || name === 'hot') ? 'currentColor' : 'none'"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <template v-if="name === 'brand'">
      <rect class="brand-face" x="2.5" y="2.5" width="19" height="19" rx="5" />
      <path class="brand-cyan" d="M4.6 8v8" />
      <path class="brand-red" d="M19.4 8v8" />
      <path class="brand-letter" d="m6.6 7.8 2.7 8.4 2.7-8.4M13.7 7.8v8.4M17.4 7.8v8.4M13.7 12h3.7" />
    </template>
    <template v-else-if="name === 'home'"><path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z" /></template>
    <template v-else-if="name === 'following'"><circle cx="9" cy="8" r="4" /><path d="M2 21a7 7 0 0 1 14 0M19 8v6m3-3h-6" /></template>
    <template v-else-if="name === 'hot'"><path d="M12 22c4.4 0 7-3.1 7-7.3 0-3.1-1.7-6-5-8.7.2 2.8-1.4 4.2-3 4.8.2-3.6-1.8-6.2-3.2-7.8.2 4.4-2.8 6.1-2.8 11.7C5 18.9 7.7 22 12 22Z" /></template>
    <template v-else-if="name === 'message'"><path d="M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4Z" /><path d="M8 9h8M8 13h5" /></template>
    <template v-else-if="name === 'upload'"><rect x="3" y="3" width="18" height="18" rx="5" /><path d="M12 7v10M7 12h10" /></template>
    <template v-else-if="name === 'user'"><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></template>
    <template v-else-if="name === 'search'"><circle cx="11" cy="11" r="7" /><path d="m20 20-4-4" /></template>
    <template v-else-if="name === 'settings'"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3A1.7 1.7 0 0 0 14 21v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V2.8h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z" /></template>
    <template v-else-if="name === 'heart'"><path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8l1.1 1.1L12 21l7.8-7.5 1.1-1.1a5.5 5.5 0 0 0-.1-7.8Z" /></template>
    <template v-else-if="name === 'comment'"><path d="M21 12a8 8 0 0 1-8 8H6l-3 2v-7a9 9 0 1 1 18-3Z" /></template>
    <template v-else-if="name === 'share'"><circle cx="18" cy="5" r="3" /><circle cx="6" cy="12" r="3" /><circle cx="18" cy="19" r="3" /><path d="m8.6 10.5 6.8-4M8.6 13.5l6.8 4" /></template>
    <template v-else-if="name === 'play'"><path d="m8 5 11 7-11 7Z" /></template>
    <template v-else-if="name === 'back'"><path d="m15 18-6-6 6-6" /></template>
    <template v-else-if="name === 'send'"><path d="m22 2-7 20-4-9-9-4Z" /><path d="M22 2 11 13" /></template>
    <template v-else-if="name === 'close'"><path d="m6 6 12 12M18 6 6 18" /></template>
    <template v-else-if="name === 'trash'"><path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6" /></template>
    <template v-else-if="name === 'check'"><path d="m5 12 4 4L19 6" /></template>
    <template v-else-if="name === 'refresh'"><path d="M20 7v5h-5M4 17v-5h5" /><path d="M18.5 9A7 7 0 0 0 6.2 6.2L4 8m2 7a7 7 0 0 0 11.8 2.8L20 16" /></template>
    <template v-else-if="name === 'more'"><circle cx="5" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="19" cy="12" r="1" fill="currentColor" stroke="none" /></template>
  </svg>
</template>

<style scoped>
svg { flex: 0 0 auto; display: block; }
.brand-face { fill: #f5f5f7; stroke: rgba(255, 255, 255, .45); stroke-width: .7; }
.brand-letter { fill: none; stroke: #101012; stroke-width: 1.55; }
.brand-cyan { stroke: #25f4ee; stroke-width: 1.2; }
.brand-red { stroke: #fe2c55; stroke-width: 1.2; }
</style>
