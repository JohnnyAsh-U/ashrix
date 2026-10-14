// types/dashboard.ts
export interface Alert {
  id: string
  type: 'error' | 'warning' | 'info'
  title: string
  message: string
  time: string
  action: string
  actionUrl: string
}

export interface GatewayStatus {
  id: string
  name: string
  status: 'healthy' | 'unhealthy'
  latency: string
  apps: number
  sessions: number
}

export interface RecentEvent {
  time: string
  user: string
  app: string
  result: 'allowed' | 'denied'
}

export interface TrafficData {
  hour: string
  allowed: number
  denied: number
}

export interface DashboardData {
  stats: {
    applications: {
      total: number
      healthy: number
      label: string
    }
    activeUsers: {
      count: number
      period: string
      label: string
    }
    activeSessions: {
      count: number
      period: string
      label: string
    }
    denied: {
      count: number
      period: string
      label: string
    }
  }
  alerts: Alert[]
  gateways: GatewayStatus[]
  applications: {
    name: string
    status: 'up' | 'down'
  }[]
  additionalAppsCount: number
  traffic: {
    totalRequests: number
    deniedRequests: number
    deniedPercentage: number
    chartData: TrafficData[]
  }
  recentEvents: RecentEvent[]
}