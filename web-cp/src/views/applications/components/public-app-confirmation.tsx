import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { AlertTriangle, CheckCircle } from 'lucide-react'

interface PublicAppConfirmationProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}

export function PublicAppConfirmation({
  open,
  onOpenChange,
  onConfirm,
}: PublicAppConfirmationProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-md'>
        <DialogHeader>
          <div className='flex items-center gap-2 mb-2'>
            <AlertTriangle className='h-6 w-6 text-yellow-500' />
            <DialogTitle className='text-xl'>Application publique</DialogTitle>
          </div>
          <DialogDescription>
            Cette application sera accessible sans connexion. Toute personne sur Internet pourra y accéder.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-3'>
          <p className='text-sm font-medium'>Ashrix fournira toujours :</p>
          <div className='space-y-2'>
            {[
              'Terminaison TLS',
              'Limitation de débit',
              'En-têtes de sécurité',
              'Journalisation des accès',
            ].map((item) => (
              <div key={item} className='flex items-center gap-2'>
                <CheckCircle className='h-4 w-4 text-green-600' />
                <span className='text-sm'>{item}</span>
              </div>
            ))}
          </div>
          <div className='bg-yellow-50 border border-yellow-200 rounded-lg p-3'>
            <p className='text-sm font-medium text-yellow-800'>
              Authentification : AUCUNE
            </p>
          </div>
        </div>

        <DialogFooter className='flex gap-2'>
          <Button
            variant='outline'
            onClick={() => onOpenChange(false)}
            className='flex-1'
          >
            Annuler
          </Button>
          <Button
            onClick={onConfirm}
            className='flex-1'
          >
            J’ai compris, continuer
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}