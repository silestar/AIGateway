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
  status: 'uploaded' | 'installed' | 'running' | 'stopped' | 'unhealthy' | 'error'
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

export interface RegistryEntry {
  name: string
  version: string
  description: string
  author: string
  download_url: string
  homepage?: string
  tags?: string
  min_agw_version?: string
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
  install(uploadId: string) {
    return api.post('/plugins/install', { upload_id: uploadId })
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
  // 注册中心
  registryList() {
    return api.get('/plugins/registry/list')
  },
  registryInstall(name: string, download_url: string) {
    return api.post('/plugins/registry/install', { name, download_url })
  },
  // 权限管理
  getPermissions(pluginId: number) {
    return api.get(`/plugins/${pluginId}/permissions`)
  },
  grantPermission(pluginId: number, permName: string) {
    return api.put(`/plugins/${pluginId}/permissions/${permName}/grant`)
  },
  denyPermission(pluginId: number, permName: string) {
    return api.put(`/plugins/${pluginId}/permissions/${permName}/deny`)
  },
  grantAllPermissions(pluginId: number) {
    return api.post(`/plugins/${pluginId}/permissions/grant-all`)
  },
  denyAllPermissions(pluginId: number) {
    return api.post(`/plugins/${pluginId}/permissions/deny-all`)
  },
}
