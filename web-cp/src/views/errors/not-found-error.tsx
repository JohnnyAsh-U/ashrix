import { useNavigate, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'

export function NotFoundError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>404</h1>
        <span className='font-medium'>Oups! Page non trouvée !</span>
        <p className='text-center text-muted-foreground'>
          Il semble que la page que vous recherchez <br />
          n'existe pas ou a peut-être été supprimée.
        </p>
        <div className='mt-6 flex gap-4'>
          <Button variant='outline' onClick={() => history.go(-1)}>
            Retour
          </Button>
          <Button onClick={() => navigate({ to: '/' })}>Retour à l'accueil</Button>
        </div>
      </div>
    </div>
  )
}
