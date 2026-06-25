import { defineStore } from 'pinia'
import { ref } from 'vue'

import * as notificationApi from '../api/notification'

export const useNotificationStore = defineStore('notification', () => {
  const unread = ref(0)

  async function refreshUnread() {
    try {
      const res = await notificationApi.unreadCount()
      unread.value = Math.max(0, res.count)
    } catch {
      unread.value = 0
    }
  }

  function readOne() {
    unread.value = Math.max(0, unread.value - 1)
  }

  function readAll() {
    unread.value = 0
  }

  function clear() {
    unread.value = 0
  }

  return { unread, refreshUnread, readOne, readAll, clear }
})
