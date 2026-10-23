import { useAuthStore } from '../stores/auth'
import type {
  Account,
  ChatMessage,
  Comment,
  Conversation,
  FeedVideo,
  MessageResponse,
  Notification,
  PublishVideoInput,
  SendMessageResponse,
  SearchUsersResponse,
  TokenResponse,
  Video,
  VideoProcessingStatus,
} from './types'

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

export class ApiError extends Error {
  status: number
  payload?: unknown

  constructor(message: string, status = 0, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

function parseResponse(text: string): unknown {
  if (!text) return null
  try { return JSON.parse(text) }
  catch { return { error: text } }
}

function responseError(data: unknown, status: number, fallback: string) {
  if (data && typeof data === 'object' && 'error' in data && typeof data.error === 'string' && data.error) return data.error
  if (status === 413) return '上传文件太大，视频最大 200MB，封面最大 10MB'
  return fallback
}

export async function request<T>(path: string, body: unknown, authRequired = false): Promise<T> {
  const auth = useAuthStore()
  if (authRequired && !auth.isLoggedIn) throw new ApiError('请先登录', 401)
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(auth.isLoggedIn && auth.token ? { Authorization: `Bearer ${auth.token}` } : {}),
    },
    body: JSON.stringify(body ?? {}),
  })
  const text = await response.text()
  const data = parseResponse(text)
  if (!response.ok) {
    if (response.status === 401 && authRequired) auth.handleUnauthorized(data)
    throw new ApiError(responseError(data, response.status, `请求失败 (${response.status})`), response.status, data)
  }
  return data as T
}

async function formRequest<T>(path: string, form: FormData): Promise<T> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn || !auth.token) throw new ApiError('请先登录', 401)
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${auth.token}` },
    body: form,
  })
  const data = parseResponse(await response.text())
  if (response.status === 401) auth.handleUnauthorized(data)
  if (!response.ok) throw new ApiError(responseError(data, response.status, `请求失败 (${response.status})`), response.status, data)
  return data as T
}

function xhrUpload<T>(path: string, body: FormData | Blob, headers: Record<string, string>, onProgress?: (progress: number) => void): Promise<T> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn || !auth.token) throw new ApiError('请先登录', 401)

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}${path}`)
    xhr.setRequestHeader('Authorization', `Bearer ${auth.token}`)
    Object.entries(headers).forEach(([key, value]) => xhr.setRequestHeader(key, value))
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onerror = () => reject(new ApiError('网络连接异常，上传失败'))
    xhr.onabort = () => reject(new ApiError('上传已取消'))
    xhr.onload = () => {
      const data = parseResponse(xhr.responseText)
      if (xhr.status === 401) auth.handleUnauthorized(data)
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new ApiError(responseError(data, xhr.status, `上传失败 (${xhr.status})`), xhr.status, data))
        return
      }
      onProgress?.(100)
      resolve(data as T)
    }
    xhr.send(body)
  })
}

async function uploadChunk(fileId: string, index: number, total: number, blob: Blob, onProgress?: (progress: number) => void) {
  return xhrUpload<{ chunk: number }>('/video/uploadChunk', blob, {
    'X-File-ID': fileId,
    'X-Chunk-Index': String(index),
    'X-Total-Chunks': String(total),
  }, onProgress)
}

const CHUNK_SIZE = 5 * 1024 * 1024
const CHUNK_THRESHOLD = 10 * 1024 * 1024
const CHUNK_CONCURRENCY = 3
const CHUNK_MAX_RETRIES = 3
const PENDING_UPLOAD_KEY = 'videohub:pending-video-upload'

export type PendingVideoUpload = {
  fileId: string
  fileName: string
  fileSize: number
  lastModified: number
  totalChunks: number
}

function uploadStorageKey() {
  const accountId = useAuthStore().claims?.account_id
  return accountId ? `${PENDING_UPLOAD_KEY}:${accountId}` : ''
}

