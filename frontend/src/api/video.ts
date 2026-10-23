import { ApiError, postForm, postJson } from './client'
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
  published_video?: Video
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

function uploadVideoWithProgress(file: File, onProgress?: (pct: number) => void): Promise<UploadResponse> {
  const auth = useAuthStore()
  if (!auth.isLoggedIn || !auth.token) return Promise.reject(new ApiError('需要先登录（缺少 token）', 401))
  const apiBase = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'
  const body = new FormData()
  body.append('file', file)

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${apiBase}/video/uploadVideo`)
    xhr.setRequestHeader('Authorization', `Bearer ${auth.token}`)
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onerror = () => reject(new ApiError('网络连接异常，视频上传失败', 0))
    xhr.onabort = () => reject(new ApiError('视频上传已取消', 0))
    xhr.onload = () => {
      let data: unknown = null
      try { data = xhr.responseText ? JSON.parse(xhr.responseText) : null }
      catch { data = xhr.responseText }
      if (xhr.status === 401) auth.handleUnauthorized(data)
      if (xhr.status < 200 || xhr.status >= 300) {
        const message = data && typeof data === 'object' && 'error' in data && typeof data.error === 'string'
          ? data.error
          : `视频上传失败 (${xhr.status})`
        reject(new ApiError(message, xhr.status, data))
        return
      }
      onProgress?.(100)
      resolve(data as UploadResponse)
    }
    xhr.send(body)
  })
}

const CHUNK_SIZE = 10 * 1024 * 1024
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

export type MultipartTask = {
  id: string
  file_id: string
  file_name: string
  file_size: number
  total_parts: number
  status: 'uploading' | 'completing' | 'completed' | 'aborting' | 'expired' | 'failed'
  video_id?: number
  expires_at: string
  failure_reason?: string
}

export function listMultipartTasks() {
  return postJson<{ tasks: MultipartTask[] }>('/video/multipart/tasks', {}, { authRequired: true })
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

// A deterministic, non-cryptographic fingerprint avoids reading the whole video into memory.
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

async function uploadOneChunk(
  fileId: string,
  chunkIndex: number,
  totalChunks: number,
  blob: Blob,
  onProgress?: (pct: number) => void,
): Promise<void> {
  const auth = useAuthStore()
  const token = auth.token
  const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/video/uploadChunk`)
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.setRequestHeader('X-File-ID', fileId)
    xhr.setRequestHeader('X-Chunk-Index', String(chunkIndex))
    xhr.setRequestHeader('X-Total-Chunks', String(totalChunks))
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onerror = () => reject(new ApiError(`分片 ${chunkIndex + 1} 网络上传失败`, 0))
    xhr.onabort = () => reject(new ApiError(`分片 ${chunkIndex + 1} 上传已取消`, 0))
    xhr.onload = () => {
      let payload: unknown = null
      try { payload = xhr.responseText ? JSON.parse(xhr.responseText) : null } catch { payload = xhr.responseText }
      if (xhr.status === 401) auth.handleUnauthorized(payload)
      if (xhr.status < 200 || xhr.status >= 300) {
        const message = payload && typeof payload === 'object' && 'error' in payload && typeof payload.error === 'string'
          ? payload.error
          : `分片 ${chunkIndex + 1} 上传失败 (${xhr.status})`
        reject(new ApiError(message, xhr.status, payload))
        return
      }
      onProgress?.(100)
      resolve()
    }
    xhr.send(blob)
  })
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
  chunkIndex: number,
  totalChunks: number,
  blob: Blob,
  onProgress: (pct: number) => void,
) {
  for (let retry = 0; ; retry += 1) {
    try {
      await uploadOneChunk(fileId, chunkIndex, totalChunks, blob, onProgress)
      return
    } catch (error) {
      onProgress(0)
      if (retry >= CHUNK_MAX_RETRIES || !isRetryableUploadError(error)) throw error
      await wait(300 * (2 ** retry))
    }
  }
}

