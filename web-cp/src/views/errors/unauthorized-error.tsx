import { useNavigate, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'

export function UnauthorisedError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>401</h1>
        <span className='font-medium'>Accès non autorisé</span>
        <p className='text-center text-muted-foreground'>
          Veuillez vous connecter avec les identifiants appropriés <br /> pour accéder à cette
          ressource.
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
