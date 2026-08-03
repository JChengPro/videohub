import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import * as messageApi from '../api/message'
import { useAuthStore } from './auth'

export const useChatStore = defineStore('chat', () => {
  const auth = useAuthStore()
  const unread = ref(0)
  const activePeerId = ref(0)
  let request = 0

  const hasUnread = computed(() => unread.value > 0)

  async function refreshUnread() {
    const current = ++request
    if (!auth.isLoggedIn) {
      unread.value = 0
      return
    }
    try {
      const response = await messageApi.unreadCount()
      if (current === request && auth.isLoggedIn) unread.value = Math.max(0, response.count)
    } catch {
      // 保留最后一次成功的未读数，短暂网络错误不清零。
    }
  }

  function setActivePeer(peerId: number) {
    activePeerId.value = peerId
  }

  function clear() {
    request += 1
    unread.value = 0
    activePeerId.value = 0
  }

  return { unread, activePeerId, hasUnread, refreshUnread, setActivePeer, clear }
})