export async function uploadVideoSmart(
  file: File,
  onProgress?: (pct: number) => void,
  publishInput?: Pick<Parameters<typeof publishVideo>[0], 'title' | 'description' | 'cover_object_key'>,
): Promise<UploadResponse> {
  if (file.size > CHUNK_THRESHOLD) {
    return uploadVideoChunked(file, onProgress, publishInput)
  }
  return uploadVideoWithProgress(file, onProgress)
}

export async function uploadVideoChunked(
  file: File,
  onProgress?: (pct: number) => void,
  publishInput?: Pick<Parameters<typeof publishVideo>[0], 'title' | 'description' | 'cover_object_key'>,
): Promise<UploadResponse> {
  const fileId = videoUploadId(file)
  const totalChunks = Math.ceil(file.size / CHUNK_SIZE)
  const fileExt = videoFileExtension(file)
  const task: PendingVideoUpload = {
    fileId,
    fileName: file.name,
    fileSize: file.size,
    lastModified: file.lastModified,
    totalChunks,
  }
  savePendingVideoUpload(task)

  const init = await postJson<{
    mode: 'local' | 'oss'
    task_id?: string
    part_size?: number
    total_parts?: number
    uploaded_parts?: number[]
  }>('/video/multipart/init', {
    file_id: fileId,
    file_name: file.name,
    file_size: file.size,
    total_parts: totalChunks,
  }, { authRequired: true })

  if (init.mode === 'oss') {
    if (!init.task_id || !publishInput) throw new ApiError('OSS 分片任务缺少发布信息', 0)
    const published = await uploadVideoToOSS(file, init.task_id, init.uploaded_parts || [], onProgress, publishInput)
    clearPendingVideoUpload(fileId)
    onProgress?.(100)
    return { published_video: published }
  }

  const status = await postJson<{ uploaded: number[] }>('/video/chunkStatus', { file_id: fileId }, { authRequired: true })
  const uploaded = new Set((status.uploaded || []).filter((index) => Number.isInteger(index) && index >= 0 && index < totalChunks))
  const missing = Array.from({ length: totalChunks }, (_, index) => index).filter((index) => !uploaded.has(index))
  const chunkSizeAt = (index: number) => Math.min(CHUNK_SIZE, file.size - index * CHUNK_SIZE)
  let completedBytes = Array.from(uploaded).reduce((sum, index) => sum + chunkSizeAt(index), 0)
  const activeProgress = new Map<number, number>()

  const reportProgress = () => {
    const activeBytes = Array.from(activeProgress.entries()).reduce(
      (sum, [index, pct]) => sum + chunkSizeAt(index) * pct / 100,
      0,
    )
    onProgress?.(Math.min(99, Math.round(((completedBytes + activeBytes) / file.size) * 100)))
  }
  reportProgress()

  let cursor = 0
  const worker = async () => {
    while (cursor < missing.length) {
      const chunkIndex = missing[cursor]
      cursor += 1
      if (chunkIndex === undefined) return
      const start = chunkIndex * CHUNK_SIZE
      const blob = file.slice(start, Math.min(start + CHUNK_SIZE, file.size))
      await uploadChunkWithRetry(fileId, chunkIndex, totalChunks, blob, (pct) => {
        activeProgress.set(chunkIndex, pct)
        reportProgress()
      })
      activeProgress.delete(chunkIndex)
      completedBytes += blob.size
      reportProgress()
    }
  }

  await Promise.all(Array.from({ length: Math.min(CHUNK_CONCURRENCY, missing.length) }, () => worker()))
  const res = await postJson<UploadResponse>('/video/mergeChunks', {
    file_id: fileId,
    file_ext: fileExt,
    total_chunks: totalChunks,
    file_size: file.size,
  }, { authRequired: true })
  clearPendingVideoUpload(fileId)
  onProgress?.(100)
  return res
}

