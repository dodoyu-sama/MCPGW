import axios from 'axios'

const token = localStorage.getItem('mcpgw_token') || 'change-me'

export const api = axios.create({
  baseURL: '/api',
  headers: { Authorization: `Bearer ${token}` },
})

export interface CallRecord {
  id: number
  client_id: string
  tool_name: string
  params: string
  result: string
  error_msg: string
  latency_ms: number
  timestamp: string
}

export interface Stats {
  total_calls: number
  error_count: number
  avg_latency_ms: number
  tool_counts: Record<string, number>
}

export async function fetchLogs(params: Record<string, any> = {}) {
  const { data } = await api.get('/logs', { params })
  return data.records as CallRecord[]
}

export async function fetchStats() {
  const { data } = await api.get('/stats')
  return data as Stats
}

export async function fetchReplay(callId: number) {
  const { data } = await api.post('/replay', { call_id: callId })
  return data
}
