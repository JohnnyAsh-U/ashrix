// import { useNavigate, useLocation } from '@tanstack/react-router'
// import { useAuthStore } from '@/stores/auth-store'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useLogoutMutation } from '@/hooks/auth.hooks'
import { toast } from 'sonner'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  // const location = useLocation()
  const { mutateAsync: logout } = useLogoutMutation()

  const handleSignOut = () => {
    logout()
    toast.success("Deconnexion avec succes")
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='Déconnexion'
      desc='Êtes-vous sûr de vouloir vous déconnecter ? Vous devrez vous reconnecter pour accéder à votre compte.'
      confirmText='Se déconnecter'
      destructive
      handleConfirm={handleSignOut}
      className='sm:max-w-sm'
    />
  )
}
