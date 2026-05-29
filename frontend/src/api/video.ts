import { postForm, postJson } from './client'
import type { MessageResponse, Video } from './types'
import { useAuthStore } from '../stores/auth'

export function publishVideo(input: { title: string; description: string; play_url: string; cover_url: string }) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

export type UploadResponse = { url: string; play_url?: string; cover_url?: string }

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
  crypto.getRandomValues(arr)
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

  for (let i = 0; i < totalChunks; i++) {
    const start = i * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const blob = file.slice(start, end)
    await uploadOneChunk(fileId, i, totalChunks, blob)
    if (onProgress) onProgress(Math.round(((i + 1) / totalChunks) * 100))
  }

  const res = await postJson<UploadResponse>('/video/mergeChunks', { file_id: fileId }, { authRequired: true })
  return res
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
