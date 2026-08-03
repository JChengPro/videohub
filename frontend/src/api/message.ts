import { postJson } from './client'

export type ConversationStatus = 'pending' | 'accepted' | 'mutual' | 'rejected' | 'blocked'

export type ChatMessage = {
  id: number
  conversation_id: number
  sender_id: number
  sender_username: string
  receiver_id: number
  client_message_id: string
  message_type: 'text'
  content: string
  created_at: string
}

export type Conversation = {
  id: number
  peer_id: number
  peer_username: string
  status: ConversationStatus
  request_sender_id: number
  request_sent_count: number
  remaining_request_messages: number
  can_send: boolean
  can_reply: boolean
  blocked_by_me: boolean
  blocked_by_peer: boolean
  last_message_id: number
  last_message_content: string
  last_message_sender_id: number
  last_message_at?: string
  unread_count: number
  updated_at: string
}

export type SendMessageResponse = {
  message: ChatMessage
  conversation_status: ConversationStatus
  remaining_request_messages: number
  idempotent: boolean
}

export function listConversations() {
  return postJson<{ conversations: Conversation[] }>('/message/listConversations', {}, { authRequired: true })
    .then((response) => ({ conversations: response.conversations ?? [] }))
}

export function listMessages(conversationId: number, beforeId = 0, limit = 30) {
  return postJson<{ messages: ChatMessage[]; has_more: boolean; next_before_id: number }>(
    '/message/listMessages',
    { conversation_id: conversationId, before_id: beforeId, limit },
    { authRequired: true },
  ).then((response) => ({ ...response, messages: response.messages ?? [] }))
}

export function sendMessage(receiverId: number, clientMessageId: string, content: string) {
  return postJson<SendMessageResponse>(
    '/message/send',
    { receiver_id: receiverId, client_message_id: clientMessageId, content },
    { authRequired: true },
  )
}

export function markRead(conversationId: number, messageId = 0) {
  return postJson<{ conversation_id: number; read_message_id: number; peer_id: number; unread_count: number }>(
    '/message/markRead',
    { conversation_id: conversationId, message_id: messageId },
    { authRequired: true },
  )
}

export function accept(conversationId: number) {
  return postJson<{ message: string; status: ConversationStatus }>(
    '/message/accept',
    { conversation_id: conversationId },
    { authRequired: true },
  )
}

export function reject(conversationId: number) {
  return postJson<{ message: string; status: ConversationStatus }>(
    '/message/reject',
    { conversation_id: conversationId },
    { authRequired: true },
  )
}

export function block(userId: number) {
  return postJson<{ message: string }>('/message/block', { user_id: userId }, { authRequired: true })
}

export function unblock(userId: number) {
  return postJson<{ message: string }>('/message/unblock', { user_id: userId }, { authRequired: true })
}

export function unreadCount() {
  return postJson<{ count: number }>('/message/unreadCount', {}, { authRequired: true })
}

export function issueWebSocketTicket() {
  return postJson<{ ticket: string; expires_in: number }>('/realtime/wsTicket', {}, { authRequired: true })
}
