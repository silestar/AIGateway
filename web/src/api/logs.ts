import api from './index'

export interface RequestLogFilter {
  keys_id?: number
  channel_id?: number
  model_name?: string
  status?: string // success | failed
  log_types?: string // consumption,probe,health_check
  keyword?: string
  trace_id?: string
  start?: string
  end?: string
  page?: number
  page_size?: number
}

export interface RetryChainEntry {
  channel_id?: number
  channel_name?: string
  account_id?: number
  account_note?: string
  latency_ms?: number
  status_code?: number
  error?: string
  result?: string
}

export interface RequestLog {
  id: number
  timestamp: string
  keys_id: number
  keys_name?: string
  model_name: string
  channel_id?: number
  channel_name?: string
  account_id?: number
  account_note?: string
  group_name?: string
  retry_chain?: RetryChainEntry[] | string
  is_stream: boolean
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  status_code: number
  error_msg?: string
  latency_ms: number
  upstream_latency_ms: number
  first_token_ms: number
  cost: number
  mapped_model: string
  upstream_model: string
  request_meta?: Record<string, unknown> | string
  response_meta?: Record<string, unknown> | string
  log_type: string // consumption / probe / health_check
  trace_id: string
  client_ip: string
  has_detail: number  // 1=有详细内容文件
}

export interface LogDetailContent {
  trace_id: string
  request: {
    method: string
    path: string
    headers: Record<string, string>
    body?: unknown
  }
  response: {
    status_code: number
    headers: Record<string, string>
    body?: unknown
  }
}

export interface RequestLogStats {
  total_requests: number
  success_requests: number
  failed_requests: number
  avg_latency_ms: number
  total_tokens: number
}

export interface ChannelOption {
  value: number
  label: string
}

export interface KeysOption {
  value: number
  label: string
}

export const requestLogApi = {
  list(params?: RequestLogFilter) {
    return api.get('/logs', { params })
  },
  getById(id: number) {
    return api.get(`/logs/${id}`)
  },
  stats(params?: { start?: string; end?: string; log_types?: string }) {
    return api.get('/logs/stats', { params })
  },
  channels() {
    return api.get('/logs/channels')
  },
  keys() {
    return api.get('/logs/keys')
  },
  getDetail(id: number) {
    return api.get(`/logs/${id}/detail`)
  },
}