export function getPendingVideoUpload(): PendingVideoUpload | null {
  const key = uploadStorageKey()
  if (!key) return null
  try {
    const value = JSON.parse(localStorage.getItem(key) || 'null') as Partial<PendingVideoUpload> | null
    if (!value || typeof value.fileId !== 'string' || typeof value.fileName !== 'string'
      || typeof value.fileSize !== 'number' || typeof value.lastModified !== 'number'
      || typeof value.totalChunks !== 'number') return null
    return value as PendingVideoUpload
  } catch {
    return null
  }
}

function savePendingVideoUpload(task: PendingVideoUpload) {
  const key = uploadStorageKey()
  if (!key) return
  try { localStorage.setItem(key, JSON.stringify(task)) } catch { /* Upload still works without local task hints. */ }
}

function clearPendingVideoUpload(fileId: string) {
  const key = uploadStorageKey()
  if (!key) return
  try {
    const task = getPendingVideoUpload()
    if (task?.fileId === fileId) localStorage.removeItem(key)
  } catch { /* Upload has already completed; stale hints are harmless. */ }
}

export function videoUploadId(file: Pick<File, 'name' | 'size' | 'lastModified'>): string {
  const input = `${file.name}:${file.size}:${file.lastModified}`
  const seeds = [0x811c9dc5, 0x9e3779b9, 0x85ebca6b, 0xc2b2ae35]
  return seeds.map((seed) => {
    let hash = seed >>> 0
    for (let index = 0; index < input.length; index += 1) {
      hash ^= input.charCodeAt(index)
      hash = Math.imul(hash, 0x01000193) >>> 0
    }
    return hash.toString(16).padStart(8, '0')
  }).join('')
}

function wait(ms: number) {
  return new Promise<void>((resolve) => window.setTimeout(resolve, ms))
}

function isRetryableUploadError(error: unknown) {
  return !(error instanceof ApiError)
    || error.status === 0
    || error.status === 408
    || error.status === 429
    || error.status >= 500
}

async function uploadChunkWithRetry(
  fileId: string,
  index: number,
  total: number,
  blob: Blob,
  onProgress: (progress: number) => void,
) {
  for (let retry = 0; ; retry += 1) {
    try {
      await uploadChunk(fileId, index, total, blob, onProgress)
      return
    } catch (error) {
      onProgress(0)
      if (retry >= CHUNK_MAX_RETRIES || !isRetryableUploadError(error)) throw error
      await wait(300 * (2 ** retry))
    }
  }
}

async function uploadVideoSmart(file: File, onProgress?: (progress: number) => void) {
  if (file.size <= CHUNK_THRESHOLD) {
    const form = new FormData()
    form.append('file', file)
    return xhrUpload<{ url?: string; play_url?: string; object_key?: string }>('/video/uploadVideo', form, {}, onProgress)
  }

  const total = Math.ceil(file.size / CHUNK_SIZE)
  const fileId = videoUploadId(file)
  const fileExt = videoFileExtension(file)
  savePendingVideoUpload({
    fileId,
    fileName: file.name,
    fileSize: file.size,
    lastModified: file.lastModified,
    totalChunks: total,
  })

  const status = await request<{ uploaded: number[] }>('/video/chunkStatus', { file_id: fileId }, true)
  const uploaded = new Set((status.uploaded || []).filter((index) => Number.isInteger(index) && index >= 0 && index < total))
  const missing = Array.from({ length: total }, (_, index) => index).filter((index) => !uploaded.has(index))
  const chunkSizeAt = (index: number) => Math.min(CHUNK_SIZE, file.size - index * CHUNK_SIZE)
  let completedBytes = Array.from(uploaded).reduce((sum, index) => sum + chunkSizeAt(index), 0)
  const activeProgress = new Map<number, number>()

  const reportProgress = () => {
    const activeBytes = Array.from(activeProgress.entries()).reduce(
      (sum, [index, progress]) => sum + chunkSizeAt(index) * progress / 100,
      0,
    )
    onProgress?.(Math.min(99, Math.round(((completedBytes + activeBytes) / file.size) * 100)))
  }
  reportProgress()

  let cursor = 0
  const worker = async () => {
    while (cursor < missing.length) {
      const index = missing[cursor]
      cursor += 1
      if (index === undefined) return
      const start = index * CHUNK_SIZE
      const blob = file.slice(start, Math.min(file.size, start + CHUNK_SIZE))
      await uploadChunkWithRetry(fileId, index, total, blob, (progress) => {
        activeProgress.set(index, progress)
        reportProgress()
      })
      activeProgress.delete(index)
      completedBytes += blob.size
      reportProgress()
    }
  }

  await Promise.all(Array.from({ length: Math.min(CHUNK_CONCURRENCY, missing.length) }, () => worker()))
  const result = await request<{ url?: string; play_url?: string; object_key?: string }>('/video/mergeChunks', {
    file_id: fileId,
    file_ext: fileExt,
    total_chunks: total,
    file_size: file.size,
  }, true)
  clearPendingVideoUpload(fileId)
  onProgress?.(100)
  return result
}

