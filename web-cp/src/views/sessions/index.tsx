import { Main } from '@/components/layout/main'
import { Badge } from '@/components/ui/badge'
import { useState } from 'react'
import { Activity, Loader2 } from 'lucide-react'
import { SessionsTable } from './components/sessions-table'
import { SessionFilters } from './components/sessions-filters'
import { useActiveSessionsQuery } from '@/hooks/session.hooks'


export default function Sessions() {
  const [filterUser, setFilterUser] = useState('')
  const [filterGateway, setFilterGateway] = useState('')
  const { data: sessions, isLoading, isError } = useActiveSessionsQuery({
    email: filterUser || undefined,
    gateway: filterGateway || undefined,
  })

  const users = !isLoading && sessions ? [...new Set(sessions.map((session) => session.user_email))] : []
  const gateways = !isLoading && sessions ? [...new Set(sessions.map((session) => session.gateway_name))] : []

  console.log(users, gateways)


  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Sessions actives</h2>
          <div className='flex items-center gap-2 mt-1'>
            <Badge variant='secondary' className='text-sm'><Activity className='h-3 w-3 mr-1' />
            {sessions?.length ?? 0} active(s)
            </Badge>
          </div>
        </div>
      </div>
      <div className='flex flex-wrap items-center gap-4 mb-4'>
        <SessionFilters
          onFilterChange={(type, value) => {
            if (type === 'user') setFilterUser(value === 'all' ? '' : value)
            if (type === 'gateway') setFilterGateway(value === 'all' ? '' : value)
          }}
          users={users}
          gateways={gateways}
        />
      </div>
      {isError ?
        <p className='text-destructive'>Impossible de charger les sessions actives.</p> :
        isLoading ?
          <div className='flex items-center justify-center h-64'>
            <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
          </div> :
          <SessionsTable sessions={sessions ?? []} isLoading={isLoading} />
      }
      <div className='mt-4 text-sm text-muted-foreground space-y-1'>
        <p>Utilisez le bouton × pour révoquer une session.</p>
      </div>
    </Main>
  )
}
