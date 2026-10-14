import { Button } from '@/components/ui/button'
import { useState, useEffect } from 'react'
import { 
  Loader2,
  Clock,
  Mail,
  AlertTriangle,
  Save,
  ArrowUpRight,
  CheckCircle,
} from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'

// Static data
const staticOrganisation = {
  name: 'Acme Corp',
  slug: 'acme',
  portalUrl: 'access',
  domain: 'acme.com',
  plan: {
    name: 'Growth',
    price: '$149/month'
  },
  sessionSettings: {
    ttl: 8,
    maxSessionsPerUser: 'unlimited',
    afterTtl: 'silent-reauth'
  },
  alertEmail: 'james@acme.com'
}

export default function OrganisationSettingsWithDetection() {
  const [formData, setFormData] = useState({
    orgName: staticOrganisation.name,
    slug: staticOrganisation.slug,
    portalUrl: staticOrganisation.portalUrl,
    sessionTtl: staticOrganisation.sessionSettings.ttl,
    maxSessions: staticOrganisation.sessionSettings.maxSessionsPerUser,
    afterTtl: staticOrganisation.sessionSettings.afterTtl,
    alertEmail: staticOrganisation.alertEmail
  })

  const [isSaving, setIsSaving] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)

  useEffect(() => {
    const hasUnsavedChanges = 
      formData.orgName !== staticOrganisation.name ||
      formData.slug !== staticOrganisation.slug ||
      formData.portalUrl !== staticOrganisation.portalUrl ||
      formData.sessionTtl !== staticOrganisation.sessionSettings.ttl ||
      formData.maxSessions !== staticOrganisation.sessionSettings.maxSessionsPerUser ||
      formData.afterTtl !== staticOrganisation.sessionSettings.afterTtl ||
      formData.alertEmail !== staticOrganisation.alertEmail

    setHasChanges(hasUnsavedChanges)
  }, [formData])

  const handleSave = () => {
    setSaveDialogOpen(true)
  }

  const confirmSave = () => {
    setIsSaving(true)
    setSaveDialogOpen(false)
    
    // Simulate API call
    setTimeout(() => {
      setIsSaving(false)
      toast.success('Settings saved successfully')
    }, 1000)
  }

  const handleUpgrade = () => {
    console.log('Upgrade plan')
    toast.info('Opening plan upgrade...')
  }

  return (
    <>
      <div className='max-w-3xl space-y-8'>
        {/* Organisation Section */}
        <div className='space-y-4'>
         

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='orgName'>Name</Label>
              <Input
                id='orgName'
                value={formData.orgName}
                onChange={(e) => setFormData({ ...formData, orgName: e.target.value })}
                placeholder='Acme Corp'
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='slug'>Slug</Label>
              <Input
                id='slug'
                value={formData.slug}
                onChange={(e) => setFormData({ ...formData, slug: e.target.value })}
                placeholder='acme'
              />
            </div>

            <div className='space-y-2'>
              <Label htmlFor='portalUrl'>Portal URL</Label>
              <div className='flex items-center gap-2'>
                <Input
                  id='portalUrl'
                  value={formData.portalUrl}
                  onChange={(e) => setFormData({ ...formData, portalUrl: e.target.value })}
                  className='max-w-xs'
                />
                <span className='text-sm text-muted-foreground'>
                  .{staticOrganisation.domain}
                </span>
              </div>
            </div>

            <div className='space-y-2'>
              <Label>Plan</Label>
              <div className='flex items-center gap-3'>
                <Badge variant='secondary' className='text-sm'>
                  {staticOrganisation.plan.name} ({staticOrganisation.plan.price})
                </Badge>
                <Button 
                  variant='outline' 
                  size='sm'
                  onClick={handleUpgrade}
                >
                  Upgrade <ArrowUpRight className='h-4 w-4 ml-1' />
                </Button>
              </div>
            </div>
          </div>
        </div>

        {/* Session Settings Section */}
        <div className='space-y-4 pt-6 border-t'>
          <div className='flex items-center gap-2'>
            <Clock className='h-5 w-5 text-muted-foreground' />
            <h3 className='text-lg font-semibold'>SESSION SETTINGS</h3>
          </div>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='sessionTtl'>Session TTL</Label>
              <div className='flex items-center gap-2'>
                <Input
                  id='sessionTtl'
                  type='number'
                  min={1}
                  max={720}
                  value={formData.sessionTtl}
                  onChange={(e) => setFormData({ ...formData, sessionTtl: parseInt(e.target.value) || 8 })}
                  className='max-w-xs'
                />
                <span className='text-sm text-muted-foreground'>hours (1 to 720)</span>
              </div>
            </div>

            <div className='space-y-2'>
              <Label htmlFor='maxSessions'>Max sessions/user</Label>
              <Select 
                value={formData.maxSessions} 
                onValueChange={(value) => setFormData({ ...formData, maxSessions: value })}
              >
                <SelectTrigger id='maxSessions' className='w-48'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='unlimited'>Unlimited</SelectItem>
                  <SelectItem value='1'>1 session</SelectItem>
                  <SelectItem value='2'>2 sessions</SelectItem>
                  <SelectItem value='3'>3 sessions</SelectItem>
                  <SelectItem value='5'>5 sessions</SelectItem>
                  <SelectItem value='10'>10 sessions</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='space-y-2'>
              <Label>After TTL:</Label>
              <RadioGroup
                value={formData.afterTtl}
                onValueChange={(value) => setFormData({ ...formData, afterTtl: value })}
                className='space-y-2'
              >
                <div className='flex items-center space-x-2'>
                  <RadioGroupItem value='silent-reauth' id='silent-reauth' />
                  <Label htmlFor='silent-reauth'>
                    Silent re-auth (if IdP session valid)
                  </Label>
                </div>
                <div className='flex items-center space-x-2'>
                  <RadioGroupItem value='force-relogin' id='force-relogin' />
                  <Label htmlFor='force-relogin'>
                    Force re-login
                  </Label>
                </div>
              </RadioGroup>
            </div>
          </div>
        </div>

        {/* Alert Email Section */}
        <div className='space-y-4 pt-6 border-t'>
          <div className='flex items-center gap-2'>
            <Mail className='h-5 w-5 text-muted-foreground' />
            <h3 className='text-lg font-semibold'>ALERT EMAIL</h3>
          </div>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='alertEmail'>Email address</Label>
              <Input
                id='alertEmail'
                type='email'
                value={formData.alertEmail}
                onChange={(e) => setFormData({ ...formData, alertEmail: e.target.value })}
                placeholder='james@acme.com'
                className='max-w-md'
              />
            </div>

            <div className='flex items-start gap-2'>
              <AlertTriangle className='h-4 w-4 text-yellow-600 mt-0.5' />
              <div>
                <p className='text-sm font-medium'>Alerts for:</p>
                <ul className='text-sm text-muted-foreground mt-1 space-y-1'>
                  <li>• Gateway offline</li>
                  <li>• Certificate expiry</li>
                  <li>• Connector down</li>
                </ul>
              </div>
            </div>
          </div>
        </div>

        {/* Save Button */}
        <div className='flex items-center justify-end pt-6 border-t gap-2'>
          {hasChanges && (
            <Button 
              variant='outline'
              onClick={() => {
                setFormData({
                  orgName: staticOrganisation.name,
                  slug: staticOrganisation.slug,
                  portalUrl: staticOrganisation.portalUrl,
                  sessionTtl: staticOrganisation.sessionSettings.ttl,
                  maxSessions: staticOrganisation.sessionSettings.maxSessionsPerUser,
                  afterTtl: staticOrganisation.sessionSettings.afterTtl,
                  alertEmail: staticOrganisation.alertEmail
                })
                toast.info('Changes discarded')
              }}
            >
              Discard changes
            </Button>
          )}
          <Button 
            onClick={handleSave}
            disabled={isSaving || !hasChanges}
            className='min-w-32'
          >
            {isSaving ? (
              <>
                <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                Saving...
              </>
            ) : (
              <>
                <Save className='h-4 w-4 mr-2' />
                Save changes
              </>
            )}
          </Button>
        </div>
      </div>

      {/* Save Confirmation Dialog */}
      <Dialog open={saveDialogOpen} onOpenChange={setSaveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <CheckCircle className='h-5 w-5 text-green-600' />
              Save Changes
            </DialogTitle>
            <DialogDescription>
              Apply these organisation settings?
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-3'>
            <div className='bg-muted/50 rounded-lg p-4 space-y-2'>
              <div className='flex justify-between'>
                <span className='text-sm text-muted-foreground'>Organisation name</span>
                <span className='text-sm font-medium'>{formData.orgName}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-sm text-muted-foreground'>Session TTL</span>
                <span className='text-sm font-medium'>{formData.sessionTtl} hours</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-sm text-muted-foreground'>Max sessions</span>
                <span className='text-sm font-medium'>{formData.maxSessions}</span>
              </div>
              <div className='flex justify-between'>
                <span className='text-sm text-muted-foreground'>Alert email</span>
                <span className='text-sm font-medium'>{formData.alertEmail}</span>
              </div>
            </div>
            <p className='text-sm text-muted-foreground'>
              These changes will take effect immediately for all users.
            </p>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setSaveDialogOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              onClick={confirmSave}
              className='flex-1'
            >
              Confirm Changes
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}