import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { decodeJwtPayload, type JwtPayload } from '../utils/jwt'

const TOKEN_KEY = 'jwt_token'

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

  const claims = computed<JwtPayload | null>(() => (token.value ? decodeJwtPayload(token.value) : null))
  const isLoggedIn = computed(() => {
    const payload = claims.value
    if (!token.value || !payload?.account_id) return false
    return typeof payload.exp !== 'number' || payload.exp * 1000 > Date.now()
  })

  function setToken(newToken: string) {
    token.value = newToken
    writeToken(newToken)
  }

  function clearToken() {
    token.value = null
    removeToken()
  }

  function syncFromStorage() {
    token.value = readToken()
  }

  return { token, isLoggedIn, claims, setToken, clearToken, syncFromStorage }
})
