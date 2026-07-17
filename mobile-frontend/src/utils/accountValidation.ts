export const USERNAME_MAX_LENGTH = 24
export const PASSWORD_MAX_LENGTH = 64

const usernamePattern = /^[\p{Script=Han}A-Za-z0-9_]+$/u

export function usernameError(username: string) {
  const value = username.trim()
  const length = Array.from(value).length
  if (length < 3 || length > USERNAME_MAX_LENGTH) return '账号长度需为 3-24 个字符。'
  if (!usernamePattern.test(value)) return '账号仅支持中文、字母、数字和下划线。'
  return ''
}

export function passwordError(password: string) {
  if (password.length < 8 || password.length > PASSWORD_MAX_LENGTH) return '密码长度需为 8-64 个字符。'
  if (!/[A-Za-z]/.test(password) || !/\d/.test(password)) return '密码需同时包含字母和数字。'
  return ''
}
