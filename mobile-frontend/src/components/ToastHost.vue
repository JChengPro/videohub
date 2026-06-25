<script setup lang="ts">
import { useToastStore } from '../stores/toast'
const toast = useToastStore()
</script>

<template>
  <Transition name="toast"><div v-if="toast.message" class="toast" :class="toast.type">{{ toast.message }}</div></Transition>
</template>

<style scoped>
.toast { position: fixed; z-index: 200; top: calc(24px + env(safe-area-inset-top)); left: 50%; max-width: calc(100vw - 40px); padding: 11px 16px; transform: translateX(-50%); border: 1px solid rgba(255,255,255,.15); border-radius: 99px; background: rgba(35,35,40,.96); box-shadow: 0 12px 40px rgba(0,0,0,.35); color: #fff; font-size: 13px; white-space: nowrap; }
.toast.error { border-color: rgba(254,44,85,.5); }.toast.success { border-color: rgba(36,215,232,.45); }
.toast-enter-active,.toast-leave-active { transition: all .2s ease; }.toast-enter-from,.toast-leave-to { opacity: 0; transform: translate(-50%,-8px); }
</style>
