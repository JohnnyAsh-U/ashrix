export interface PolicySubject {
  type: string
  value: string
}

export interface PolicyResource {
  type: string
  value: string
}

export interface TimeRule {
  day: string
  start: string
  end: string
}

export interface PolicyConditions {
  mfa?: { required: boolean; min_level?: string }
  device?: { postures: string[] }
  network?: {
    allowed_cidrs?: string[]
    blocked_cidrs?: string[]
    allowed_countries?: string[]
    blocked_countries?: string[]
    block_tor?: boolean
  }
  time?: {
    schedule_name: string
    timezone?: string
    rules?: TimeRule[]
  }
}

export interface PolicyApiResponse {
  items: PolicyResponse[]
}

export interface PolicyResponse {
  id: string
  org_id: string
  name: string
  description: string
  effect: 'ALLOW' | 'DENY'
  priority: number
  enabled: boolean
  version: number
  sequence: number
  created_by: string
  created_at: string
  updated_at: string
  subjects: PolicySubject[]
  resources: PolicyResource[]
  conditions: PolicyConditions
}

export interface CreatePolicyRequest {
  name: string
  description?: string
  effect: 'ALLOW' | 'DENY'
  priority: number
  subjects: PolicySubject[]
  resources: PolicyResource[]
  conditions: PolicyConditions
}

export interface UpdatePolicyRequest extends Partial<CreatePolicyRequest> {
  enabled?: boolean
}