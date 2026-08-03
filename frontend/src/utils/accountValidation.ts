export const USERNAME_MIN_LENGTH = 3
export const USERNAME_MAX_LENGTH = 24
export const ACCOUNT_NAME_MIN_LENGTH = 4
export const ACCOUNT_NAME_MAX_LENGTH = 24
export const PASSWORD_MIN_LENGTH = 8
export const PASSWORD_MAX_LENGTH = 64

const usernamePattern = /^[\p{Script=Han}A-Za-z0-9_]+$/u
const accountNamePattern = /^[A-Za-z][A-Za-z0-9_]*$/

function characterLength(value: string) {
  return Array.from(value).length
}

export function usernameRules(username: string) {
  const value = username.trim()
  const length = characterLength(value)
  return [
    { text: '长度为 3-24 个字符', valid: length >= USERNAME_MIN_LENGTH && length <= USERNAME_MAX_LENGTH },
    { text: '仅包含中文、字母、数字或下划线', valid: usernamePattern.test(value) },
  ]
}

export function accountNameRules(accountName: string) {
  const value = accountName.trim()
  const length = characterLength(value)
  return [
    { text: '长度为 4-24 个字符', valid: length >= ACCOUNT_NAME_MIN_LENGTH && length <= ACCOUNT_NAME_MAX_LENGTH },
    { text: '以字母开头，仅包含字母、数字或下划线', valid: accountNamePattern.test(value) },
  ]
}

export function passwordRules(password: string) {
  const length = characterLength(password)
  return [
    { text: '长度为 8-64 个字符', valid: length >= PASSWORD_MIN_LENGTH && length <= PASSWORD_MAX_LENGTH },
    { text: '同时包含字母和数字', valid: /\p{L}/u.test(password) && /\p{N}/u.test(password) },
    { text: '不包含空格或其他空白字符', valid: !/\s/u.test(password) },
    { text: '内容未超过安全长度', valid: new TextEncoder().encode(password).length <= 72 },
  ]
}

export function validateUsername(username: string) {
  const failed = usernameRules(username).find((rule) => !rule.valid)
  return failed ? `用户名${failed.text}` : ''
}

export function validateAccountName(accountName: string) {
  const failed = accountNameRules(accountName).find((rule) => !rule.valid)
  return failed ? `账号名${failed.text}` : ''
}

export function validatePassword(password: string) {
  const failed = passwordRules(password).find((rule) => !rule.valid)
  return failed ? `密码${failed.text}` : ''
}
