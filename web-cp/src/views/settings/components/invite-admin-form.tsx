import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { 
  Shield,
  Eye,
  Mail,
  Clock,
  Building2,
  Loader2,
  Info
} from 'lucide-react'
import { toast } from 'sonner'

interface InviteAdminFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function InviteAdminForm({
  open,
  onOpenChange,
}: InviteAdminFormProps) {
  const [email, setEmail] = useState('')
  const [role, setRole] = useState('Admin')
  const [isSending, setIsSending] = useState(false)

  useEffect(() => {
    if (!open) {
      // Reset form when closed
      setEmail('')
      setRole('Admin')
      setIsSending(false)
    }
  }, [open])

  const handleSendInvite = () => {
    if (!email || !email.includes('@')) {
      toast.error('Please enter a valid email address')
      return
    }

    setIsSending(true)
    
    // Simulate API call
    setTimeout(() => {
      setIsSending(false)
      toast.success(`Invite sent to ${email}`)
      onOpenChange(false)
    }, 1500)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col'>
        <SheetHeader className='text-start'>
          <SheetTitle>Invite Admin</SheetTitle>
          <SheetDescription>
            Invite a new admin user to the console
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
          {/* Email Address */}
          <div className='space-y-2'>
            <Label htmlFor='email'>Email address</Label>
            <Input
              id='email'
              type='email'
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder='kwame@acme.com'
            />
          </div>

          {/* Role */}
          <div className='space-y-2'>
            <Label>Role</Label>
            <RadioGroup
              value={role}
              onValueChange={setRole}
              className='space-y-3'
            >
              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50'>
                <RadioGroupItem value='Admin' id='admin' />
                <Label htmlFor='admin' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Shield className='h-5 w-5 text-blue-600' />
                    <div>
                      <p className='font-medium'>Admin</p>
                      <p className='text-xs text-muted-foreground'>
                        Manage apps, policies, gateways
                      </p>
                    </div>
                  </div>
                </Label>
              </div>

              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50'>
                <RadioGroupItem value='Auditor' id='auditor' />
                <Label htmlFor='auditor' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Eye className='h-5 w-5 text-gray-600' />
                    <div>
                      <p className='font-medium'>Auditor</p>
                      <p className='text-xs text-muted-foreground'>
                        Read-only access to logs and audit trail
                      </p>
                    </div>
                  </div>
                </Label>
              </div>
            </RadioGroup>
          </div>

          {/* Information Box */}
          <div className='space-y-3'>
            <div className='flex items-start gap-2 text-sm text-muted-foreground'>
              <Mail className='h-4 w-4 mt-0.5' />
              <p>They will receive an email invite.</p>
            </div>
            <div className='flex items-start gap-2 text-sm text-muted-foreground'>
              <Clock className='h-4 w-4 mt-0.5' />
              <p>Invite expires in 24 hours.</p>
            </div>
            <div className='flex items-start gap-2 text-sm text-muted-foreground'>
              <Building2 className='h-4 w-4 mt-0.5' />
              <p>They must sign in with acme.com SSO.</p>
            </div>
          </div>

          {/* Info Box */}
          <div className='bg-blue-50 border border-blue-200 rounded-lg p-4'>
            <div className='flex items-start gap-2'>
              <Info className='h-4 w-4 text-blue-600 mt-0.5' />
              <div className='text-sm text-blue-800'>
                <p className='font-medium mb-1'>
                  Only users with @acme.com email can be invited.
                </p>
                <p className='text-blue-700'>
                  The invite will be linked to their SSO identity.
                </p>
              </div>
            </div>
          </div>
        </div>

        <SheetFooter>
          <div className='flex gap-2 pt-4'>
            <Button
              type='button'
              variant='outline'
              onClick={() => onOpenChange(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button 
              type='submit' 
              className='flex-1'
              onClick={handleSendInvite}
              disabled={isSending || !email}
            >
              {isSending ? (
                <>
                  <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                  Sending...
                </>
              ) : (
                'Send Invite'
              )}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}