function putOSSPart(url: string, blob: Blob, onProgress: (pct: number) => void): Promise<string> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', url)
    xhr.setRequestHeader('Content-Type', 'application/octet-stream')
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100))
    }
    xhr.onerror = () => reject(new ApiError('OSS 分片网络上传失败', 0))
    xhr.onabort = () => reject(new ApiError('OSS 分片上传已取消', 0))
    xhr.onload = () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        reject(new ApiError(`OSS 分片上传失败 (${xhr.status})`, xhr.status))
        return
      }
      const etag = xhr.getResponseHeader('ETag')
      if (!etag) {
        reject(new ApiError('OSS 未暴露 ETag，请在 Bucket CORS 中暴露 ETag 响应头', 0))
        return
      }
      onProgress(100)
      resolve(etag)
    }
    xhr.send(blob)
  })
}

async function uploadOSSPartWithRetry(taskId: string, partNumber: number, blob: Blob, onProgress: (pct: number) => void) {
  for (let retry = 0; ; retry += 1) {
    try {
      // Each attempt obtains a fresh URL, so an expired signature is never reused.
      const signed = await postJson<{ parts: Array<{ part_number: number; url: string }> }>('/video/multipart/signParts', {
        task_id: taskId,
        part_numbers: [partNumber],
      }, { authRequired: true })
      const url = signed.parts[0]?.url
      if (!url) throw new ApiError('未获取到 OSS 分片上传地址', 0)
      const etag = await putOSSPart(url, blob, onProgress)
      await postJson('/video/multipart/partCompleted', {
        task_id: taskId,
        part_number: partNumber,
        etag,
        size: blob.size,
      }, { authRequired: true })
      return
    } catch (error) {
      onProgress(0)
      if (retry >= CHUNK_MAX_RETRIES || !isRetryableUploadError(error)) throw error
      await wait(300 * (2 ** retry))
    }
  }
}

async function uploadVideoToOSS(
  file: File,
  taskId: string,
  uploadedParts: number[],
  onProgress: ((pct: number) => void) | undefined,
  publishInput: Pick<Parameters<typeof publishVideo>[0], 'title' | 'description' | 'cover_object_key'>,
): Promise<Video> {
  const total = Math.ceil(file.size / CHUNK_SIZE)
  const uploaded = new Set(uploadedParts.filter((part) => Number.isInteger(part) && part >= 1 && part <= total))
  const missing = Array.from({ length: total }, (_, index) => index + 1).filter((part) => !uploaded.has(part))
  const partSizeAt = (part: number) => Math.min(CHUNK_SIZE, file.size - (part - 1) * CHUNK_SIZE)
  let completedBytes = Array.from(uploaded).reduce((sum, part) => sum + partSizeAt(part), 0)
  const activeProgress = new Map<number, number>()
  const report = () => {
    const activeBytes = Array.from(activeProgress.entries()).reduce((sum, [part, pct]) => sum + partSizeAt(part) * pct / 100, 0)
    onProgress?.(Math.min(99, Math.round((completedBytes + activeBytes) / file.size * 100)))
  }
  report()
  let cursor = 0
  const worker = async () => {
    while (cursor < missing.length) {
      const part = missing[cursor++]
      if (part === undefined) return
      const start = (part - 1) * CHUNK_SIZE
      const blob = file.slice(start, Math.min(start + CHUNK_SIZE, file.size))
      await uploadOSSPartWithRetry(taskId, part, blob, (pct) => { activeProgress.set(part, pct); report() })
      activeProgress.delete(part)
      completedBytes += blob.size
      report()
    }
  }
  await Promise.all(Array.from({ length: Math.min(CHUNK_CONCURRENCY, missing.length) }, () => worker()))
  return postJson<Video>('/video/multipart/complete', {
    task_id: taskId,
    title: publishInput.title,
    description: publishInput.description,
    cover_object_key: publishInput.cover_object_key,
  }, { authRequired: true })
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
