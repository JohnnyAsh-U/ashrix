import { useState } from 'react'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  X,
  Loader2,
  Shield,
  Clock,
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  Users
} from 'lucide-react'
import type { ActiveSessionApiResponse } from '@/types/session'
import { toast } from 'sonner'
import { useRevokeSessionMutation } from '@/hooks/session.hooks'

interface SessionsTableProps {
  sessions: ActiveSessionApiResponse[]
  isLoading?: boolean
}

export function SessionsTable({ sessions, isLoading }: SessionsTableProps) {
  const [revokeDialogOpen, setRevokeDialogOpen] = useState(false)
  const [selectedSession, setSelectedSession] = useState<ActiveSessionApiResponse | null>(null)
  const [showAll, setShowAll] = useState(false)
  const revokeSession = useRevokeSessionMutation()

  const displaySessions = showAll ? sessions : sessions.slice(0, 5)
  const remainingCount = sessions.length - displaySessions.length

  const handleRevokeClick = (session: ActiveSessionApiResponse, e?: React.MouseEvent) => {
    e?.stopPropagation()
    setSelectedSession(session)
    setRevokeDialogOpen(true)
  }

  const confirmRevoke = async () => {
    if (!selectedSession) return
    try {
      await revokeSession.mutateAsync(selectedSession.id)
      toast.success(`Session de ${selectedSession.user_email} révoquée`)
      setRevokeDialogOpen(false)
      setSelectedSession(null)
    } catch {
      toast.error('Impossible de révoquer la session')
    }
  }

  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (sessions.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>Aucune session active</p>
      </div>
    )
  }

  return (
    <>
      <div className='border rounded-lg overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Utilisateur</TableHead>
              <TableHead>Passerelle</TableHead>
              <TableHead>Début</TableHead>
              <TableHead>Expire</TableHead>
              <TableHead className='text-right'>Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {displaySessions.map((session) => (
              <TableRow
                key={session.id}
                className='cursor-pointer hover:bg-muted/50'
                onClick={() => handleRevokeClick(session)}
              >
                <TableCell>
                  <div>
                    <p className='font-medium'>{session.user_email}</p>
                    <p className='text-xs text-muted-foreground'>{session.user_id}</p>
                  </div>
                </TableCell>
                <TableCell>{session.gateway_name}</TableCell>
                <TableCell>{new Date(session.issued_at).toLocaleString('fr-FR')}</TableCell>
                <TableCell>{new Date(session.expires_at).toLocaleString('fr-FR')}</TableCell>
                <TableCell className='text-right'>
                  <Button
                    variant='ghost'
                    size='icon'
                    onClick={(e) => handleRevokeClick(session, e)}
                    className='hover:bg-red-50 hover:text-red-600'
                    title='Révoquer la session'
                  >
                    <X className='h-4 w-4' />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Show more/less */}
      {remainingCount > 0 && (
        <div className='mt-4 text-center'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setShowAll(!showAll)}
          >
            {showAll ? (
              <>
                <ChevronUp className='h-4 w-4 mr-1' /> Afficher moins
              </>
            ) : (
              <>
                <ChevronDown className='h-4 w-4 mr-1' />
                {remainingCount} session(s) supplémentaire(s)
              </>
            )}
          </Button>
        </div>
      )}

      {/* Revoke Session Confirmation Dialog */}
      <Dialog open={revokeDialogOpen} onOpenChange={setRevokeDialogOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5 text-yellow-600' />
              Révoquer la session ?
            </DialogTitle>
            <DialogDescription>
              La session de l’utilisateur sera immédiatement terminée.
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='bg-muted/50 rounded-lg p-4 space-y-3'>
              <div className='flex items-center gap-2'>
                <Users className='h-4 w-4 text-muted-foreground' />
                <div>
                  <p className='text-sm font-medium'>Utilisateur</p>
                  <p className='text-sm text-muted-foreground'>{selectedSession?.user_email}</p>
                </div>
              </div>
              <div className='flex items-center gap-2'>
                <Clock className='h-4 w-4 text-muted-foreground' />
                <div>
                  <p className='text-sm font-medium'>Début</p>
                  <p className='text-sm text-muted-foreground'>{selectedSession?.issued_at}</p>
                </div>
              </div>
              <div className='flex items-center gap-2'>
                <Shield className='h-4 w-4 text-muted-foreground' />
                <div>
                  <p className='text-sm font-medium'>Passerelle</p>
                  <p className='text-sm text-muted-foreground'>{selectedSession?.gateway_name}</p>
                </div>
              </div>
            </div>

            <div className='bg-yellow-50 border border-yellow-200 rounded-lg p-3'>
              <p className='text-sm text-yellow-800'>
                {selectedSession?.user_email} sera déconnecté lors de sa prochaine requête
                (généralement sous une seconde).
              </p>
            </div>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setRevokeDialogOpen(false)}
              className='flex-1'
            >
              Annuler
            </Button>
            <Button
              variant='destructive'
              onClick={confirmRevoke}
              className='flex-1'
            >
              Révoquer la session
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}