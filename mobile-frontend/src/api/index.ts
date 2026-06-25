import { useAuthStore } from '../stores/auth'
import type { Account, Comment, FeedVideo, MessageResponse, Notification, TokenResponse, Video } from './types'

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

export class ApiError extends Error {}

function parseResponse(text: string) {
  if (!text) return null
  try { return JSON.parse(text) }
  catch { return { error: text } }
}

async function request<T>(path: string, body: unknown, authRequired = false): Promise<T> {
  const auth = useAuthStore()
  if (authRequired && !auth.isLoggedIn) throw new ApiError('请先登录')
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
    if (response.status === 401) auth.clearToken()
    throw new ApiError(data?.error || `请求失败 (${response.status})`)
  }
  return data as T
}

async function formRequest<T>(path: string, form: FormData): Promise<T> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn || !auth.token) throw new ApiError('请先登录')
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${auth.token}` },
    body: form,
  })
  const data = parseResponse(await response.text())
  if (response.status === 401) auth.clearToken()
  if (!response.ok) throw new ApiError(data?.error || `请求失败 (${response.status})`)
  return data as T
}

function xhrUpload<T>(path: string, body: FormData | Blob, headers: Record<string, string>, onProgress?: (progress: number) => void): Promise<T> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn || !auth.token) throw new ApiError('请先登录')

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}${path}`)
    xhr.setRequestHeader('Authorization', `Bearer ${auth.token}`)
    Object.entries(headers).forEach(([key, value]) => xhr.setRequestHeader(key, value))
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onerror = () => reject(new ApiError('网络连接异常，上传失败'))
    xhr.onload = () => {
      const data = parseResponse(xhr.responseText)
      if (xhr.status === 401) auth.clearToken()
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new ApiError(data?.error || `上传失败 (${xhr.status})`))
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

async function uploadVideoSmart(file: File, onProgress?: (progress: number) => void) {
  if (file.size <= 10 * 1024 * 1024) {
    const form = new FormData()
    form.append('file', file)
    return xhrUpload<{ url?: string; play_url?: string; object_key?: string }>('/video/uploadVideo', form, {}, onProgress)
  }

  const chunkSize = 5 * 1024 * 1024
  const total = Math.ceil(file.size / chunkSize)
  const fileId = randomHex(16)
  const fileExt = videoFileExtension(file)
  for (let index = 0; index < total; index += 1) {
    await uploadChunk(
      fileId,
      index,
      total,
      file.slice(index * chunkSize, Math.min(file.size, (index + 1) * chunkSize)),
      (chunkProgress) => onProgress?.(Math.min(95, Math.round(((index + chunkProgress / 100) / total) * 95))),
    )
  }
  const result = await request<{ url?: string; play_url?: string; object_key?: string }>('/video/mergeChunks', { file_id: fileId, file_ext: fileExt }, true)
  onProgress?.(100)
  return result
}

function randomHex(size: number) {
  const bytes = new Uint8Array(size)
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes)
  } else {
    for (let index = 0; index < bytes.length; index += 1) bytes[index] = Math.floor(Math.random() * 256)
  }
  return Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')
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
  login: (username: string, password: string) => request<TokenResponse>('/account/login', { username, password }),
  register: (username: string, password: string) => request<MessageResponse>('/account/register', { username, password }),
  logout: () => request<MessageResponse>('/account/logout', {}, true),
  rename: (newUsername: string) => request<TokenResponse>('/account/rename', { new_username: newUsername }, true),
  changePassword: (username: string, oldPassword: string, newPassword: string) =>
    request<MessageResponse>('/account/changePassword', { username, old_password: oldPassword, new_password: newPassword }),
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
  likedVideos: () => request<Video[] | null>('/like/listMyLikedVideos', {}, true).then((value) => value ?? []),
  deleteVideo: (id: number) => request<MessageResponse>('/video/delete', { id }, true),
  notifications: (beforeId = 0) =>
    request<{ notifications: Notification[] | null; has_more: boolean; next_before_id: number }>('/notification/list', { limit: 30, before_id: beforeId }, true)
      .then((value) => ({ ...value, notifications: value.notifications ?? [] })),
  unread: () => request<{ count: number }>('/notification/unreadCount', {}, true),
  markRead: (id: number) => request<MessageResponse>('/notification/markRead', { id }, true),
  markAllRead: () => request<MessageResponse>('/notification/markAllRead', {}, true),
  uploadCover: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return formRequest<{ url?: string; cover_url?: string; object_key?: string }>('/video/uploadCover', form)
  },
  uploadVideo: uploadVideoSmart,
  publish: (body: object) => request<Video>('/video/publish', body, true),
}
