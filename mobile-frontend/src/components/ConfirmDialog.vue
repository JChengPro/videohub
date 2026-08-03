<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'
import { useDialogStore } from '../stores/dialog'

const dialog = useDialogStore()
const cancelButton = ref<HTMLButtonElement | null>(null)
let previousFocus: HTMLElement | null = null
let previousBodyOverflow = ''

function onKeydown(event: KeyboardEvent) {
  if (dialog.open && event.key === 'Escape') {
    event.preventDefault()
    dialog.cancel()
  }
}

watch(() => dialog.open, async (open) => {
  if (open) {
    previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    previousBodyOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    window.addEventListener('keydown', onKeydown)
    await nextTick()
    cancelButton.value?.focus()
  } else {
    document.body.style.overflow = previousBodyOverflow
    window.removeEventListener('keydown', onKeydown)
    previousFocus?.focus()
    previousFocus = null
  }
})

onBeforeUnmount(() => {
  document.body.style.overflow = previousBodyOverflow
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="mobile-dialog">
      <div v-if="dialog.open" class="dialog-backdrop" @click.self="dialog.cancel">
        <section role="alertdialog" aria-modal="true" aria-labelledby="mobile-dialog-title" aria-describedby="mobile-dialog-message">
          <div class="dialog-icon" :class="{ danger: dialog.tone === 'danger' }">
            <AppIcon :name="dialog.tone === 'danger' ? 'warning' : 'check'" :size="25" />
          </div>
          <small>{{ dialog.tone === 'danger' ? '请确认此操作' : '操作确认' }}</small>
          <h2 id="mobile-dialog-title">{{ dialog.title }}</h2>
          <p id="mobile-dialog-message">{{ dialog.message }}</p>
          <div class="dialog-actions">
            <button ref="cancelButton" type="button" @click="dialog.cancel">{{ dialog.cancelLabel }}</button>
            <button type="button" class="confirm" @click="dialog.accept">{{ dialog.confirmLabel }}</button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-backdrop { position: fixed; z-index: 10000; inset: 0; padding: 20px; display: grid; place-items: center; background: rgba(0,0,0,.68); backdrop-filter: blur(10px); }
section { width: min(340px,100%); padding: 22px 20px 18px; border: 1px solid var(--mobile-border); border-radius: 12px; background: #19191c; box-shadow: var(--mobile-shadow); text-align: center; }
.dialog-icon { width: 44px; height: 44px; margin: 0 auto 12px; display: grid; place-items: center; border-radius: 8px; background: rgba(37,244,238,.1); color: var(--mobile-cyan); }
.dialog-icon.danger { background: var(--mobile-accent-dim); color: var(--mobile-accent); }
small { color: var(--mobile-accent); font-size: 8px; font-weight: 900; letter-spacing: .16em; text-transform: uppercase; }
h2 { margin-top: 5px; font-size: 20px; letter-spacing: -.03em; }
p { margin: 8px auto 18px; max-width: 290px; color: var(--mobile-text-secondary); font-size: 12px; line-height: 1.65; }
.dialog-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 9px; }
.dialog-actions button { min-height: 44px; border: 1px solid var(--mobile-border); border-radius: 7px; background: var(--mobile-surface-raised); color: var(--mobile-text-secondary); font-weight: 800; }
.dialog-actions .confirm { border-color: transparent; background: var(--mobile-accent); color: #fff; }
.mobile-dialog-enter-active,.mobile-dialog-leave-active { transition: opacity 180ms ease; }
.mobile-dialog-enter-active section,.mobile-dialog-leave-active section { transition: transform 220ms ease,opacity 180ms ease; }
.mobile-dialog-enter-from,.mobile-dialog-leave-to { opacity: 0; }
.mobile-dialog-enter-from section,.mobile-dialog-leave-to section { opacity: 0; transform: translateY(12px) scale(.97); }
</style>
