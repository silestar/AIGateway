import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器：添加管理端 Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('agw_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  config.headers['Accept-Language'] = navigator.language.startsWith('zh') ? 'zh-CN' : 'en-US'
  return config
})

// 响应拦截器：统一错误处理
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('agw_token')
      // 跳转登录页（避免重复跳转）
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  },
)

export default api
