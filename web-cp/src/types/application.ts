// types/application.ts
export interface Policy {
  id: string
  name: string
  action: 'allow' | 'deny'
  priority: number
}

export interface CreateApplicationRequest {
  name: string
  subdomain: string
  upstream: string
  protocol: 'http' | 'https' | 'tcp' | 'ssh'
  is_public: boolean
  enable_security_headers?: boolean
  connector_id: string
  sock_pass?: string
  check_health: boolean
  check_interval: number
  health_endpoint?: string
}

export type UpdateApplicationRequest = CreateApplicationRequest & { id: string }

export interface ApplicationApiResponse {
  id: string
  org_id: string
  connector_id?: string
  connector_name?: string
  gateway_name?: string
  name: string
  subdomain: string
  upstream: string
  protocol: string
  is_public: boolean
  enable_security_headers: boolean
  sock_pass: string
  check_health: boolean
  check_interval: number
  health_endpoint: string
  health_status: string
  number_of_traffic_today: number
  last_seen?: string
  policies: Array<{
    id: string
    name: string
    effect: string
    priority: number
  }>
  number_of_users: number
  number_of_groups: number
  created_at: string
}