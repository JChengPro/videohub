<script setup lang="ts">
import { useToastStore } from '../stores/toast'
const toast = useToastStore()
</script>

<template>
  <div class="toast-host" aria-live="polite" aria-atomic="true">
    <Transition name="toast"><div v-if="toast.message" class="toast" :class="toast.type" :role="toast.type === 'error' ? 'alert' : 'status'">{{ toast.message }}</div></Transition>
  </div>
</template>

<style scoped>
.toast-host { position: fixed; z-index: 200; top: calc(18px + env(safe-area-inset-top)); right: 16px; left: 16px; display: grid; justify-items: center; pointer-events: none; }
.toast { max-width: min(360px, 100%); padding: 11px 16px; border: 1px solid var(--mobile-border-strong); border-radius: 16px; background: var(--mobile-overlay); box-shadow: var(--mobile-shadow); color: var(--mobile-text); font-size: 12px; line-height: 1.45; text-align: center; overflow-wrap: anywhere; pointer-events: auto; }
.toast.error { border-color: rgba(255,83,103,.5); }.toast.success { border-color: rgba(85,213,151,.4); }
.toast-enter-active,.toast-leave-active { transition: opacity .2s ease, transform .2s ease; }
.toast-enter-from,.toast-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
