import api from './index'

export interface Consumer {
  id: number
  name: string
  status: string
  created_at: string
  updated_at: string
}

export interface ConsumerCreateResult {
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

export const consumerApi = {
  list(params?: { page?: number; page_size?: number; status?: string; name?: string }) {
    return api.get<ListResult<Consumer>>('/consumers', { params })
  },
  create(data: { name: string }) {
    return api.post<{ data: ConsumerCreateResult }>('/consumers', data)
  },
  getById(id: number) {
    return api.get<{ data: Consumer }>(`/consumers/${id}`)
  },
  update(id: number, data: { name: string }) {
    return api.put(`/consumers/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/consumers/${id}`)
  },
  updateStatus(id: number, status: string) {
    return api.put(`/consumers/${id}/status`, { status })
  },
  resetKey(id: number) {
    return api.post<{ data: { id: number; api_key: string } }>(`/consumers/${id}/reset-key`)
  },
  revealKey(id: number) {
    return api.post<{ data: { id: number; api_key: string } }>(`/consumers/${id}/reveal-key`)
  },
}
