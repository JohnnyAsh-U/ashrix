// types/admin-user.ts
export type AdminRole = 'Owner' | 'Admin' | 'Auditor'

export interface AdminUser {
  id: string
  name: string
  email: string
  role: AdminRole
  joined: string
  canRemove: boolean
}

export interface AdminInvite {
  email: string
  role: Exclude<AdminRole, 'Owner'>
  invitedBy: string
  invitedAt: string
  expiresAt: string
}