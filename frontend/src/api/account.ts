import { postForm, postJson } from './client'
import type { Account, AccountNameAvailability, AvatarResponse, MessageResponse, RegisterResponse, SearchUsersResponse, TokenResponse } from './types'

export function register(accountName: string, username: string, password: string) {
  return postJson<RegisterResponse>('/account/register', { account_name: accountName, username, password })
}

export function login(accountName: string, password: string) {
  return postJson<TokenResponse>('/account/login', { account_name: accountName, password })
}

export function checkAccountName(accountName: string) {
  return postJson<AccountNameAvailability>('/account/checkAccountName', { account_name: accountName })
}

export function me() {
  return postJson<Account>('/account/me', {}, { authRequired: true })
}

export function logout() {
  return postJson<MessageResponse>('/account/logout', {}, { authRequired: true })
}

export function rename(newUsername: string) {
  return postJson<TokenResponse>('/account/rename', { new_username: newUsername }, { authRequired: true })
}

export function changePassword(oldPassword: string, newPassword: string) {
  return postJson<MessageResponse>('/account/changePassword', {
    old_password: oldPassword,
    new_password: newPassword,
  }, { authRequired: true })
}

export function uploadAvatar(file: File) {
  const body = new FormData()
  body.append('file', file)
  return postForm<AvatarResponse>('/account/avatar', body, { authRequired: true })
}

export function findById(id: number) {
  return postJson<Account>('/account/findByID', { id })
}

export function findByUsername(username: string) {
  return postJson<Account>('/account/findByUsername', { username })
}

export function searchUsers(query: string, limit = 20, offset = 0) {
  return postJson<SearchUsersResponse>('/account/search', { query, limit, offset })
}
