import api from './index'

export interface Keys {
  id: number
  name: string
  status: string
  api_key_prefix?: string
  created_at: string
  updated_at: string
}

export interface KeysCreateResult {
  id: number
  name: string
  status: string
  api_key: string
  created_at: string
}

export interface ListResult<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export const keysApi = {
  list(params?: { page?: number; page_size?: number; status?: string; name?: string }) {
    return api.get<ListResult<Keys>>('/keys', { params })
  },
  create(data: { name: string }) {
    return api.post<{ data: KeysCreateResult }>('/keys', data)
  },
  getById(id: number) {
    return api.get<{ data: Keys }>(`/keys/${id}`)
  },
  update(id: number, data: { name: string }) {
    return api.put(`/keys/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/keys/${id}`)
  },
  updateStatus(id: number, status: string) {
    return api.put(`/keys/${id}/status`, { status })
  },
  resetKey(id: number) {
    return api.post<{ data: { id: number; api_key: string } }>(`/keys/${id}/reset-key`)
  },
  revealKey(id: number) {
    return api.post<{ data: { id: number; api_key: string } }>(`/keys/${id}/reveal-key`)
  },
}