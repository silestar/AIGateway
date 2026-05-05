import api from './index'

export interface Account {
  id: number
  channel_id: number
  status: string
  priority: number
  api_key_mask: string
  created_at?: string
  updated_at?: string
}

export const accountApi = {
  create(data: { channel_id: number; api_key: string }) {
    return api.post('/accounts', data)
  },
  getById(id: number) {
    return api.get<{ data: Account }>(`/accounts/${id}`)
  },
  listByChannel(channelId: number) {
    return api.get<{ data: Account[] }>(`/accounts/channel/${channelId}`)
  },
  updatePriority(id: number, priority: number) {
    return api.put(`/accounts/${id}/priority`, { priority })
  },
  updateStatus(id: number, status: string) {
    return api.put(`/accounts/${id}/status`, { status })
  },
  revealKey(id: number) {
    return api.post<{ data: { id: number; api_key: string } }>(`/accounts/${id}/reveal-key`)
  },
  delete(id: number) {
    return api.delete(`/accounts/${id}`)
  },
}
