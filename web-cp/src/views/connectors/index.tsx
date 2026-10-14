import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { ConnectorsTable } from './components/connectors-table'
import { GenerateTokenForm } from './components/generate-token-form'
import { Plus, Loader2 } from 'lucide-react'
import { useConnectorsQuery } from '@/hooks/connector.hooks'


export default function Connectors() {
  const [generateTokenOpen, setGenerateTokenOpen] = useState(false)
  const { data: connectors, isLoading, isError } = useConnectorsQuery()
  // const connectors = (data ?? []).map(toConnector)

  const handleOpenGenerateToken = () => {
    setGenerateTokenOpen(true)
  }

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Connecteurs</h2>
          <p className='text-muted-foreground'>
            Surveillez l’état, révoquez et réenrôlez vos connecteurs
          </p>
        </div>
        <div className='flex items-center space-x-2'>
          <Button onClick={handleOpenGenerateToken}>
            <Plus className='h-4 w-4 mr-2' />
            Enrôler un connecteur
          </Button>
        </div>
      </div>

      {isError ? (
        <p className='text-destructive'>Impossible de charger les connecteurs.</p>
      ) : isLoading ? (
        <div className='flex items-center justify-center h-64'>
          <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
        </div>
      ) : (
        <ConnectorsTable
          connectors={connectors || []}
          isLoading={isLoading}
        />
      )}

      {/* Generate Token Drawer */}
      <GenerateTokenForm
        open={generateTokenOpen}
        onOpenChange={setGenerateTokenOpen}
      />
    </Main>
  )
}