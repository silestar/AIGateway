import api from './index'
import type { ListResult } from './keys'

export interface Channel {
  id: number
  name: string
  type: string
  base_url: string
  status: string
  weight: number
  max_rpm: number
  max_tpm: number
  max_daily_requests: number
  test_model: string
  last_test_latency: number
  last_tested_at: string | null
  created_at: string
  updated_at: string
}

export interface ChannelListItem extends Channel {
  active_account_count: number
  total_account_count: number
  groups: GroupInfo[]
}

export interface GroupInfo {
  id: number
  name: string
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

export interface TestResult {
  success: boolean
  latency: number
  error?: string
  model: string
}

export interface BatchTestResultItem {
  model: string
  success: boolean
  latency: number
  error?: string
  status?: number
  testing?: boolean
}

export interface ChannelTypeInfo {
  type: string
  name: string
  is_plugin: boolean
  base_url?: string
  description?: string
}

export const channelApi = {
  list(params?: {
    page?: number
    page_size?: number
    status?: string
    type?: string
    search?: string
    sort_by?: string
    sort_order?: string
  }) {
    return api.get<ListResult<ChannelListItem>>('/channels', { params })
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
  update(id: number, data: {
    name?: string
    base_url?: string
    weight?: number
    max_rpm?: number
    max_tpm?: number
    max_daily_requests?: number
    test_model?: string
  }) {
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
  // 新增接口
  testChannel(id: number) {
    return api.post<{ data: TestResult }>(`/channels/${id}/test`)
  },
  batchTestModels(id: number, models: string[]) {
    return api.post<{ data: BatchTestResultItem[] }>(`/channels/${id}/test-models`, { models })
  },
  updateTestModel(id: number, testModel: string) {
    return api.put(`/channels/${id}/test-model`, { test_model: testModel })
  },
  copyChannel(id: number) {
    return api.post<{ data: Channel; message: string }>(`/channels/${id}/copy`)
  },
  // 渠道类型（内置 + 插件注册的）
  listChannelTypes() {
    return api.get<{ data: ChannelTypeInfo[] }>('/plugins/channel-types')
  },
  // 获取所有渠道已配置的自定义模型名（display != actual，用于自动补全）
  getCustomModelNames() {
    return api.get<{ data: string[] }>('/channels/custom-model-names')
  },
}
