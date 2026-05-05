import api from './index'

export const statsApi = {
  dashboard(days?: number) {
    return api.get('/stats/dashboard', { params: days ? { days } : undefined })
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
  consumerStats(consumerId: number, params?: { start?: string; end?: string }) {
    return api.get(`/stats/consumer/${consumerId}`, { params })
  },
  channelStats(channelId: number, params?: { start?: string; end?: string }) {
    return api.get(`/stats/channel/${channelId}`, { params })
  },
}
