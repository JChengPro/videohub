export const USERNAME_MAX_LENGTH = 24
export const ACCOUNT_NAME_MAX_LENGTH = 24
export const PASSWORD_MAX_LENGTH = 64

const usernamePattern = /^[\p{Script=Han}A-Za-z0-9_]+$/u
const accountNamePattern = /^[A-Za-z][A-Za-z0-9_]*$/

export function accountNameError(accountName: string) {
  const value = accountName.trim()
  const length = Array.from(value).length
  if (length < 4 || length > ACCOUNT_NAME_MAX_LENGTH) return '账号名长度需为 4-24 个字符。'
  if (!accountNamePattern.test(value)) return '账号名必须以字母开头，且只能包含字母、数字和下划线。'
  return ''
}

export function usernameError(username: string) {
  const value = username.trim()
  const length = Array.from(value).length
  if (length < 3 || length > USERNAME_MAX_LENGTH) return '昵称长度需为 3-24 个字符。'
  if (!usernamePattern.test(value)) return '昵称仅支持中文、字母、数字和下划线。'
  return ''
}

export function passwordError(password: string) {
  if (password.length < 8 || password.length > PASSWORD_MAX_LENGTH) return '密码长度需为 8-64 个字符。'
  if (!/[A-Za-z]/.test(password) || !/\d/.test(password)) return '密码需同时包含字母和数字。'
  return ''
}
