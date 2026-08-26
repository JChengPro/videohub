import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { decodeJwtPayload } from '../utils/jwt'

const TOKEN_KEY = 'jwt_token'
export type SessionEndReason = 'replaced' | 'revoked' | 'expired'

function readToken() {
  try { return localStorage.getItem(TOKEN_KEY) }
  catch { return null }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(readToken())
  const sessionEndReason = ref<SessionEndReason | null>(null)
  const claims = computed(() => (token.value ? decodeJwtPayload(token.value) : null))
  const isLoggedIn = computed(() => {
    const payload = claims.value
    if (!token.value || !payload?.account_id) return false
    return typeof payload.exp !== 'number' || payload.exp * 1000 > Date.now()
  })

  function setToken(value: string) {
    sessionEndReason.value = null
    token.value = value
    try { localStorage.setItem(TOKEN_KEY, value) } catch { /* Private mode may block storage. */ }
  }

  function clearToken() {
    sessionEndReason.value = null
    token.value = null
    try { localStorage.removeItem(TOKEN_KEY) } catch { /* Keep in-memory logout state. */ }
  }

  function endSession(reason: SessionEndReason) {
    if (!token.value) return
    token.value = null
    try { localStorage.removeItem(TOKEN_KEY) } catch { /* Keep in-memory logout state. */ }
    sessionEndReason.value = reason
  }

  function handleUnauthorized(payload: unknown) {
    if (!token.value) return
    const code = payload && typeof payload === 'object' && 'code' in payload
      ? String(payload.code)
      : ''
    if (code === 'SESSION_REPLACED') endSession('replaced')
    else if (code === 'SESSION_REVOKED') endSession('revoked')
    else endSession('expired')
  }

  function dismissSessionNotice() {
    sessionEndReason.value = null
  }

  function syncFromStorage() {
    token.value = readToken()
  }

  return { token, claims, isLoggedIn, sessionEndReason, setToken, clearToken, endSession, handleUnauthorized, dismissSessionNotice, syncFromStorage }
})
