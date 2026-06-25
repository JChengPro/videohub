export type MessageResponse = { message: string }
export type TokenResponse = { token: string }
export type Account = { id: number; username: string }

export type FeedVideo = {
  id: number
  author: Account
  title: string
  description?: string
  play_url: string
  cover_url: string
  create_time: number
  likes_count: number
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
