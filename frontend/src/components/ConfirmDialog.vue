<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useDialogStore } from '../stores/dialog'

const dialog = useDialogStore()
const panel = ref<HTMLElement | null>(null)
const cancelButton = ref<HTMLButtonElement | null>(null)
let previousFocus: HTMLElement | null = null
let previousBodyOverflow = ''

function focusableElements() {
  return Array.from(panel.value?.querySelectorAll<HTMLElement>('button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])') ?? [])
}

function onKeydown(event: KeyboardEvent) {
  if (!dialog.open) return
  if (event.key === 'Escape') {
    event.preventDefault()
    dialog.cancel()
    return
  }
  if (event.key !== 'Tab') return
  const elements = focusableElements()
  if (!elements.length) return
  const first = elements[0]
  const last = elements[elements.length - 1]
  if (!first || !last) return
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
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
    <Transition name="dialog">
      <div v-if="dialog.open" class="dialog-backdrop" @mousedown.self="dialog.cancel">
        <section
          ref="panel"
          class="dialog-panel"
          :class="{ danger: dialog.tone === 'danger' }"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="confirm-dialog-title"
          aria-describedby="confirm-dialog-message"
        >
          <div class="dialog-mark" aria-hidden="true">
            <svg v-if="dialog.tone === 'danger'" viewBox="0 0 24 24" fill="none"><path d="M12 8v5"/><path d="M12 17h.01"/><path d="M10.3 3.6 2.2 18a2 2 0 0 0 1.8 3h16a2 2 0 0 0 1.8-3L13.7 3.6a2 2 0 0 0-3.4 0Z"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="none"><path d="m7 12 3 3 7-7"/><circle cx="12" cy="12" r="9"/></svg>
          </div>
          <div class="dialog-copy">
            <p class="dialog-kicker">{{ dialog.tone === 'danger' ? '请确认此操作' : '操作确认' }}</p>
            <h2 id="confirm-dialog-title">{{ dialog.title }}</h2>
            <p id="confirm-dialog-message">{{ dialog.message }}</p>
          </div>
          <div class="dialog-actions">
            <button ref="cancelButton" type="button" class="dialog-cancel" @click="dialog.cancel">{{ dialog.cancelLabel }}</button>
            <button type="button" class="dialog-confirm" @click="dialog.accept">{{ dialog.confirmLabel }}</button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.dialog-backdrop {
  position: fixed;
  z-index: 10000;
  inset: 0;
  padding: 20px;
  display: grid;
  place-items: center;
  background: rgba(0, 0, 0, .68);
  backdrop-filter: blur(10px);
}

.dialog-panel {
  width: min(400px, 100%);
  padding: 22px;
  display: grid;
  grid-template-columns: 46px 1fr;
  gap: 14px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: #19191c;
  box-shadow: 0 30px 90px rgba(0, 0, 0, .62);
}

.dialog-panel.danger { border-color: rgba(254, 44, 85, .26); }
.dialog-mark { width: 46px; height: 46px; display: grid; place-items: center; border-radius: 8px; background: rgba(37, 244, 238, .1); color: var(--accent-cyan); }
.danger .dialog-mark { background: var(--accent-dim); color: var(--accent); }
.dialog-mark svg { width: 25px; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.dialog-kicker { margin-bottom: 5px; color: var(--accent-cyan); font-size: 10px; font-weight: 800; letter-spacing: .14em; }
.danger .dialog-kicker { color: var(--accent); }
.dialog-copy h2 { font-size: 20px; line-height: 1.25; letter-spacing: -.02em; }
.dialog-copy > p:last-child { margin-top: 8px; color: var(--text-secondary); font-size: 13px; line-height: 1.65; }
.dialog-actions { grid-column: 1 / -1; margin-top: 8px; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.dialog-actions button { min-height: 44px; font-weight: 700; }
.dialog-cancel { border: 1px solid var(--border); background: var(--surface-raised); color: var(--text-secondary); }
.dialog-confirm { background: var(--accent); color: #fff; }
.dialog-confirm:hover { background: var(--accent-hover); }
.dialog-enter-active, .dialog-leave-active { transition: opacity 180ms ease; }
.dialog-enter-active .dialog-panel, .dialog-leave-active .dialog-panel { transition: transform 220ms var(--ease-out), opacity 180ms ease; }
.dialog-enter-from, .dialog-leave-to { opacity: 0; }
.dialog-enter-from .dialog-panel, .dialog-leave-to .dialog-panel { opacity: 0; transform: translateY(12px) scale(.97); }
</style>
