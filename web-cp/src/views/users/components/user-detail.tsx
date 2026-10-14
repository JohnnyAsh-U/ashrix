import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  ArrowLeft,
  CheckCircle,
  XCircle,
  Monitor,
  MapPin,
  Clock,
  AlertTriangle,
  FileText,
  Shield
} from 'lucide-react'
import type { User, Session } from '@/types/user'
import { toast } from 'sonner'

interface UserDetailProps {
  user: User
  onBack: () => void
}

export function UserDetail({ user, onBack }: UserDetailProps) {
  const [revokeSessionOpen, setRevokeSessionOpen] = useState(false)
  const [selectedSession, setSelectedSession] = useState<Session | null>(null)

  const handleRevokeSession = (session: Session) => {
    setSelectedSession(session)
    setRevokeSessionOpen(true)
  }

  const confirmRevokeSession = () => {
    console.log('Revoking session:', selectedSession?.id)
    toast.success('Session revoked')
    setRevokeSessionOpen(false)
    setSelectedSession(null)
  }

  const handleRevokeAllSessions = () => {
    console.log('Revoking all sessions for:', user.email)
    toast.success('All sessions revoked')
  }

  return (
    <div className='space-y-6'>
      {/* Header */}
      <div className='flex items-center gap-4'>
        <Button variant='ghost' size='sm' onClick={onBack}>
          <ArrowLeft className='h-4 w-4 mr-1' /> Users
        </Button>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>{user.name}</h2>
          <div className='flex items-center gap-2 mt-1'>
            <p className='text-muted-foreground'>{user.email}</p>
            <span className='text-muted-foreground'>·</span>
            <div className='flex gap-1'>
              {user.groups.map((group, idx) => (
                <Badge key={idx} variant='secondary' className='text-xs'>
                  {group}
                </Badge>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Permitted Applications */}
      <div>
        <h3 className='text-sm font-semibold mb-3'>PERMITTED APPLICATIONS</h3>
        <div className='space-y-2'>
          {user.permittedApps.map((app, idx) => (
            <div 
              key={idx} 
              className='flex items-center justify-between py-2 px-4 bg-muted/30 rounded-lg'
            >
              <div className='flex items-center gap-3'>
                {app.access === 'allowed' ? (
                  <CheckCircle className='h-5 w-5 text-green-600' />
                ) : (
                  <XCircle className='h-5 w-5 text-red-400' />
                )}
                <span className='font-medium'>{app.name}</span>
              </div>
              <span className='text-sm text-muted-foreground'>
                {app.access === 'allowed' ? `via: ${app.via}` : 'No matching policy rule'}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Active Sessions */}
      <div>
        <h3 className='text-sm font-semibold mb-3'>
          ACTIVE SESSIONS ({user.activeSessions.length})
        </h3>
        {user.activeSessions.length === 0 ? (
          <p className='text-sm text-muted-foreground'>No active sessions</p>
        ) : (
          <div className='space-y-3'>
            {user.activeSessions.map((session) => (
              <div key={session.id} className='border rounded-lg p-4'>
                <div className='flex items-center gap-4 mb-2'>
                  <Monitor className='h-4 w-4 text-muted-foreground' />
                  <span className='text-sm font-medium'>{session.device}</span>
                  <MapPin className='h-4 w-4 text-muted-foreground ml-4' />
                  <span className='text-sm'>{session.ip}</span>
                  <span className='text-sm text-muted-foreground'>· {session.location}</span>
                </div>
                <div className='flex items-center gap-2 text-sm text-muted-foreground mb-3'>
                  <Shield className='h-4 w-4' />
                  <span>Gateway: {session.gateway}</span>
                  <Clock className='h-4 w-4 ml-4' />
                  <span>Started: {session.started}</span>
                  <span>· {session.ttl}</span>
                </div>
                <div className='flex gap-2'>
                  <Button 
                    variant='outline' 
                    size='sm'
                    onClick={() => handleRevokeSession(session)}
                  >
                    Revoke this session
                  </Button>
                  <Button 
                    variant='outline' 
                    size='sm'
                    onClick={handleRevokeAllSessions}
                  >
                    Revoke all sessions
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Recent Access */}
      <div>
        <div className='flex items-center justify-between mb-3'>
          <h3 className='text-sm font-semibold'>RECENT ACCESS (last 7 days)</h3>
          <Button variant='ghost' size='sm'>
            View full log <FileText className='h-4 w-4 ml-1' />
          </Button>
        </div>
        <div className='border rounded-lg overflow-hidden'>
          <table className='w-full'>
            <thead>
              <tr className='border-b bg-muted/30'>
                <th className='text-left p-3 text-sm font-medium'>Date</th>
                <th className='text-left p-3 text-sm font-medium'>App</th>
                <th className='text-left p-3 text-sm font-medium'>Result</th>
                <th className='text-left p-3 text-sm font-medium'>Device</th>
              </tr>
            </thead>
            <tbody>
              {user.recentAccess.map((access, idx) => (
                <tr key={idx} className='border-b last:border-0'>
                  <td className='p-3 text-sm'>{access.date}</td>
                  <td className='p-3 text-sm font-medium'>{access.app}</td>
                  <td className='p-3'>
                    {access.result === 'allowed' ? (
                      <span className='inline-flex items-center gap-1 text-green-600 text-sm'>
                        <CheckCircle className='h-4 w-4' /> allowed
                      </span>
                    ) : (
                      <span className='inline-flex items-center gap-1 text-red-600 text-sm'>
                        <XCircle className='h-4 w-4' /> denied
                      </span>
                    )}
                  </td>
                  <td className='p-3 text-sm text-muted-foreground'>{access.device}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Revoke Session Confirmation Dialog */}
      <Dialog open={revokeSessionOpen} onOpenChange={setRevokeSessionOpen}>
        <DialogContent className='max-w-md'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5 text-yellow-600' />
              Revoke session?
            </DialogTitle>
          </DialogHeader>

          <div className='space-y-3'>
            <div className='bg-muted/50 rounded-lg p-4 space-y-2'>
              <div>
                <p className='text-sm font-medium'>User:</p>
                <p className='text-sm text-muted-foreground'>{user.email}</p>
              </div>
              <div>
                <p className='text-sm font-medium'>Device:</p>
                <p className='text-sm text-muted-foreground'>{selectedSession?.device}</p>
              </div>
              <div>
                <p className='text-sm font-medium'>Started:</p>
                <p className='text-sm text-muted-foreground'>{selectedSession?.started}</p>
              </div>
            </div>
            <p className='text-sm text-muted-foreground'>
              {user.name.split(' ')[0]} will be logged out on her next request 
              (typically within 1s).
            </p>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setRevokeSessionOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              variant='destructive'
              onClick={confirmRevokeSession}
              className='flex-1'
            >
              Revoke session
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}