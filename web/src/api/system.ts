import api from './index'

export const systemApi = {
  info() {
    return api.get('/system/info')
  },
  getConfig() {
    return api.get('/system/config')
  },
  updateConfig(data: Record<string, unknown>) {
    return api.put('/system/config', data)
  },
  getChannelHealth() {
    return api.get<{ data: { channels: any[]; cooling_accounts: any[] } }>('/system/monitor/channel-health')
  },
  flushCache() {
    return api.post('/system/cache/flush')
  },
}

// 系统日志 API
export interface SystemLogQuery {
  date: string // YYYY-MM-DD
  level?: string // 逗号分隔，如 info,warn
  keyword?: string
  trace_id?: string
  page?: number
  page_size?: number
  since?: string // RFC3339 时间戳
  source?: string // "" / "system" / 插件名
}

export interface SystemLogEntry {
  ts?: string
  level?: string
  msg?: string
  caller?: string
  trace_id?: string
  source?: string // 插件日志来源
  module?: string // 可读模块名（后端 callerToModule 转换）
  message?: string // 可读消息描述（后端 msgToReadable 转换）
  method?: string // 请求方法（GET/POST 等）
  path?: string // 请求路径
  [key: string]: unknown
}

export interface PluginLogSource {
  name: string
  has_logs: boolean
  log_dir: string
}

export const systemLogApi = {
  list(params: SystemLogQuery) {
    return api.get('/system/logs', { params })
  },
  dates(source?: string) {
    return api.get('/system/logs/dates', { params: source ? { source } : {} })
  },
  download(date: string, source?: string) {
    return api.get('/system/logs/download', { params: { date, ...(source ? { source } : {}) }, responseType: 'blob' })
  },
  listPlugins() {
    return api.get('/system/logs/plugins')
  },
}