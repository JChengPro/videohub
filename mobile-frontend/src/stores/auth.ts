import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { decodeJwtPayload } from '../utils/jwt'

const TOKEN_KEY = 'jwt_token'

function readToken() {
  try { return localStorage.getItem(TOKEN_KEY) }
  catch { return null }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(readToken())
  const claims = computed(() => (token.value ? decodeJwtPayload(token.value) : null))
  const isLoggedIn = computed(() => {
    const payload = claims.value
    if (!token.value || !payload?.account_id) return false
    return typeof payload.exp !== 'number' || payload.exp * 1000 > Date.now()
  })

  function setToken(value: string) {
    token.value = value
    try { localStorage.setItem(TOKEN_KEY, value) } catch { /* Private mode may block storage. */ }
  }

  function clearToken() {
    token.value = null
    try { localStorage.removeItem(TOKEN_KEY) } catch { /* Keep in-memory logout state. */ }
  }

  return { token, claims, isLoggedIn, setToken, clearToken }
})
