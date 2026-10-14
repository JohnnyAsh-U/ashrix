import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useState } from 'react'
import { FileJson, FileSpreadsheet, Filter, Loader2, Search } from 'lucide-react'
import { toast } from 'sonner'
import { useAccessLogsQuery } from '@/hooks/log.hooks'
import { AccessLogTable } from './components/access-log-table'
import type { AccessLogApiResponse, AccessLogEntry } from '@/types/access-log'

const toEntry = (log: AccessLogApiResponse): AccessLogEntry => {
  const date = new Date(log.created_at)
  const method = log.method?.toUpperCase()
  return {
    id: log.id,
    time: date.toLocaleTimeString('fr-FR'),
    user: log.user_email || 'unknown',
    userName: null,
    app: log.app_name || log.app_id || 'Inconnue',
    appHostname: 'N/D',
    method: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].includes(method || '') ? method as AccessLogEntry['method'] : null,
    path: log.path || null,
    status: log.status ?? null,
    latency: log.latency_ms === undefined ? null : `${log.latency_ms} ms`,
    result: log.result.toLowerCase() === 'allowed' ? 'allowed' : 'denied',
    reason: log.deny_reason,
    ip: log.ip || 'N/D',
    gateway: log.gateway_name || log.gateway_id || 'Inconnue',
    sessionId: null,
    timestamp: log.created_at,
    groups: [],
  }
}

export default function AccessLog() {
  const [user, setUser] = useState('all')
  const [app, setApp] = useState('all')
  const [result, setResult] = useState('all')
  const [path, setPath] = useState('')
  const { data, isLoading, isError } = useAccessLogsQuery({
    user_email: user === 'all' ? undefined : user,
    result: result === 'all' ? undefined : result,
    page: 1,
    limit: 50,
  })
  const logs = (data ?? []).map(toEntry)
  const filteredLogs = logs.filter((log) =>
    (app === 'all' || log.app === app) && (!path || log.path?.toLowerCase().includes(path.toLowerCase()))
  )

  const exportLogs = (format: 'json' | 'csv') => {
    const content = format === 'json'
      ? JSON.stringify(filteredLogs, null, 2)
      : ['Time,User,App,Method,Status,Latency,Result', ...filteredLogs.map((log) => [log.time, log.user, log.app, log.method || '', log.status || '', log.latency || '', log.result].join(','))].join('\n')
    const blob = new Blob([content], { type: format === 'json' ? 'application/json' : 'text/csv' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `journaux-acces-${new Date().toISOString().split('T')[0]}.${format}`
    link.click()
    URL.revokeObjectURL(url)
    toast.success(`Export ${format.toUpperCase()} terminé`)
  }

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Journaux d’accès</h2>
          <p className='text-muted-foreground'>Chaque requête proxy avec recherche et filtres</p>
        </div>
        <div className='flex items-center space-x-2'>
          <Button variant='outline' onClick={() => exportLogs('json')}><FileJson className='h-4 w-4 mr-2' />Exporter JSON</Button>
          <Button variant='outline' onClick={() => exportLogs('csv')}><FileSpreadsheet className='h-4 w-4 mr-2' />Exporter CSV</Button>
        </div>
      </div>
      <div className='flex flex-wrap items-center gap-4 mb-4'>
        <div className='flex items-center gap-2'>
          <Filter className='h-4 w-4 text-muted-foreground' />
          <Select value={user} onValueChange={setUser}><SelectTrigger className='w-48'><SelectValue placeholder='Utilisateur' /></SelectTrigger><SelectContent><SelectItem value='all'>Tous les utilisateurs</SelectItem>{[...new Set(logs.map((log) => log.user))].map((value) => <SelectItem key={value} value={value}>{value}</SelectItem>)}</SelectContent></Select>
          <Select value={app} onValueChange={setApp}><SelectTrigger className='w-40'><SelectValue placeholder='Application' /></SelectTrigger><SelectContent><SelectItem value='all'>Toutes les applications</SelectItem>{[...new Set(logs.map((log) => log.app))].map((value) => <SelectItem key={value} value={value}>{value}</SelectItem>)}</SelectContent></Select>
          <Select value={result} onValueChange={setResult}><SelectTrigger className='w-36'><SelectValue placeholder='Résultat' /></SelectTrigger><SelectContent><SelectItem value='all'>Tous les résultats</SelectItem><SelectItem value='allowed'>Autorisé</SelectItem><SelectItem value='denied'>Refusé</SelectItem></SelectContent></Select>
        </div>
        <div className='relative flex-1 max-w-xs'><Search className='absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground' /><Input placeholder='Rechercher un chemin…' value={path} onChange={(event) => setPath(event.target.value)} className='pl-10' /></div>
      </div>
      {isError ? <p className='text-destructive'>Impossible de charger les journaux d’accès.</p> : isLoading ? <div className='flex items-center justify-center h-64'><Loader2 className='h-8 w-8 animate-spin text-muted-foreground' /></div> : <AccessLogTable logs={filteredLogs} isLoading={isLoading} />}
    </Main>
  )
}
