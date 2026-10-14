import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useState } from 'react'
import { Loader2, Plus, Search } from 'lucide-react'
import { ApplicationsTable } from './components/applications-table'
import { AddApplicationForm } from './components/add-application-form'
import { PublicAppConfirmation } from './components/public-app-confirmation'
import { useApplicationsQuery } from '@/hooks/application.hooks'
import type { ApplicationApiResponse } from '@/types/application'

type FilterType = 'Tous' | 'HTTP' | 'SSH' | 'TCP' | 'Saines' | 'Hors ligne'


export default function Applications() {
  const [addDrawerOpen, setAddDrawerOpen] = useState(false)
  const [publicConfirmOpen, setPublicConfirmOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [activeFilter, setActiveFilter] = useState<FilterType>('Tous')
  const { data : applications, isLoading, isError } = useApplicationsQuery()

  const filteredApplications = applications ?? [].filter((app : ApplicationApiResponse) => {
    const query = searchQuery.toLowerCase()
    const matchesSearch = app.name.toLowerCase().includes(query) || app.name.toLowerCase().includes(query)
    const matchesFilter = activeFilter === 'Tous' ||
      (activeFilter === 'HTTP' && app.protocol === 'HTTP') ||
      (activeFilter === 'SSH' && app.protocol === 'SSH') ||
      (activeFilter === 'TCP' && app.protocol === 'TCP') ||
      (activeFilter === 'Saines' && app.health_status === 'healthy') ||
      (activeFilter === 'Hors ligne' && app.health_status === 'unhealthy')
    return matchesSearch && matchesFilter
  })

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Applications</h2>
          <p className='text-muted-foreground'>Ajoutez et gérez les applications protégées et leurs politiques</p>
        </div>
        <Button onClick={() => setAddDrawerOpen(true)}>
          <Plus className='h-4 w-4 mr-2' /> Ajouter une application
        </Button>
      </div>

      <div className='flex flex-wrap items-center gap-4 mb-4'>
        <div className='relative flex-1 max-w-md'>
          <Search className='absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground' />
          <Input placeholder='Rechercher…' value={searchQuery} onChange={(event) => setSearchQuery(event.target.value)} className='pl-10' />
        </div>
        <div className='flex items-center gap-2'>
          <span className='text-sm text-muted-foreground'>Filtre :</span>
          {(['Tous', 'HTTP', 'SSH', 'TCP', 'Saines', 'Hors ligne'] as FilterType[]).map((filter) => (
            <Button key={filter} variant={activeFilter === filter ? 'secondary' : 'ghost'} size='sm' onClick={() => setActiveFilter(filter)} className='px-3'>
              {filter}
            </Button>
          ))}
        </div>
      </div>

      {isError ? (
        <p className='text-destructive'>Impossible de charger les applications.</p>
      ) : isLoading ? (
        <div className='flex items-center justify-center h-64'><Loader2 className='h-8 w-8 animate-spin text-muted-foreground' /></div>
      ) : (
        <ApplicationsTable applications={filteredApplications} isLoading={isLoading} />
      )}

      <AddApplicationForm
        open={addDrawerOpen}
        onOpenChange={setAddDrawerOpen}
        onPublicSelect={() => { setAddDrawerOpen(false); setPublicConfirmOpen(true) }}
      />
      <PublicAppConfirmation
        open={publicConfirmOpen}
        onOpenChange={setPublicConfirmOpen}
        onConfirm={() => { setPublicConfirmOpen(false); setAddDrawerOpen(true) }}
      />
    </Main>
  )
}