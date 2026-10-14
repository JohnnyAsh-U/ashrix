import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { Loader2, Plus } from 'lucide-react'
import { PoliciesTable } from './components/policies-table'
import { VersionHistory } from './components/version-history'
import { usePoliciesQuery } from '@/hooks/policy.hooks'
import type { Policy } from '@/types/policy'
import type { PolicyResponse} from '@/types/policy-api'
import { useNavigate } from '@tanstack/react-router'

const toPolicy = (policy: PolicyResponse): Policy => {
  const groupSubjects = policy.subjects.filter((subject) => subject.type === 'group').map((subject) => subject.value)
  const resource = policy.resources.find((item) => item.type === 'app')
  return {
    id: policy.id,
    ruleName: policy.name,
    app: resource?.value || 'Toutes les applications',
    appHostname: 'N/D',
    who: groupSubjects.join(', ') || policy.subjects.map((subject) => subject.value).join(', ') || 'Tous',
    usersCount: 0,
    status: policy.enabled ? 'active' : 'disabled',
    effect: policy.effect,
    priority: policy.priority,
    mfaRequired: policy.conditions.mfa?.required ?? false,
    groups: groupSubjects.length ? groupSubjects : undefined,
    ipRanges: policy.conditions.network?.allowed_cidrs,
    version: policy.version,
    lastModified: new Date(policy.updated_at).toLocaleString('fr-FR'),
    modifiedBy: policy.created_by,
    versions: [{ version: policy.version, date: new Date(policy.updated_at).toLocaleString('fr-FR'), by: policy.created_by, change: 'Version actuelle' }],
  }
}

export default function Policies() {
  const [versionHistoryOpen, setVersionHistoryOpen] = useState(false)
  const [selectedPolicy, setSelectedPolicy] = useState<Policy | null>(null)
  const { data, isLoading, isError } = usePoliciesQuery()
  console.log(data)
  const policies = (data?.items ?? []).map((policy) => toPolicy(policy))
  const [appFilter, setAppFilter] = useState('all')
  const [effectFilter, setEffectFilter] = useState<'Tous' | 'ALLOW' | 'DENY'>('Tous')
  const [statusFilter, setStatusFilter] = useState<'Tous' | 'Actives' | 'Désactivées'>('Tous')
  const filteredPolicies = policies.filter((policy) =>
    (appFilter === 'all' || policy.app === appFilter) &&
    (effectFilter === 'Tous' || policy.effect === effectFilter) &&
    (statusFilter === 'Tous' || (statusFilter === 'Actives' && policy.status === 'active') || (statusFilter === 'Désactivées' && policy.status === 'disabled'))
  )

  const navigate = useNavigate()

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Politiques</h2>
          <p className='text-muted-foreground'>Créez et gérez les règles d’accès</p>
        </div>
        <Button onClick={() => navigate({to:'/create-policy'})}>
          <Plus className='h-4 w-4 mr-2' />
          Créer une politique
        </Button>
      </div>
      <div className='flex flex-wrap items-center gap-4 mb-4'>
        <div className='flex items-center gap-2'>
          <span className='text-sm text-muted-foreground'>Application :</span>
          <select className='h-9 rounded-md border bg-background px-3 text-sm' value={appFilter} onChange={(event) => setAppFilter(event.target.value)}>
            <option value='all'>Toutes</option>
            {[...new Set(policies.map((policy) => policy.app))].map((app) => <option key={app} value={app}>{app}</option>)}
          </select>
        </div>
        <div className='flex items-center gap-2'>
          <span className='text-sm text-muted-foreground'>Effet :</span>
          {(['Tous', 'ALLOW', 'DENY'] as const).map((filter) =>
            <Button key={filter} variant={effectFilter === filter ? 'secondary' : 'ghost'} size='sm' onClick={() => setEffectFilter(filter)}>{filter}</Button>)}
        </div>
        <div className='flex items-center gap-2'>
          <span className='text-sm text-muted-foreground'>État :</span>
          {(['Tous', 'Actives', 'Désactivées'] as const).map((filter) =>
            <Button key={filter} variant={statusFilter === filter ? 'secondary' : 'ghost'} size='sm' onClick={() => setStatusFilter(filter)}>{filter}</Button>)}
        </div>
      </div>
      {isError ?
        <p className='text-destructive'>Impossible de charger les politiques.</p>
        : isLoading ?
          <div className='flex items-center justify-center h-64'>
            <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
          </div>
          : <PoliciesTable policies={filteredPolicies} isLoading={isLoading} onViewVersionHistory={(policy) => { setSelectedPolicy(policy); setVersionHistoryOpen(true) }} />
      }
      <VersionHistory open={versionHistoryOpen} onOpenChange={setVersionHistoryOpen} policy={selectedPolicy} />
    </Main>
  )
}
