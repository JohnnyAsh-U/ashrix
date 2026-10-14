// types/access-log.ts
export interface AccessLogEntry {
  id: string
  time: string
  user: string
  userName: string | null
  app: string
  appHostname: string
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | null
  path: string | null
  status: number | null
  latency: string | null
  result: 'allowed' | 'denied'
  reason?: string
  ip: string
  gateway: string
  sessionId: string | null
  timestamp: string
  groups: string[]
}

export interface AccessLogApiResponse {
  id: string
  gateway_id?: string
  gateway_name?: string
  app_id?: string
  app_name?: string
  user_email?: string
  method?: string
  path?: string
  status?: number
  latency_ms?: number
  action?: string
  ip?: string
  result: string
  deny_reason?: string
  created_at: string
}

export interface AccessLogQuery {
  user_email?: string
  app?: string
  gateway?: string
  result?: string
  page?: number
  limit?: number
}