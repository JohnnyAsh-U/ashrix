import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { GatewayCards } from './components/gateway-cards'
import { EnrollGatewayForm } from './components/enroll-gateway-form'
import { Plus, Loader2 } from 'lucide-react'
import { useGatewaysQuery } from '@/hooks/gateway.hooks'


export default function Gateways() {
  const [enrollGatewayOpen, setEnrollGatewayOpen] = useState(false)
  const { data: gateways, isLoading, isError } = useGatewaysQuery()

  const handleOpenEnrollGateway = () => {
    setEnrollGatewayOpen(true)
  }

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Passerelles</h2>
          <p className='text-muted-foreground'>
            Enrôlez, surveillez et désactivez votre parc de passerelles
          </p>
        </div>
        <div className='flex items-center space-x-2'>
          <Button onClick={handleOpenEnrollGateway}>
            <Plus className='h-4 w-4 mr-2' />
            Enrôler une passerelle
          </Button>
        </div>
      </div>

      {isError ? (
        <p className='text-destructive'>Impossible de charger les passerelles.</p>
      ) : isLoading ? (
        <div className='flex items-center justify-center h-64'>
          <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
        </div>
      ) : (
        <GatewayCards
          gateways={gateways ?? []}
          isLoading={isLoading}
        />
      )}

      {/* Panneau d’enrôlement */}
      <EnrollGatewayForm
        open={enrollGatewayOpen}
        onOpenChange={setEnrollGatewayOpen}
      />
    </Main>
  )
}