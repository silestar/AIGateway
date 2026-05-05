import api from './index'

export const authApi = {
  login(token: string) {
    return api.post('/auth/login', { token })
  },
}
