<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps<{
  username: string
  id?: number
  size?: number
  avatarUrl?: string
  version?: number
}>()

const avatarPalette = [
  ['#21d4fd', '#4f46e5'],
  ['#ff5c7c', '#ff8a3d'],
  ['#a855f7', '#ec4899'],
  ['#14b8a6', '#22c55e'],
  ['#f59e0b', '#ef4444'],
  ['#6366f1', '#06b6d4'],
  ['#f43f5e', '#8b5cf6'],
  ['#84cc16', '#0ea5e9'],
] as const

function hashString(input: string) {
  let h = 0
  for (let i = 0; i < input.length; i += 1) {
    h = (h * 31 + input.charCodeAt(i)) >>> 0
  }
  return h
}

const initial = computed(() => {
  const s = (props.username ?? '').trim()
  if (!s) return '?'
  return s.slice(0, 1).toUpperCase()
})

const sizePx = computed(() => `${props.size ?? 40}px`)
const imageFailed = ref(false)
const localVersion = ref(0)
const imageSrc = computed(() => {
  if (props.avatarUrl) return props.avatarUrl
  if (!props.id) return ''
  const version = props.version ?? localVersion.value
  return `/api/account/avatar/${props.id}${version ? `?v=${version}` : ''}`
})

const bg = computed(() => {
  const seed = props.username.trim().toLowerCase() || String(props.id ?? 0)
  const [start, end] = avatarPalette[hashString(seed) % avatarPalette.length] ?? ['#21d4fd', '#4f46e5']
  return `linear-gradient(145deg, ${start}, ${end})`
})

watch(imageSrc, () => { imageFailed.value = false })

function onAvatarUpdated(event: Event) {
  const accountID = (event as CustomEvent<{ accountId?: number }>).detail?.accountId
  if (props.id && accountID === props.id) {
    localVersion.value = Date.now()
    imageFailed.value = false
  }
}

onMounted(() => window.addEventListener('videohub:avatar-updated', onAvatarUpdated))
onUnmounted(() => window.removeEventListener('videohub:avatar-updated', onAvatarUpdated))
</script>

<template>
  <div class="avatar" :style="{ width: sizePx, height: sizePx, backgroundImage: bg }">
    <img v-if="imageSrc && !imageFailed" :src="imageSrc" :alt="`${username} 的头像`" @error="imageFailed = true" />
    <span v-else aria-hidden="true">{{ initial }}</span>
  </div>
</template>

<style scoped>
.avatar {
  position: relative;
  display: grid;
  place-items: center;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  box-shadow: inset 0 1px rgba(255, 255, 255, 0.08), 0 6px 16px rgba(0, 0, 0, 0.2);
  color: #f5f5f7;
  font-weight: 800;
  letter-spacing: 0;
  user-select: none;
  overflow: hidden;
}
.avatar img { width: 100%; height: 100%; display: block; object-fit: cover; }
.avatar span { position: relative; z-index: 1; }
.avatar::after { content: ''; position: absolute; inset: 0; border-radius: inherit; box-shadow: inset 0 0 0 1px rgba(255,255,255,.08); pointer-events: none; }
</style>
