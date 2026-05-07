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
}

export interface SystemLogEntry {
  ts?: string
  level?: string
  msg?: string
  caller?: string
  trace_id?: string
  [key: string]: unknown
}

export const systemLogApi = {
  list(params: SystemLogQuery) {
    return api.get('/system/logs', { params })
  },
  dates() {
    return api.get('/system/logs/dates')
  },
  download(date: string) {
    return api.get('/system/logs/download', { params: { date }, responseType: 'blob' })
  },
}
