import api from './index'

export const statsApi = {
  dashboard(days?: number) {
    return api.get('/stats/dashboard', { params: days ? { days } : undefined })
  },
  tokenStats(days?: number) {
    return api.get('/stats/token-stats', { params: days ? { days } : undefined })
  },
  realtime() {
    return api.get('/stats/realtime')
  },
  requests(params?: { start?: string; end?: string; granularity?: string }) {
    return api.get('/stats/requests', { params })
  },
  models() {
    return api.get('/stats/models')
  },
  channels() {
    return api.get('/stats/channels')
  },
  keysStats(keysId: number, params?: { start?: string; end?: string }) {
    return api.get(`/stats/keys/${keysId}`, { params })
  },
  // 兼容旧 consumerStats — 改为 keys-realtime
  keysRealtime(keysId: number) {
    return api.get(`/stats/keys-realtime/${keysId}`)
  },
  channelStats(channelId: number, params?: { start?: string; end?: string }) {
    return api.get(`/stats/channel/${channelId}`, { params })
  },
}
