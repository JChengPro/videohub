import { defineStore } from 'pinia'
import { ref } from 'vue'

type DialogTone = 'default' | 'danger'
type DialogOptions = {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: DialogTone
}

export const useDialogStore = defineStore('dialog', () => {
  const open = ref(false)
  const title = ref('')
  const message = ref('')
  const confirmLabel = ref('确认')
  const cancelLabel = ref('取消')
  const tone = ref<DialogTone>('default')
  let resolver: ((result: boolean) => void) | null = null

  function ask(options: DialogOptions) {
    resolver?.(false)
    title.value = options.title
    message.value = options.message
    confirmLabel.value = options.confirmLabel ?? '确认'
    cancelLabel.value = options.cancelLabel ?? '取消'
    tone.value = options.tone ?? 'default'
    open.value = true
    return new Promise<boolean>((resolve) => { resolver = resolve })
  }

  function settle(result: boolean) {
    if (!open.value) return
    open.value = false
    resolver?.(result)
    resolver = null
  }

  return { open, title, message, confirmLabel, cancelLabel, tone, ask, accept: () => settle(true), cancel: () => settle(false) }
})
