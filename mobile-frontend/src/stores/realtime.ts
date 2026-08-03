import { defineStore } from 'pinia'
import { ref } from 'vue'

import { api } from '../api'
import { useAuthStore } from './auth'
import { useChatStore } from './chat'
import { useNotificationStore } from './notification'

export type RealtimeEvent<T = unknown> = {
  type: string
  request_id?: string
  timestamp: number
  data?: T
}

type Pending = {
  resolve: (value: unknown) => void
  reject: (error: Error) => void
  timer: number
}

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

function wsUrl(ticket: string) {
  const base = new URL(API_BASE, window.location.origin)
  const protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${base.host}${base.pathname.replace(/\/$/, '')}/ws?ticket=${encodeURIComponent(ticket)}`
}

function nextRequestId() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export const useRealtimeStore = defineStore('realtime', () => {
  const auth = useAuthStore()
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const connected = ref(false)
  const connecting = ref(false)
  let socket: WebSocket | null = null
  let reconnectTimer = 0
  let reconnectAttempt = 0
  let generation = 0
  const pending = new Map<string, Pending>()

  async function connect() {
    if (!auth.isLoggedIn || socket || connecting.value) return
    const current = ++generation
    connecting.value = true
    try {
      const { ticket } = await api.wsTicket()
      if (current !== generation || !auth.isLoggedIn) {
        connecting.value = false
        return
      }
      const next = new WebSocket(wsUrl(ticket))
      socket = next
      next.onopen = () => {
        if (socket !== next) return
        connected.value = true
        connecting.value = false
        reconnectAttempt = 0
      }
      next.onmessage = (message) => {
        let event: RealtimeEvent
        try { event = JSON.parse(String(message.data)) as RealtimeEvent }
        catch { return }
        if (event.type === 'ping') {
          next.send(JSON.stringify({ type: 'pong', request_id: event.request_id }))
          return
        }
        if (event.request_id && pending.has(event.request_id)) {
          const request = pending.get(event.request_id)!
          window.clearTimeout(request.timer)
          pending.delete(event.request_id)
          if (event.type === 'error') {
            request.reject(new Error((event.data as { message?: string } | undefined)?.message || '消息发送失败'))
          } else {
            request.resolve(event.data)
          }
        }
        if (event.type === 'chat.new_message' || event.type === 'chat.conversation_changed') void chat.refresh()
        if (event.type === 'notification.new') notifications.increment()
        window.dispatchEvent(new CustomEvent('videohub:realtime', { detail: event }))
      }
      next.onerror = () => next.close()
      next.onclose = () => {
        if (socket === next) socket = null
        connected.value = false
        connecting.value = false
        rejectPending('实时连接已断开')
        scheduleReconnect()
      }
    } catch {
      if (current === generation) {
        connecting.value = false
        scheduleReconnect()
      }
    }
  }

  function scheduleReconnect() {
    if (!auth.isLoggedIn || reconnectTimer) return
    const delay = Math.min(30_000, 1000 * 2 ** reconnectAttempt) + Math.floor(Math.random() * 400)
    reconnectAttempt += 1
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = 0
      void connect()
    }, delay)
  }

  function disconnect() {
    generation += 1
    window.clearTimeout(reconnectTimer)
    reconnectTimer = 0
    reconnectAttempt = 0
    const current = socket
    socket = null
    current?.close()
    connected.value = false
    connecting.value = false
    rejectPending('实时连接已关闭')
  }

  function rejectPending(message: string) {
    for (const request of pending.values()) {
      window.clearTimeout(request.timer)
      request.reject(new Error(message))
    }
    pending.clear()
  }

  function send<T>(type: string, data: unknown): Promise<T> {
    if (!socket || socket.readyState !== WebSocket.OPEN) return Promise.reject(new Error('实时连接尚未建立'))
    const id = nextRequestId()
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        pending.delete(id)
        reject(new Error('实时请求超时'))
      }, 12_000)
      pending.set(id, { resolve: (value) => resolve(value as T), reject, timer })
      socket!.send(JSON.stringify({ type, request_id: id, data }))
    })
  }

  return { connected, connecting, connect, disconnect, send }
})
