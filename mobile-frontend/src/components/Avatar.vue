<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{ name: string; size?: number }>(), { size: 44 })
const initials = computed(() => props.name.trim().slice(0, 2).toUpperCase() || 'VH')
const hue = computed(() => Array.from(props.name).reduce((sum, char) => sum + char.charCodeAt(0), 0) % 360)
</script>

<template>
  <span class="avatar" :style="{ width: `${size}px`, height: `${size}px`, '--hue': hue }">{{ initials }}</span>
</template>

<style scoped>
.avatar { flex: 0 0 auto; display: grid; place-items: center; border: 2px solid rgba(255,255,255,.9); border-radius: 50%; background: linear-gradient(145deg, hsl(var(--hue) 72% 55%), hsl(calc(var(--hue) + 40) 68% 35%)); color: #fff; font-size: calc(v-bind(size) * .26px); font-weight: 800; letter-spacing: -.06em; }
</style>
