import api from './index'

export interface PluginItem {
  id: number
  name: string
  display_name: string    // 新增
  version: string
  description: string
  author: string
  binary: string
  port: number
  hooks: string
  config_schema: string
  priority: number         // 新增
  has_db: boolean          // 新增
  status: string           // 放宽为 string，后端可能返回新状态值
  config: string
  pid: number
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

export interface LogsStatusResult {
  log_dir: string
  total_size: number
  file_count: number
  latest_file: string
  latest_mtime: string
  keep_on_uninstall: boolean
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
  delete(id: number, keepLogs?: boolean) {
    const params = keepLogs ? { keep_logs: true } : {}
    return api.delete(`/plugins/${id}`, { params })
  },
  logsStatus(id: number) {
    return api.get(`/plugins/${id}/logs-status`)
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
  // === 阶段五：钩子统一调度引擎 ===
  updatePriority(pluginId: number, priority: number) {
    return api.put(`/plugins/${pluginId}/priority`, { priority })
  },
  getTables(pluginId: number) {
    return api.get(`/plugins/${pluginId}/tables`)
  },
  deleteWithTables(id: number, keepLogs?: boolean, dropTables?: boolean) {
    const params: any = {}
    if (keepLogs) params.keep_logs = true
    if (dropTables) params.drop_tables = true
    return api.delete(`/plugins/${id}`, { params })
  },
}

export interface HookItem {
  id: number
  name: string
  hook_type: string
  description: string
  params_schema: string
  timeout: number
  enabled: boolean
  updated_at: string
}

export const hookApi = {
  list() {
    return api.get('/hooks')
  },
  updateEnabled(id: number, enabled: boolean) {
    return api.put(`/hooks/${id}/enabled`, { enabled })
  },
  getPlugins(hookName: string) {
    return api.get(`/hooks/${hookName}/plugins`)
  },
}
