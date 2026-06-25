import { postJson } from './client'
import type { MessageResponse } from './types'

export type NotificationType = 'like' | 'comment' | 'follow'

export type Notification = {
  id: number
  receiver_id: number
  actor_id: number
  actor_username: string
  type: NotificationType
  target_type: 'video' | 'account'
  target_id: number
  content: string
  is_read: boolean
  create_time: string
  read_time?: string | null
}

export type NotificationListResponse = {
  notifications: Notification[]
  has_more: boolean
  next_before_id: number
}

export function list(input: { type?: NotificationType; limit?: number; before_id?: number }) {
  return postJson<NotificationListResponse>('/notification/list', input, { authRequired: true }).then((res) => ({
    ...res,
    notifications: Array.isArray(res.notifications) ? res.notifications : [],
  }))
}

export function unreadCount() {
  return postJson<{ count: number }>('/notification/unreadCount', {}, { authRequired: true })
}

export function markRead(id: number) {
  return postJson<MessageResponse>('/notification/markRead', { id }, { authRequired: true })
}

export function markAllRead() {
  return postJson<MessageResponse>('/notification/markAllRead', {}, { authRequired: true })
}
