import { postForm, postJson } from './client'
import type { MessageResponse, Video } from './types'

export function publishVideo(input: { title: string; description: string; play_url: string; cover_url: string }) {
  return postJson<Video>('/video/publish', input, { authRequired: true })
}

export type UploadResponse = { url: string; play_url?: string; cover_url?: string }

export function uploadVideo(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadVideo', fd, { authRequired: true })
}

export function uploadCover(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  return postForm<UploadResponse>('/video/uploadCover', fd, { authRequired: true })
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
