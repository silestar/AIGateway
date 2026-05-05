import api from './index'

export interface LogFilter {
  consumer_id?: number
  channel_id?: number
  model_name?: string
  status?: string // success | failed
  start?: string
  end?: string
  page?: number
  page_size?: number
}

export const logsApi = {
  list(params?: LogFilter) {
    return api.get('/logs', { params })
  },
  getById(id: number) {
    return api.get(`/logs/${id}`)
  },
}
