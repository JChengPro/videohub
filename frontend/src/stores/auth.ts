import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { decodeJwtPayload, type JwtPayload } from '../utils/jwt'

const TOKEN_KEY = 'jwt_token'
export type SessionEndReason = 'replaced' | 'revoked' | 'expired'

function readToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

function writeToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

function removeToken() {
  localStorage.removeItem(TOKEN_KEY)
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(readToken())
  const sessionEndReason = ref<SessionEndReason | null>(null)

  const claims = computed<JwtPayload | null>(() => (token.value ? decodeJwtPayload(token.value) : null))
  const isLoggedIn = computed(() => {
    const payload = claims.value
    if (!token.value || !payload?.account_id) return false
    return typeof payload.exp !== 'number' || payload.exp * 1000 > Date.now()
  })

  function setToken(newToken: string) {
    sessionEndReason.value = null
    token.value = newToken
    writeToken(newToken)
  }

  function clearToken() {
    sessionEndReason.value = null
    token.value = null
    removeToken()
  }

  function endSession(reason: SessionEndReason) {
    if (!token.value) return
    token.value = null
    removeToken()
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

  return { token, isLoggedIn, claims, sessionEndReason, setToken, clearToken, endSession, handleUnauthorized, dismissSessionNotice, syncFromStorage }
})
