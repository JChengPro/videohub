import { postForm, postJson } from './client'
import type { MessageResponse, Video } from './types'
import { useAuthStore } from '../stores/auth'

export function publishVideo(input: {
  title: string
  description: string
  play_url: string
  cover_url: string
  play_object_key: string
  cover_object_key: string
}) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

export type UploadResponse = {
  url?: string
  play_url?: string
  cover_url?: string
  object_key?: string
}

export function uploadCover(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadCover', fd, { authRequired: true })
}

export function uploadVideo(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadVideo', fd, { authRequired: true })
}

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB
const CHUNK_THRESHOLD = 10 * 1024 * 1024 // 大于 10MB 的文件启用分片上传

function randHex(n: number): string {
  const arr = new Uint8Array(n)
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(arr)
  } else {
    for (let i = 0; i < arr.length; i++) arr[i] = Math.floor(Math.random() * 256)
  }
  return Array.from(arr, b => b.toString(16).padStart(2, '0')).join('')
}

async function uploadOneChunk(
  fileId: string, chunkIndex: number, totalChunks: number, blob: Blob
): Promise<void> {
  const auth = useAuthStore()
  const token = auth.token
  const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

  const res = await fetch(`${API_BASE}/video/uploadChunk`, {
    method: 'POST',
    headers: {
      'X-File-ID': fileId,
      'X-Chunk-Index': String(chunkIndex),
      'X-Total-Chunks': String(totalChunks),
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: blob,
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `chunk ${chunkIndex} upload failed`)
  }
}

export async function uploadVideoSmart(
  file: File,
  onProgress?: (pct: number) => void
): Promise<UploadResponse> {
  if (file.size > CHUNK_THRESHOLD) {
    return uploadVideoChunked(file, onProgress)
  }
  if (onProgress) onProgress(100)
  return uploadVideo(file)
}

export async function uploadVideoChunked(
  file: File,
  onProgress?: (pct: number) => void
): Promise<UploadResponse> {
  const fileId = randHex(16)
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  const fileExt = videoFileExtension(file)

  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const blob = file.slice(start, end)
    await uploadOneChunk(fileId, i, totalChunks, blob)
    if (onProgress) onProgress(Math.round(((i + 1) / totalChunks) * 100))
  }

  const res = await postJson<UploadResponse>('/video/mergeChunks', { file_id: fileId, file_ext: fileExt }, { authRequired: true })
  return res
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

export function listByAuthorId(authorId: number) {
  return postJson<Video[] | null>('/video/listByAuthorID', { author_id: authorId }).then((res) => (Array.isArray(res) ? res : []))
}

export function getDetail(id: number) {
  return postJson<Video>('/video/getDetail', { id })
}

export function deleteVideo(id: number) {
  return postJson<MessageResponse>('/video/delete', { id }, { authRequired: true })
}
