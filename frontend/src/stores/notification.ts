import { defineStore } from 'pinia'
import { ref } from 'vue'

import * as notificationApi from '../api/notification'
import { useAuthStore } from './auth'

export const useNotificationStore = defineStore('notification', () => {
  const auth = useAuthStore()
  const unread = ref(0)
  let request = 0

  async function refreshUnread() {
    const current = ++request
    if (!auth.isLoggedIn) {
      unread.value = 0
      return
    }
    try {
      const res = await notificationApi.unreadCount()
      if (current === request && auth.isLoggedIn) unread.value = Math.max(0, res.count)
    } catch {
      if (current === request) unread.value = 0
    }
  }

  function readOne() {
    unread.value = Math.max(0, unread.value - 1)
  }

  function readAll() {
    unread.value = 0
  }

  function increment() {
    unread.value += 1
  }

  function clear() {
    request += 1
    unread.value = 0
  }

  return { unread, refreshUnread, readOne, readAll, increment, clear }
})
