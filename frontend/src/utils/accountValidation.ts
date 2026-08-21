export const USERNAME_MIN_LENGTH = 3
export const USERNAME_MAX_LENGTH = 24
export const ACCOUNT_NAME_MIN_LENGTH = 6
export const ACCOUNT_NAME_MAX_LENGTH = 12
export const PASSWORD_MIN_LENGTH = 8
export const PASSWORD_MAX_LENGTH = 64

const usernamePattern = /^[\p{Script=Han}A-Za-z0-9_]+$/u
const accountNamePattern = /^[0-9]+$/

function characterLength(value: string) {
  return Array.from(value).length
}

export function usernameRules(username: string) {
  const value = username.trim()
  const length = characterLength(value)
  return [
    { text: '应为 3–24 位，仅支持中文、字母、数字或下划线', valid: length >= USERNAME_MIN_LENGTH && length <= USERNAME_MAX_LENGTH && usernamePattern.test(value) },
  ]
}

export function accountNameRules(accountName: string) {
  const value = accountName.trim()
  const length = characterLength(value)
  return [
    { text: '应为 6–12 位纯数字，且不可与其他账号重复', valid: length >= ACCOUNT_NAME_MIN_LENGTH && length <= ACCOUNT_NAME_MAX_LENGTH && accountNamePattern.test(value) },
  ]
}

export function passwordRules(password: string) {
  const length = characterLength(password)
  return [
    { text: '应为 8–64 位，包含字母和数字且不能含空白字符', valid: length >= PASSWORD_MIN_LENGTH && length <= PASSWORD_MAX_LENGTH && /\p{L}/u.test(password) && /\p{N}/u.test(password) && !/\s/u.test(password) && new TextEncoder().encode(password).length <= 72 },
  ]
}

export function validateUsername(username: string) {
  const failed = usernameRules(username).find((rule) => !rule.valid)
  return failed ? `用户名${failed.text}` : ''
}

export function validateAccountName(accountName: string) {
  const failed = accountNameRules(accountName).find((rule) => !rule.valid)
  return failed ? `数字账号名${failed.text}` : ''
}

export function validatePassword(password: string) {
  const failed = passwordRules(password).find((rule) => !rule.valid)
  return failed ? `密码${failed.text}` : ''
}
