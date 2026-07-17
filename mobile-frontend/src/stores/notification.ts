import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api } from '../api'
import { useAuthStore } from './auth'

export const useNotificationStore = defineStore('notification', () => {
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
      const response = await api.unread()
      if (current === request) unread.value = Math.max(0, response.count)
    } catch {
      // Keep the last known count when a transient request fails.
    }
  }

  function readOne() {
    unread.value = Math.max(0, unread.value - 1)
  }

  function readAll() {
    unread.value = 0
  }

  function clear() {
    request += 1
    unread.value = 0
  }

  return { unread, refresh, readOne, readAll, clear }
})
