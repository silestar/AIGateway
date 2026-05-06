import api from './index'
import type { ListResult } from './keys'

export interface Channel {
  id: number
  name: string
  type: string
  base_url: string
  status: string
  weight: number
  created_at: string
  updated_at: string
}

export interface ChannelModel {
  id?: number
  channel_id?: number
  display_model_name: string
  actual_model_name: string
  status: string
}

export interface ModelInfo {
  id: string
  owned_by: string
}

export const channelApi = {
  list(params?: { page?: number; page_size?: number; status?: string; type?: string }) {
    return api.get<ListResult<Channel>>('/channels', { params })
  },
  create(data: { name: string; type: string; base_url: string; api_key: string }) {
    return api.post<{ data: Channel }>('/channels', data)
  },
  testConnection(data: { type: string; base_url: string; api_key: string }) {
    return api.post<{ success: boolean; error?: string }>('/channels/test-connection', data)
  },
  getById(id: number) {
    return api.get<{ data: Channel }>(`/channels/${id}`)
  },
  update(id: number, data: { name?: string; base_url?: string; weight?: number }) {
    return api.put(`/channels/${id}`, data)
  },
  updateStatus(id: number, status: string) {
    return api.patch(`/channels/${id}/status`, { status })
  },
  updateWeight(id: number, weight: number) {
    return api.patch(`/channels/${id}/weight`, { weight })
  },
  delete(id: number) {
    return api.delete(`/channels/${id}`)
  },
  fetchModels(id: number, testKey?: string) {
    return api.post<{ data: ModelInfo[] }>(`/channels/${id}/fetch-models`, { test_key: testKey || '' })
  },
  getModelsByChannel(id: number) {
    return api.get<{ data: ChannelModel[] }>(`/channels/${id}/models`)
  },
  saveModels(id: number, models: ChannelModel[]) {
    return api.put(`/channels/${id}/models`, { models })
  },
}
