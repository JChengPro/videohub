<script setup lang="ts">
import { useToastStore } from '../stores/toast'

const toast = useToastStore()
</script>

<template>
  <div class="toast-wrap" aria-live="polite" aria-atomic="true" aria-relevant="additions removals">
    <TransitionGroup name="toast-list">
      <div v-for="t in toast.toasts" :key="t.id" class="toast" :class="t.type" :role="t.type === 'error' ? 'alert' : 'status'">
        <span class="toast-dot" aria-hidden="true" />
        <div class="toast-msg">{{ t.message }}</div>
        <button class="toast-x" type="button" aria-label="关闭提示" @click="toast.remove(t.id)">×</button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-wrap {
  position: fixed;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  display: grid;
  gap: 10px;
  z-index: 200;
  width: min(520px, calc(100vw - 24px));
  pointer-events: none;
}

.toast {
  pointer-events: auto;
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 10px;
  align-items: center;
  border-radius: var(--radius);
  padding: 11px 12px;
  backdrop-filter: blur(12px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: var(--surface-overlay);
  box-shadow: var(--shadow-sm);
}

.toast-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--text-muted);
}

.toast.success {
  border-color: rgba(34, 197, 94, 0.35);
}

.toast.success .toast-dot {
  background: var(--ok);
}

.toast.error {
  border-color: rgba(254, 44, 85, 0.45);
}

.toast.error .toast-dot {
  background: var(--danger);
}

.toast.info {
  border-color: rgba(255, 255, 255, 0.18);
}

.toast-list-enter-active,
.toast-list-leave-active {
  transition: opacity var(--duration) ease, transform var(--duration) var(--ease-out);
}

.toast-list-enter-from,
.toast-list-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

.toast-msg {
  font-size: 13px;
  line-height: 1.35;
  color: rgba(255, 255, 255, 0.92);
}

.toast-x {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.88);
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  padding: 0;
}
</style>
