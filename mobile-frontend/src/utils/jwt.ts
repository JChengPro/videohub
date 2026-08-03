export type JwtPayload = { account_id?: number; account_name?: string; username?: string; exp?: number }

export function decodeJwtPayload(token: string): JwtPayload | null {
  const payload = token.split('.')[1]
  if (!payload) return null
  try {
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const decoded = decodeURIComponent(
      Array.from(atob(normalized), (char) => `%${char.charCodeAt(0).toString(16).padStart(2, '0')}`).join(''),
    )
    return JSON.parse(decoded) as JwtPayload
  } catch {
    return null
  }
}
