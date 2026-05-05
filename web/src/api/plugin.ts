import api from './index'

export const pluginApi = {
  list() {
    return api.get('/plugins')
  },
  create(data: { name: string; config?: Record<string, unknown> }) {
    return api.post('/plugins', data)
  },
  getById(id: number) {
    return api.get(`/plugins/${id}`)
  },
  updateStatus(id: number, status: string) {
    return api.put(`/plugins/${id}/status`, { status })
  },
  delete(id: number) {
    return api.delete(`/plugins/${id}`)
  },
}