export const VIDEO_ACCEPT = 'video/mp4,video/quicktime,video/x-m4v,video/webm,video/3gpp,.mp4,.mov,.m4v,.webm,.3gp,.3gpp'
const allowedVideoExtensions = new Set(['.mp4', '.mov', '.m4v', '.webm', '.3gp', '.3gpp'])

export function videoFileExtension(file: File) {
  const dot = file.name.lastIndexOf('.')
  const ext = dot >= 0 ? file.name.slice(dot).toLowerCase() : ''
  return allowedVideoExtensions.has(ext) ? ext : ''
}

export function isSupportedVideo(file: File) {
  return allowedVideoExtensions.has(videoFileExtension(file))
}

function normalizeFeed<T extends { video_list?: FeedVideo[] | null }>(response: T) {
  return { ...response, video_list: Array.isArray(response.video_list) ? response.video_list : [] }
}

export const api = {
  login: (accountName: string, password: string) => request<TokenResponse>('/account/login', { account_name: accountName, password }),
  register: (accountName: string, username: string, password: string) =>
    request<MessageResponse>('/account/register', { account_name: accountName, username, password }),
  checkAccountName: (accountName: string) =>
    request<{ account_name: string; available: boolean }>('/account/checkAccountName', { account_name: accountName }),
  me: () => request<Account>('/account/me', {}, true),
  logout: () => request<MessageResponse>('/account/logout', {}, true),
  rename: (newUsername: string) => request<TokenResponse>('/account/rename', { new_username: newUsername }, true),
  changePassword: (oldPassword: string, newPassword: string) =>
    request<MessageResponse>('/account/changePassword', { old_password: oldPassword, new_password: newPassword }, true),
  uploadAvatar: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return formRequest<{ avatar_url: string }>('/account/avatar', form)
  },
  latest: (latestTime = 0) =>
    request<{ video_list: FeedVideo[]; next_time: number; has_more: boolean }>('/feed/listLatest', { limit: 12, latest_time: latestTime }).then(normalizeFeed),
  followingFeed: (latestTime = 0) =>
    request<{ video_list: FeedVideo[]; next_time: number; has_more: boolean }>('/feed/listByFollowing', { limit: 12, latest_time: latestTime }, true).then(normalizeFeed),
  hot: (offset = 0, asOf = 0) =>
    request<{ video_list: FeedVideo[]; next_offset: number; as_of: number; has_more: boolean }>('/feed/listByPopularity', { limit: 12, offset, as_of: asOf }).then(normalizeFeed),
  like: (id: number) => request<{ is_liked: boolean; likes_count: number }>('/like/like', { video_id: id }, true),
  unlike: (id: number) => request<{ is_liked: boolean; likes_count: number }>('/like/unlike', { video_id: id }, true),
  isLiked: (id: number) => request<{ is_liked: boolean }>('/like/isLiked', { video_id: id }, true),
  comments: (id: number) => request<Comment[] | null>('/comment/listAll', { video_id: id }).then((v) => v ?? []),
  comment: (id: number, content: string) => request<MessageResponse>('/comment/publish', { video_id: id, content }, true),
  deleteComment: (id: number) => request<MessageResponse>('/comment/delete', { comment_id: id }, true),
  follow: (id: number) => request<MessageResponse>('/social/follow', { vlogger_id: id }, true),
  unfollow: (id: number) => request<MessageResponse>('/social/unfollow', { vlogger_id: id }, true),
  followers: (id?: number) => request<{ followers: Account[] | null }>('/social/getAllFollowers', id ? { vlogger_id: id } : {}, true)
    .then((value) => ({ followers: value.followers ?? [] })),
  following: (id?: number) => request<{ vloggers: Account[] | null }>('/social/getAllVloggers', id ? { follower_id: id } : {}, true)
    .then((value) => ({ vloggers: value.vloggers ?? [] })),
  videosByAuthor: (id: number) => request<Video[] | null>('/video/listByAuthorID', { author_id: id }).then((v) => v ?? []),
  videoDetail: (id: number) => request<Video>('/video/getDetail', { id }),
  accountById: (id: number) => request<Account>('/account/findByID', { id }),
  searchUsers: (query: string, limit = 20, offset = 0) =>
    request<SearchUsersResponse>('/account/search', { query, limit, offset }),
  likedVideos: () => request<Video[] | null>('/like/listMyLikedVideos', {}, true).then((value) => value ?? []),
  deleteVideo: (id: number) => request<MessageResponse>('/video/delete', { id }, true),
  notifications: (beforeId = 0) =>
    request<{ notifications: Notification[] | null; has_more: boolean; next_before_id: number }>('/notification/list', { limit: 30, before_id: beforeId }, true)
      .then((value) => ({ ...value, notifications: value.notifications ?? [] })),
  unread: () => request<{ count: number }>('/notification/unreadCount', {}, true),
  markRead: (id: number) => request<MessageResponse>('/notification/markRead', { id }, true),
  markAllRead: () => request<MessageResponse>('/notification/markAllRead', {}, true),
  conversations: () => request<{ conversations: Conversation[] | null }>('/message/listConversations', {}, true)
    .then((value) => ({ conversations: value.conversations ?? [] })),
  chatMessages: (conversationId: number, beforeId = 0) =>
    request<{ messages: ChatMessage[] | null; has_more: boolean; next_before_id: number }>(
      '/message/listMessages',
      { conversation_id: conversationId, before_id: beforeId, limit: 30 },
      true,
    ).then((value) => ({ ...value, messages: value.messages ?? [] })),
  sendChatMessage: (receiverId: number, clientMessageId: string, content: string) =>
    request<SendMessageResponse>(
      '/message/send',
      { receiver_id: receiverId, client_message_id: clientMessageId, content },
      true,
    ),
  markChatRead: (conversationId: number, messageId = 0) =>
    request<{ conversation_id: number; read_message_id: number; peer_id: number; unread_count: number }>(
      '/message/markRead',
      { conversation_id: conversationId, message_id: messageId },
      true,
    ),
  acceptConversation: (conversationId: number) =>
    request<MessageResponse>('/message/accept', { conversation_id: conversationId }, true),
  rejectConversation: (conversationId: number) =>
    request<MessageResponse>('/message/reject', { conversation_id: conversationId }, true),
  blockUser: (userId: number) => request<MessageResponse>('/message/block', { user_id: userId }, true),
  unblockUser: (userId: number) => request<MessageResponse>('/message/unblock', { user_id: userId }, true),
  chatUnread: () => request<{ count: number }>('/message/unreadCount', {}, true),
  wsTicket: () => request<{ ticket: string; expires_in: number }>('/realtime/wsTicket', {}, true),
  uploadCover: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return formRequest<{ url?: string; cover_url?: string; object_key?: string }>('/video/uploadCover', form)
  },
  uploadVideo: uploadVideoSmart,
  publish: (body: PublishVideoInput) => request<Video>('/video/publish', body, true),
  processingStatus: (id: number) => request<VideoProcessingStatus>('/video/processingStatus', { id }, true),
}
