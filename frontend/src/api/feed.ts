import { postJson } from './client'
import type {
  FeedVideoItem,
  ListByFollowingResponse,
  ListByPopularityResponse,
  ListLatestResponse,
  ListLikesCountResponse,
} from './types'

function normalizeFeedList<T extends { video_list?: FeedVideoItem[] | null }>(res: T) {
  return {
    ...res,
    video_list: Array.isArray(res.video_list) ? res.video_list : [],
  }
}

export function listLatest(input: { limit: number; latest_time: number }) {
  return postJson<ListLatestResponse>('/feed/listLatest', input).then(normalizeFeedList)
}

export function listLikesCount(input: { limit: number; likes_count_before?: number; id_before?: number }) {
  const body: Record<string, unknown> = { limit: input.limit }
  if (typeof input.likes_count_before === 'number' || typeof input.id_before === 'number') {
    body.likes_count_before = input.likes_count_before ?? 0
    body.id_before = input.id_before ?? 0
  }
  return postJson<ListLikesCountResponse>('/feed/listLikesCount', body).then(normalizeFeedList)
}

export function listByPopularity(input: { limit: number; as_of: number; offset: number }) {
  return postJson<ListByPopularityResponse>('/feed/listByPopularity', input).then(normalizeFeedList)
}

export function listByFollowing(input: { limit: number; latest_time: number }) {
  return postJson<ListByFollowingResponse>('/feed/listByFollowing', input, { authRequired: true }).then(normalizeFeedList)
}
