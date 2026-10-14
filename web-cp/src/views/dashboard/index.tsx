import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState, useEffect } from 'react'
import { 
  Loader2,
  ArrowRight,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Shield,
  Users,
  Activity,
  Ban,
  TrendingUp,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import type { DashboardData, Alert, RecentEvent } from '@/types/dashboard'

// Static data
const staticDashboardData: DashboardData = {
  stats: {
    applications: {
      total: 12,
      healthy: 12,
      label: 'Applications'
    },
    activeUsers: {
      count: 247,
      period: 'last 24h',
      label: 'Active Users'
    },
    activeSessions: {
      count: 61,
      period: 'right now',
      label: 'Active Sessions'
    },
    denied: {
      count: 18,
      period: 'today',
      label: 'Denied'
    }
  },
  alerts: [
    {
      id: '1',
      type: 'warning',
      title: 'Connector "erp-connector" disconnected',
      message: 'ERP is unreachable. Users will see offline page.',
      time: '6 min ago',
      action: 'Go to connector →',
      actionUrl: '/connectors'
    }
  ],
  gateways: [
    {
      id: '1',
      name: 'London HQ',
      status: 'healthy',
      latency: '9ms',
      apps: 8,
      sessions: 47
    },
    {
      id: '2',
      name: 'Manchester',
      status: 'healthy',
      latency: '18ms',
      apps: 4,
      sessions: 14
    }
  ],
  applications: [
    { name: 'CRM', status: 'up' },
    { name: 'Grafana', status: 'up' },
    { name: 'GitLab', status: 'up' },
    { name: 'HR Portal', status: 'up' },
    { name: 'Wiki', status: 'up' },
    { name: 'Jenkins', status: 'up' },
    { name: 'ERP', status: 'down' },
    { name: 'Jira', status: 'up' }
  ],
  additionalAppsCount: 8,
  traffic: {
    totalRequests: 8420,
    deniedRequests: 18,
    deniedPercentage: 0.2,
    chartData: [
      { hour: '00:00', allowed: 245, denied: 2 },
      { hour: '03:00', allowed: 180, denied: 1 },
      { hour: '06:00', allowed: 320, denied: 3 },
      { hour: '09:00', allowed: 540, denied: 5 },
      { hour: '12:00', allowed: 620, denied: 4 },
      { hour: '15:00', allowed: 580, denied: 3 },
      { hour: '18:00', allowed: 450, denied: 2 },
      { hour: '21:00', allowed: 300, denied: 1 }
    ]
  },
  recentEvents: [
    { time: '09:14', user: 'alice@', app: 'CRM', result: 'allowed' },
    { time: '09:13', user: 'bob@', app: 'Finance', result: 'allowed' },
    { time: '09:11', user: 'carol@', app: 'DevEnv', result: 'denied' },
    { time: '09:11', user: 'carol@', app: 'DevEnv', result: 'denied' },
    { time: '09:09', user: 'dave@', app: 'GitLab', result: 'allowed' },
    { time: '09:08', user: 'alice@', app: 'Grafana', result: 'allowed' }
  ]
}

export default function Dashboard() {
  const [data] = useState<DashboardData>(staticDashboardData)
  const [isLoading] = useState(false)
  const [currentTime, setCurrentTime] = useState(new Date())
  const [lastUpdated, setLastUpdated] = useState('just now')

  // Simulate live updates
  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentTime(new Date())
      setLastUpdated('just now')
    }, 1000)

    return () => clearInterval(timer)
  }, [])

  // Simulate data refresh every 60 seconds
  useEffect(() => {
    const refreshTimer = setInterval(() => {
      setLastUpdated('just now')
      console.log('Refreshing dashboard data...')
    }, 60000)

    return () => clearInterval(refreshTimer)
  }, [])

  const formatDate = (date: Date) => {
    const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
    const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']
    
    return `${days[date.getDay()]} ${date.getDate()} ${months[date.getMonth()]} · ${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
  }

  return (
    <Main>
      {/* Header */}
      <div className='flex flex-wrap items-end justify-between gap-2 mb-6'>
        <div>
          <h2 className='text-3xl font-bold tracking-tight'>Good morning, James.</h2>
          <p className='text-lg text-muted-foreground mt-1'>Acme Corp</p>
        </div>
        <div className='text-right'>
          <p className='text-sm text-muted-foreground'>
            {formatDate(currentTime)}
          </p>
          <p className='text-xs text-muted-foreground'>
            Last updated: {lastUpdated}
          </p>
        </div>
      </div>

      {isLoading ? (
        <div className='flex items-center justify-center h-64'>
          <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
        </div>
      ) : (
        <>
          {/* Stats Cards */}
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6'>
            <StatCard
              icon={<Shield className='h-5 w-5 text-blue-600' />}
              label={data.stats.applications.label}
              value={data.stats.applications.total}
              subtext={`${data.stats.applications.healthy} healthy`}
            />
            <StatCard
              icon={<Users className='h-5 w-5 text-green-600' />}
              label={data.stats.activeUsers.label}
              value={data.stats.activeUsers.count}
              subtext={data.stats.activeUsers.period}
            />
            <StatCard
              icon={<Activity className='h-5 w-5 text-purple-600' />}
              label={data.stats.activeSessions.label}
              value={data.stats.activeSessions.count}
              subtext={data.stats.activeSessions.period}
            />
            <StatCard
              icon={<Ban className='h-5 w-5 text-red-600' />}
              label={data.stats.denied.label}
              value={data.stats.denied.count}
              subtext={data.stats.denied.period}
            />
          </div>

          {/* Alerts Section */}
          <div className='mb-6'>
            <div className='flex items-center justify-between mb-3'>
              <h3 className='text-lg font-semibold'>ALERTS</h3>
              <Button variant='ghost' size='sm'>
                View all <ArrowRight className='h-4 w-4 ml-1' />
              </Button>
            </div>

            {data.alerts.length > 0 ? (
              <div className='space-y-2'>
                {data.alerts.map((alert :any) => (
                  <AlertBanner key={alert.id} alert={alert} />
                ))}
              </div>
            ) : (
              <div className='bg-green-50 border border-green-200 rounded-lg p-4'>
                <div className='flex items-center gap-2 text-green-700'>
                  <CheckCircle className='h-5 w-5' />
                  <p className='text-sm font-medium'>No alerts. Everything looks good.</p>
                </div>
              </div>
            )}

            {data.alerts.length > 0 && (
              <p className='text-sm text-muted-foreground mt-2'>
                No other alerts.
              </p>
            )}
          </div>

          {/* Gateways Section */}
          <div className='mb-6'>
            <div className='flex items-center justify-between mb-3'>
              <h3 className='text-lg font-semibold'>GATEWAYS</h3>
              <Button variant='ghost' size='sm'>
                Manage <ArrowRight className='h-4 w-4 ml-1' />
              </Button>
            </div>

            <div className='border rounded-lg overflow-hidden'>
              {data.gateways.map((gateway:any, idx: number) => (
                <div 
                  key={gateway.id}
                  className={`flex items-center justify-between p-4 ${
                    idx !== data.gateways.length - 1 ? 'border-b' : ''
                  }`}
                >
                  <div className='flex items-center gap-3'>
                    <div className='relative'>
                      <div className='w-3 h-3 bg-green-500 rounded-full'></div>
                      <div className='absolute inset-0 w-3 h-3 bg-green-500 rounded-full animate-ping opacity-75'></div>
                    </div>
                    <span className='font-medium'>{gateway.name}</span>
                  </div>
                  <div className='flex items-center gap-6 text-sm'>
                    <span className='inline-flex items-center gap-1 text-green-600'>
                      <CheckCircle className='h-4 w-4' /> {gateway.status}
                    </span>
                    <span>{gateway.latency}</span>
                    <span>{gateway.apps} apps</span>
                    <span>{gateway.sessions} sessions</span>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Applications Section */}
          <div className='mb-6'>
            <div className='flex items-center justify-between mb-3'>
              <h3 className='text-lg font-semibold'>APPLICATIONS</h3>
              <Button variant='ghost' size='sm'>
                Manage <ArrowRight className='h-4 w-4 ml-1' />
              </Button>
            </div>

            <div className='border rounded-lg p-4'>
              <div className='grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4'>
                {data.applications.map((app:any) => (
                  <div key={app.name} className='flex items-center gap-2'>
                    {app.status === 'up' ? (
                      <CheckCircle className='h-5 w-5 text-green-600' />
                    ) : (
                      <XCircle className='h-5 w-5 text-red-600' />
                    )}
                    <span className='font-medium'>{app.name}</span>
                  </div>
                ))}
              </div>
              <p className='text-sm text-muted-foreground mt-3'>
                ({data.additionalAppsCount} more)
              </p>
            </div>

            {data.applications.some((app: any) => app.status === 'down') && (
              <div className='flex items-center gap-2 mt-2 text-sm text-red-600'>
                <XCircle className='h-4 w-4' />
                <span>ERP is offline.</span>
                <Button variant='link' size='sm' className='text-red-600 p-0'>
                  Investigate <ArrowRight className='h-4 w-4 ml-1' />
                </Button>
              </div>
            )}
          </div>

          {/* Traffic and Recent Events */}
          <div className='grid grid-cols-1 lg:grid-cols-2 gap-6'>
            {/* Traffic Chart */}
            <div>
              <div className='flex items-center justify-between mb-3'>
                <h3 className='text-lg font-semibold'>TRAFFIC (last 24h)</h3>
                <Badge variant='outline'>
                  <TrendingUp className='h-3 w-3 mr-1' />
                  {data.traffic.totalRequests.toLocaleString()} req
                </Badge>
              </div>
              <TrafficChart data={data.traffic.chartData} />
              <div className='flex items-center justify-between text-sm text-muted-foreground mt-3'>
                <span>Today: {data.traffic.totalRequests.toLocaleString()} req</span>
                <span className='text-red-600'>
                  Denied: {data.traffic.deniedRequests} ({data.traffic.deniedPercentage}%)
                </span>
              </div>
            </div>

            {/* Recent Access Events */}
            <div>
              <div className='flex items-center justify-between mb-3'>
                <h3 className='text-lg font-semibold'>RECENT ACCESS EVENTS</h3>
                <Badge variant='secondary' className='animate-pulse'>
                  <Activity className='h-3 w-3 mr-1' /> live
                </Badge>
              </div>
              <RecentEventsList events={data.recentEvents} />
            </div>
          </div>
        </>
      )}
    </Main>
  )
}

// Stat Card Component
function StatCard({ 
  icon, 
  label, 
  value, 
  subtext 
}: { 
  icon: React.ReactNode
  label: string
  value: number
  subtext: string
}) {
  return (
    <Card>
      <CardContent className='p-4'>
        <div className='flex items-center gap-3 mb-2'>
          {icon}
          <span className='text-sm text-muted-foreground'>{label}</span>
        </div>
        <p className='text-3xl font-bold'>{value}</p>
        <p className='text-sm text-muted-foreground mt-1'>{subtext}</p>
      </CardContent>
    </Card>
  )
}

// Alert Banner Component
function AlertBanner({ alert }: { alert: Alert }) {
  const getAlertStyles = (type: string) => {
    switch (type) {
      case 'error':
        return 'bg-red-50 border-red-200'
      case 'warning':
        return 'bg-orange-50 border-orange-200'
      case 'info':
        return 'bg-yellow-50 border-yellow-200'
      default:
        return 'bg-gray-50 border-gray-200'
    }
  }

  return (
    <div className={`border rounded-lg p-4 ${getAlertStyles(alert.type)}`}>
      <div className='flex items-start gap-3'>
        <AlertTriangle className='h-5 w-5 text-orange-500 mt-0.5' />
        <div className='flex-1'>
          <div className='flex items-center gap-2'>
            <p className='text-sm font-medium'>{alert.title}</p>
            <span className='text-xs text-muted-foreground'>{alert.time}</span>
          </div>
          <p className='text-sm text-muted-foreground mt-1'>{alert.message}</p>
          <Button variant='link' size='sm' className='p-0 mt-2'>
            {alert.action} <ArrowRight className='h-4 w-4 ml-1' />
          </Button>
        </div>
      </div>
    </div>
  )
}

// Traffic Chart Component (Simple SVG-based)
function TrafficChart({ data }: { data: { hour: string; allowed: number; denied: number }[] }) {
  const maxValue = Math.max(...data.map(d => d.allowed + d.denied))
  const chartHeight = 200
  const chartWidth = 400
  const padding = 40

  const points = data.map((d, i) => {
    const x = padding + (i * (chartWidth - padding * 2) / (data.length - 1))
    const y = chartHeight - padding - ((d.allowed + d.denied) / maxValue * (chartHeight - padding * 2))
    return { x, y, ...d }
  })

  const linePath = points.map((p, i) => 
    `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`
  ).join(' ')

  return (
    <div className='border rounded-lg p-4'>
      <svg width='100%' height={chartHeight} viewBox={`0 0 ${chartWidth} ${chartHeight}`}>
        {/* Grid lines */}
        {[0, 1, 2, 3].map((i) => {
          const y = padding + (i * (chartHeight - padding * 2) / 3)
          return (
            <line
              key={i}
              x1={padding}
              y1={y}
              x2={chartWidth - padding}
              y2={y}
              stroke='#e5e7eb'
              strokeWidth='1'
            />
          )
        })}

        {/* Area under line */}
        <path
          d={`${linePath} L ${points[points.length - 1].x} ${chartHeight - padding} L ${points[0].x} ${chartHeight - padding} Z`}
          fill='#3b82f6'
          fillOpacity='0.1'
        />

        {/* Line */}
        <path
          d={linePath}
          fill='none'
          stroke='#3b82f6'
          strokeWidth='2'
        />

        {/* Data points */}
        {points.map((p, i) => (
          <circle
            key={i}
            cx={p.x}
            cy={p.y}
            r='3'
            fill='#3b82f6'
          />
        ))}

        {/* X-axis labels */}
        {points.filter((_, i) => i % 2 === 0).map((p, i) => (
          <text
            key={i}
            x={p.x}
            y={chartHeight - 10}
            textAnchor='middle'
            className='text-xs'
            fill='#6b7280'
          >
            {p.hour}
          </text>
        ))}
      </svg>
    </div>
  )
}

// Recent Events List Component
function RecentEventsList({ events }: { events: RecentEvent[] }) {
  return (
    <div className='border rounded-lg overflow-hidden'>
      {events.map((event, idx) => (
        <div 
          key={idx}
          className={`flex items-center justify-between p-3 ${
            idx !== events.length - 1 ? 'border-b' : ''
          }`}
        >
          <div className='flex items-center gap-3'>
            <span className='text-xs font-mono text-muted-foreground'>{event.time}</span>
            <span className='text-sm font-medium'>{event.user}</span>
            <span className='text-sm'>{event.app}</span>
          </div>
          {event.result === 'allowed' ? (
            <span className='inline-flex items-center gap-1 text-green-600 text-sm'>
              <CheckCircle className='h-4 w-4' /> allowed
            </span>
          ) : (
            <span className='inline-flex items-center gap-1 text-red-600 text-sm'>
              <XCircle className='h-4 w-4' /> denied
            </span>
          )}
        </div>
      ))}
    </div>
  )
}