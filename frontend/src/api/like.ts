import { postJson } from './client'
import type { IsLikedResponse, LikeStateResponse, Video } from './types'

export function like(videoId: number) {
  return postJson<LikeStateResponse>('/like/like', { video_id: videoId }, { authRequired: true })
}

export function unlike(videoId: number) {
  return postJson<LikeStateResponse>('/like/unlike', { video_id: videoId }, { authRequired: true })
}

export function isLiked(videoId: number) {
  return postJson<IsLikedResponse>('/like/isLiked', { video_id: videoId }, { authRequired: true })
}

export function listMyLikedVideos() {
  return postJson<Video[] | null>('/like/listMyLikedVideos', {}, { authRequired: true })
    .then((res) => (Array.isArray(res) ? res : []))
}
