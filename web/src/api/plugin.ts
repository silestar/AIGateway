import api from './index'

export interface PluginItem {
  id: number
  name: string
  version: string
  description: string
  author: string
  binary: string
  port: number
  hooks: string
  config_schema: string
  status: 'installed' | 'running' | 'stopped' | 'unhealthy' | 'error'
  config: string
  pid: number
  created_at: string
  updated_at: string
}

export interface ChannelPluginConfig {
  id: number
  channel_id: number
  plugin_id: number
  config: string
  created_at: string
  updated_at: string
}

export const pluginApi = {
  list() {
    return api.get('/plugins')
  },
  upload(formData: FormData) {
    return api.post('/plugins/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  getById(id: number) {
    return api.get(`/plugins/${id}`)
  },
  updateStatus(id: number, action: 'start' | 'stop') {
    return api.put(`/plugins/${id}/status`, { action })
  },
  updateConfig(id: number, config: string) {
    return api.put(`/plugins/${id}/config`, { config })
  },
  delete(id: number) {
    return api.delete(`/plugins/${id}`)
  },
  // 渠道级插件配置
  listChannelConfigs(pluginId: number) {
    return api.get(`/plugins/${pluginId}/channel-configs`)
  },
  setChannelConfig(pluginId: number, channelId: number, config: string) {
    return api.put(`/plugins/${pluginId}/channel-configs/${channelId}`, { config })
  },
  deleteChannelConfig(pluginId: number, channelId: number) {
    return api.delete(`/plugins/${pluginId}/channel-configs/${channelId}`)
  },
}
