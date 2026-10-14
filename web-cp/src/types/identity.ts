export interface IdentityConfig {
  id: string
  org_id: string
  name: string
  provider_type: string
  client_id: string
  issuer_url: string
  scopes: string[]
  email_claim: string
  name_claim: string
  groups_claim: string
  is_active: boolean
  is_verified: boolean
  created_at: string
}

export interface CreateIdentityConfigRequest {
  type: string
  name: string
  issuer_url: string
  client_id: string
  client_secret: string
  scopes: string[]
  email_claim: string
  name_claim: string
  groups_claim: string
  extra_config?: Record<string, string>
}