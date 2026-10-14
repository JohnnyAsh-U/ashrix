// types/user.ts
export interface PermittedApp {
  name: string
  access: 'allowed' | 'denied'
  via: string
}

export interface Session {
  id: string
  device: string
  ip: string
  location: string
  gateway: string
  started: string
  ttl: string
}

export interface AccessLog {
  date: string
  app: string
  result: 'allowed' | 'denied'
  device: string
}

export interface User {
  id: string
  name: string
  email: string
  groups: string[]
  lastSeen: string
  status: 'online' | 'offline'
  permittedApps: PermittedApp[]
  activeSessions: Session[]
  recentAccess: AccessLog[]
}

export interface Group {
  id: string
  name: string
  members: number
  description: string
}