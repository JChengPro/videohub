export type MessageResponse = { message: string }
export type TokenResponse = { token: string }
export type Account = { id: number; account_id?: number; account_name?: string; username: string; avatar_url?: string }
export type SearchUsersResponse = { users: Account[]; has_more: boolean; next_offset: number }

export type FeedVideo = {
  id: number
  author: Account
  title: string
  description?: string
  play_url: string
  cover_url: string
  create_time: number
  likes_count: number
  comments_count: number
  is_liked: boolean
}

export type Video = {
  id: number
  author_id: number
  username: string
  title: string
  description?: string
  play_url: string
  cover_url: string
  create_time: string
  likes_count: number
  comments_count: number
}

export type PublishVideoInput = {
  title: string
  description: string
  play_url: string
  cover_url: string
  play_object_key: string
  cover_object_key: string
}

export type Comment = {
  id: number
  username: string
  video_id: number
  author_id: number
  content: string
  created_at: string
}

export type Notification = {
  id: number
  actor_id: number
  actor_username: string
  type: 'like' | 'comment' | 'follow'
  target_type: 'video' | 'account'
  target_id: number
  content: string
  is_read: boolean
  create_time: string
}

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
