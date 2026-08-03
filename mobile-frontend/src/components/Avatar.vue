<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

const props = withDefaults(defineProps<{ name: string; id?: number; size?: number; avatarUrl?: string; version?: number }>(), { size: 44 })

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
  let hash = 0
  for (let index = 0; index < input.length; index += 1) {
    hash = (hash * 31 + input.charCodeAt(index)) >>> 0
  }
  return hash
}

const initials = computed(() => props.name.trim().slice(0, 1).toUpperCase() || '?')
const imageFailed = ref(false)
const localVersion = ref(0)
const imageSrc = computed(() => {
  if (props.avatarUrl) return props.avatarUrl
  if (!props.id) return ''
  const version = props.version ?? localVersion.value
  return `/api/account/avatar/${props.id}${version ? `?v=${version}` : ''}`
})
const background = computed(() => {
  const seed = props.name.trim().toLowerCase() || '0'
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
  <span class="avatar" :style="{ width: `${size}px`, height: `${size}px`, backgroundImage: background }">
    <img v-if="imageSrc && !imageFailed" :src="imageSrc" :alt="`${name} 的头像`" @error="imageFailed = true" />
    <span v-else aria-hidden="true">{{ initials }}</span>
  </span>
</template>

<style scoped>
.avatar { position: relative; flex: 0 0 auto; display: grid; place-items: center; overflow: hidden; border: 1px solid rgba(255,255,255,.16); border-radius: 50%; box-shadow: inset 0 1px rgba(255,255,255,.12), 0 5px 14px rgba(0,0,0,.2); color: #fff; font-size: calc(v-bind(size) * .3px); font-weight: 800; letter-spacing: 0; user-select: none; }
.avatar > img { width: 100%; height: 100%; display: block; object-fit: cover; }
.avatar > span { position: relative; z-index: 1; }
.avatar::after { content: ''; position: absolute; inset: 0; border-radius: inherit; box-shadow: inset 0 0 0 1px rgba(255,255,255,.08); pointer-events: none; }
</style>
