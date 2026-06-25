import { postJson } from './client'
import type { GetAllFollowersResponse, GetAllVloggersResponse, MessageResponse } from './types'

function normalizeFollowers(res: GetAllFollowersResponse) {
  return {
    ...res,
    followers: Array.isArray(res.followers) ? res.followers : [],
  }
}

function normalizeVloggers(res: GetAllVloggersResponse) {
  return {
    ...res,
    vloggers: Array.isArray(res.vloggers) ? res.vloggers : [],
  }
}

export function follow(vloggerId: number) {
  return postJson<MessageResponse>('/social/follow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function unfollow(vloggerId: number) {
  return postJson<MessageResponse>('/social/unfollow', { vlogger_id: vloggerId }, { authRequired: true })
}

export function getAllFollowers(vloggerId?: number) {
  return postJson<GetAllFollowersResponse>(
    '/social/getAllFollowers',
    vloggerId ? { vlogger_id: vloggerId } : {},
    { authRequired: true },
  ).then(normalizeFollowers)
}

export function getAllVloggers(followerId?: number) {
  return postJson<GetAllVloggersResponse>(
    '/social/getAllVloggers',
    followerId ? { follower_id: followerId } : {},
    { authRequired: true },
  ).then(normalizeVloggers)
}
