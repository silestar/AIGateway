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
  downloadLogs() {
    return api.get('/system/logs/download', { responseType: 'blob' })
  },
}
