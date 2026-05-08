import { useAuthStore } from '../stores/auth'

export class ApiError extends Error {
  status: number
  payload?: unknown

  constructor(message: string, status: number, payload?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.payload = payload
  }
}

type ApiErrorBody = { error?: string }

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

function errorMessageFromResponse(status: number, data: unknown) {
  if (data && typeof data === 'object' && (data as ApiErrorBody).error) {
    return String((data as ApiErrorBody).error)
  }
  if (status === 413) {
    return '上传文件太大，请压缩后再试（视频最大 200MB，封面最大 10MB）'
  }
  return `请求失败 (${status})`
}

export async function postJson<T>(path: string, body: unknown, options?: { authRequired?: boolean }): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError('需要先登录（缺少 token）', 401)
  }

  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body ?? {}),
  })

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }

  if (!res.ok) {
    if (res.status === 401) {
      auth.clearToken()
    }
    const msg = errorMessageFromResponse(res.status, data)
    throw new ApiError(msg, res.status, data)
  }

  return data as T
}

export async function postForm<T>(path: string, body: FormData, options?: { authRequired?: boolean }): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError('需要先登录（缺少 token）', 401)
  }

  const headers: Record<string, string> = {}
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body,
  })

  const text = await res.text()
  let data: unknown = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }

  if (!res.ok) {
    if (res.status === 401) {
      auth.clearToken()
    }
    const msg = errorMessageFromResponse(res.status, data)
    throw new ApiError(msg, res.status, data)
  }

  return data as T
}
