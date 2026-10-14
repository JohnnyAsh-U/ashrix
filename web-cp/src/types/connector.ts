// types/connector.ts
export interface EnrollmentInfo {
  date: string
  by: string
}

export interface ConnectorApp {
  name: string
  hostname: string
}

export interface CreateConnectorRequest {
  name: string
  gateway_id: string
  secondary_gateway_id?: string
  open_sock: boolean
}

export interface ConnectorInstruction {
  download_cmd: string
  start_cmd: string
}

export interface UpdateConnectorRequest extends CreateConnectorRequest {
  id: string
}

export interface RevokeConnectorCertificateRequest {
  id: string
  revoke_reason?: string
}

export interface RevokeConnectorRequest {
  id: string
  revoke_reason: string
}

export interface RenewConnectorCertificateRequest {
  connector_id: string
  signature: string
  csr_pem: string
  timestamp: string
}

export interface ConnectorCertificateResponse {
  connector_id: string
  certificate: string
  trust_bundle: string
  open_sock: boolean
  expires_at: string
}

export interface ConnectorApiResponse {
  id: string
  name: string
  org_id: string
  gateway_id: string
  gateway_name: string
  secondary_gateway_id?: string
  secondary_gateway_name?: string
  version: string
  last_seen?: string
  token?: string
  status: string
  open_sock: boolean
  active_stream: number
  apps: Array<{
    id: string
    name: string
    subdomain: string
    health_status: string
  }>
  instruction: ConnectorInstruction
  created_at: string
  enrolled_at?: string
  revoked_at?: string
  download_url?: string
  download_cmd?: string
  enroll_cmd?: string
  start_cmd?: string
}