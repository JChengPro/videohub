import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api } from '../api'
import { useAuthStore } from './auth'

export const useChatStore = defineStore('chat', () => {
  const auth = useAuthStore()
  const unread = ref(0)
  let request = 0

  async function refresh() {
    const current = ++request
    if (!auth.isLoggedIn) {
      unread.value = 0
      return
    }
    try {
      const response = await api.chatUnread()
      if (current === request) unread.value = Math.max(0, response.count)
    } catch {
      // 短暂网络错误时保留最后一次成功的未读数。
    }
  }

  function clear() {
    request += 1
    unread.value = 0
  }

  return { unread, refresh, clear }
})
