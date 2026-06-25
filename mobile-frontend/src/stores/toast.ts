import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useToastStore = defineStore('toast', () => {
  const message = ref('')
  const type = ref<'success' | 'error' | 'info'>('info')
  let timer = 0

  function show(value: string, nextType: typeof type.value = 'info') {
    message.value = value
    type.value = nextType
    window.clearTimeout(timer)
    timer = window.setTimeout(() => (message.value = ''), 2600)
  }

  return {
    message,
    type,
    success: (value: string) => show(value, 'success'),
    error: (value: string) => show(value, 'error'),
    info: (value: string) => show(value, 'info'),
  }
})
