import { api } from './client'
import type { User, UserCreateInput, UserList, UserPatch } from './types'

export function listUsers() {
  return api<UserList>('/api/v1/users')
}

// Pendaftaran akun baru dilakukan admin lewat endpoint register.
export function createUser(input: UserCreateInput) {
  return api<User>('/api/v1/auth/register', { method: 'POST', body: JSON.stringify(input) })
}

export function updateUser(id: string, patch: UserPatch) {
  return api<User>(`/api/v1/users/${id}`, { method: 'PATCH', body: JSON.stringify(patch) })
}
