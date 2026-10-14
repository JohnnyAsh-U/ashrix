import useDialogState from '@/hooks/use-dialog-state'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { PasswordInput } from '@/components/password-input'
import { SignOutDialog } from '@/components/sign-out-dialog'
import { useAuthStore } from '@/stores/auth.store'
import { useChangePasswordMutation } from '@/hooks/auth.hooks'
import { toast } from 'sonner'
import { useState } from 'react'

export function ProfileDropdown() {
  const [open, setOpen] = useDialogState()
  const [changePasswordOpen, setChangePasswordOpen] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const auth = useAuthStore()
  const changePassword = useChangePasswordMutation()

  const handleChangePasswordOpen = (nextOpen: boolean) => {
    if (!nextOpen) {
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    }
    setChangePasswordOpen(nextOpen)
  }

  const handleChangePassword = async () => {
    if (newPassword.length < 8) {
      toast.error('Le nouveau mot de passe doit contenir au moins 8 caractères')
      return
    }
    if (newPassword !== confirmPassword) {
      toast.error('Les mots de passe ne correspondent pas')
      return
    }

    try {
      await changePassword.mutateAsync({
        old_password: oldPassword,
        new_password: newPassword,
      })
      toast.success('Mot de passe modifié avec succès')
      handleChangePasswordOpen(false)
    } catch {
      toast.error('Impossible de modifier le mot de passe')
    }
  }

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' className='relative h-8 w-8 rounded-full'>
            <Avatar className='h-8 w-8'>
              <AvatarImage src='/avatars/01.png' alt='@shadcn' />
              <AvatarFallback>{auth.user?.email.charAt(0).toUpperCase()}</AvatarFallback>
            </Avatar>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent className='w-56' align='end' forceMount>
          <DropdownMenuLabel className='font-normal'>
            <div className='flex flex-col gap-1.5'>
              <p className='text-sm leading-none font-medium'>{auth.user?.email}</p>
              <p className='text-xs leading-none text-muted-foreground'>
                {auth.user?.org_name}
              </p>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem onClick={() => handleChangePasswordOpen(true)}>
              Modifier le mot de passe
              <DropdownMenuShortcut>⇧⌘P</DropdownMenuShortcut>
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant='destructive' onClick={() => setOpen(true)}>
            Deconnexion
            <DropdownMenuShortcut className='text-current'>
              ⇧⌘Q
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={changePasswordOpen} onOpenChange={handleChangePasswordOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Modifier le mot de passe</DialogTitle>
            <DialogDescription>
              Choisissez un nouveau mot de passe pour votre compte.
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <label htmlFor='old-password' className='text-sm font-medium'>Ancien mot de passe</label>
              <PasswordInput
                id='old-password'
                value={oldPassword}
                onChange={(event) => setOldPassword(event.target.value)}
                autoComplete='current-password'
              />
            </div>
            <div className='space-y-2'>
              <label htmlFor='new-password' className='text-sm font-medium'>Nouveau mot de passe</label>
              <PasswordInput
                id='new-password'
                value={newPassword}
                onChange={(event) => setNewPassword(event.target.value)}
                autoComplete='new-password'
              />
            </div>
            <div className='space-y-2'>
              <label htmlFor='confirm-password' className='text-sm font-medium'>Confirmer le mot de passe</label>
              <PasswordInput
                id='confirm-password'
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                autoComplete='new-password'
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => handleChangePasswordOpen(false)}>
              Annuler
            </Button>
            <Button
              onClick={handleChangePassword}
              disabled={changePassword.isPending || !oldPassword || !newPassword || !confirmPassword}
            >
              {changePassword.isPending ? 'Modification…' : 'Modifier le mot de passe'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <SignOutDialog open={!!open} onOpenChange={setOpen} />
    </>
  )
}
