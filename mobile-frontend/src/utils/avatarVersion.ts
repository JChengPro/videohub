import { reactive } from 'vue'

const versions = reactive(new Map<number, number>())

export function getAvatarVersion(accountId?: number) {
  if (!accountId) return 0
  return versions.get(accountId) ?? 0
}

export function setAvatarVersion(accountId?: number, version = Date.now()) {
  if (!accountId) return 0
  const next = Math.max(version, versions.get(accountId) ?? 0)
  versions.set(accountId, next)
  return next
}

export function announceAvatarUpdated(accountId?: number) {
  const version = setAvatarVersion(accountId)
  if (accountId && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('videohub:avatar-updated', { detail: { accountId, version } }))
  }
  return version
}
