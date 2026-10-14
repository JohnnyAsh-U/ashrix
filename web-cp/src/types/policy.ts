// types/policy.ts
export interface PolicyVersion {
  version: number
  date: string
  by: string
  change: string
}

export interface Policy {
  id: string
  ruleName: string
  description?: string
  app: string
  appHostname: string
  who: string
  users?: string[]
  usersCount?: number
  status: 'active' | 'disabled'
  effect?: string
  priority: number
  mfaRequired: boolean
  groups?: string[]
  domains?: string[]
  emails?: string[]
  ipRanges?: string[]
  blockedIPs?: string[]
  version: number
  lastModified: string
  modifiedBy: string
  versions: PolicyVersion[]
  originalPolicy?: any
}