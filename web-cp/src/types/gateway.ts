// types/gateway.ts

export interface CreateGatewayRequest {
  name: string
  ip_address: string
  public_url: string
  log_to_cp: boolean
}

export interface GatewayInstruction {
  download_url: string
  download_cmd: string
  enroll_cmd: string
  start_cmd: string
}

export interface UpdateGatewayRequest extends CreateGatewayRequest {
  id: string
}


export interface RevokeGatewayRequest {
  id: string
  revoke_reason: string
}


export interface GatewayApiResponse {
  id: string
  name: string
  org_id: string
  status: string
  version: string
  public_url: string
  ip_address: string
  uptime: number
  last_heartbeat?: string
  deployment_type: string
  log_to_cp: boolean
  number_of_session_active: number
  number_of_apps: number
  enrolled_at: string
  policy_version: string,
    current_policy_version: string,
  apps: Array<{
    id: string
    name: string
    subdomain: string
    health_status: string
  }>
  token?: string
  download_url?: string
  download_cmd?: string
  enroll_cmd?: string
  start_cmd?: string
  instruction: GatewayInstruction
  certificate_expires_at?: string
  certificate_issued_at?: string
  revoked_at?: string | null
  revoke_reason?: string | null
  created_at?: string
}