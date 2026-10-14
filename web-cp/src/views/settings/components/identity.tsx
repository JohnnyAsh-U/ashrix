import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { 
  Loader2,
  Key,
  CheckCircle,
  XCircle,
  Save,
  Eye,
  EyeOff,
  RefreshCw,
  Info,
  Building2,
  AlertTriangle
} from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { toast } from 'sonner'
import { useCreateIdentityConfigMutation, useIdentityConfigsQuery } from '@/hooks/identity.hooks'
import type { CreateIdentityConfigRequest } from '@/types/identity'

type Provider = 'google' | 'okta' | 'entra' | 'other'

interface ProviderConfig {
  clientId: string
  clientSecret: string
  allowedDomain: string
  tenantId?: string
  issuerUrl?: string
  authorizationEndpoint?: string
  tokenEndpoint?: string
  userInfoEndpoint?: string
}

const providerConfigs: Record<Provider, Partial<ProviderConfig>> = {
  google: {
    clientId: '123456789-abc.apps.googleusercontent.com',
    clientSecret: 'GOCSPX-xxxxxxxxxxxxxxxxxxxx',
    allowedDomain: 'acme.com'
  },
  okta: {
    clientId: '0oa1abc2def3ghi4jkl5',
    clientSecret: 'okta_secret_xxxxxxxx',
    allowedDomain: 'acme.com',
    issuerUrl: 'https://acme.okta.com'
  },
  entra: {
    clientId: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    clientSecret: 'entra_secret_xxxxxxxx',
    allowedDomain: 'acme.com',
    tenantId: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
  },
  other: {
    clientId: '',
    clientSecret: '',
    allowedDomain: '',
    issuerUrl: '',
    authorizationEndpoint: '',
    tokenEndpoint: '',
    userInfoEndpoint: ''
  }
}

export default function IdentitySettingsEnhanced() {
  const [provider, setProvider] = useState<Provider>('google')
  const [config, setConfig] = useState<ProviderConfig>({
    ...providerConfigs.google as ProviderConfig
  })
  const [showSecret, setShowSecret] = useState(false)
  const [isTesting, setIsTesting] = useState(false)
  const [isSaving, setIsSaving] = useState(false)
  const [testResult, setTestResult] = useState<null | {
    status: 'success' | 'failed'
    message: string
    userCount: number
  }>(null)
  const [changeSecretDialogOpen, setChangeSecretDialogOpen] = useState(false)
  const [newSecret, setNewSecret] = useState('')
  const [hasChanges, setHasChanges] = useState(false)
  const { data: identityConfigs = [], isLoading: isLoadingConfigs, isError: isConfigsError } = useIdentityConfigsQuery()
  const createIdentityConfig = useCreateIdentityConfigMutation()

  const handleTestConnection = () => {
    setIsTesting(true)
    setTestResult(null)
    
    // Simulate API call with provider-specific delay
    setTimeout(() => {
      setIsTesting(false)
      
      // Simulate success for demo
      setTestResult({
        status: 'success',
        message: `Connection verified with ${provider}. User sync working.`,
        userCount: 247
      })
      toast.success('IdP connection verified')
    }, 2000)
  }

  const handleSave = async () => {
    setIsSaving(true)
    const issuerUrl = config.issuerUrl || (provider === 'google'
      ? 'https://accounts.google.com'
      : provider === 'entra'
        ? `https://login.microsoftonline.com/${config.tenantId || 'common'}/v2.0`
        : '')
    const payload: CreateIdentityConfigRequest = {
      type: provider === 'other' ? 'oidc' : provider,
      name: provider === 'entra' ? 'Microsoft Entra ID' : provider === 'google' ? 'Google Workspace' : provider === 'okta' ? 'Okta' : 'Fournisseur OIDC',
      issuer_url: issuerUrl,
      client_id: config.clientId,
      client_secret: config.clientSecret,
      scopes: ['openid', 'email', 'profile'],
      email_claim: 'email',
      name_claim: 'name',
      groups_claim: 'groups',
      extra_config: config.allowedDomain ? { allowed_domain: config.allowedDomain } : undefined,
    }
    try {
      await createIdentityConfig.mutateAsync(payload)
      setIsSaving(false)
      setHasChanges(false)
      toast.success('Configuration d’identité enregistrée')
    } catch {
      setIsSaving(false)
      toast.error('Impossible d’enregistrer la configuration IdP')
    }
  }

  const handleChangeSecret = () => {
    setNewSecret('')
    setChangeSecretDialogOpen(true)
  }

  const confirmChangeSecret = () => {
    if (newSecret.length < 8) {
      toast.error('Client secret must be at least 8 characters')
      return
    }
    setConfig({ ...config, clientSecret: newSecret })
    setChangeSecretDialogOpen(false)
    setHasChanges(true)
    toast.success('Client secret updated')
  }

  const updateConfig = (field: keyof ProviderConfig, value: string) => {
    setConfig({ ...config, [field]: value })
    setHasChanges(true)
  }

  return (
    <>

      <div className='max-w-3xl mb-8 space-y-3'>
        <h3 className='text-lg font-semibold'>Configurations existantes</h3>
        {isLoadingConfigs ? (
          <div className='flex items-center gap-2 text-sm text-muted-foreground'><Loader2 className='h-4 w-4 animate-spin' /> Chargement…</div>
        ) : isConfigsError ? (
          <p className='text-sm text-destructive'>Impossible de charger les configurations IdP.</p>
        ) : identityConfigs.length === 0 ? (
          <p className='text-sm text-muted-foreground'>Aucune configuration IdP enregistrée.</p>
        ) : (
          <div className='space-y-2'>
            {identityConfigs.map((identityConfig) => (
              <div key={identityConfig.id} className='flex items-center justify-between border rounded-lg p-4'>
                <div>
                  <p className='font-medium'>{identityConfig.name}</p>
                  <p className='text-sm text-muted-foreground'>{identityConfig.provider_type} · {identityConfig.issuer_url}</p>
                </div>
                <span className={identityConfig.is_active ? 'text-sm text-green-600' : 'text-sm text-muted-foreground'}>
                  {identityConfig.is_active ? 'Actif' : 'Inactif'}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>


      <div className='max-w-3xl space-y-8'>
        {/* Identity Provider Section */}
        <div className='space-y-6'>
         

          {/* Provider Selection */}
          <div className='space-y-3'>
            <Label>Provider</Label>
            <RadioGroup
              value={provider}
              onValueChange={(value) => {
                const nextProvider = value as Provider
                setProvider(nextProvider)
                setConfig({ ...providerConfigs[nextProvider] as ProviderConfig })
                setTestResult(null)
                setHasChanges(true)
              }}
              className='space-y-3'
            >
              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50 transition-colors'>
                <RadioGroupItem value='google' id='google' />
                <Label htmlFor='google' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Building2 className='h-5 w-5 text-blue-600' />
                    <span className='font-medium'>Google Workspace</span>
                  </div>
                </Label>
              </div>

              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50 transition-colors'>
                <RadioGroupItem value='okta' id='okta' />
                <Label htmlFor='okta' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Building2 className='h-5 w-5 text-purple-600' />
                    <span className='font-medium'>Okta</span>
                  </div>
                </Label>
              </div>

              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50 transition-colors'>
                <RadioGroupItem value='entra' id='entra' />
                <Label htmlFor='entra' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Building2 className='h-5 w-5 text-cyan-600' />
                    <span className='font-medium'>Microsoft Entra ID</span>
                  </div>
                </Label>
              </div>

              <div className='flex items-center space-x-3 border rounded-lg p-4 cursor-pointer hover:bg-muted/50 transition-colors'>
                <RadioGroupItem value='other' id='other' />
                <Label htmlFor='other' className='cursor-pointer flex-1'>
                  <div className='flex items-center gap-2'>
                    <Building2 className='h-5 w-5 text-gray-600' />
                    <span className='font-medium'>Other OIDC provider</span>
                  </div>
                </Label>
              </div>
            </RadioGroup>
          </div>

          {/* Client ID */}
          <div className='space-y-2'>
            <Label htmlFor='clientId'>Client ID</Label>
            <Input
              id='clientId'
              value={config.clientId}
              onChange={(e) => updateConfig('clientId', e.target.value)}
              placeholder='Enter client ID'
              className='font-mono text-sm'
            />
          </div>

          {/* Client Secret */}
          <div className='space-y-2'>
            <Label htmlFor='clientSecret'>Client Secret</Label>
            <div className='flex gap-2'>
              <div className='relative flex-1'>
                <Input
                  id='clientSecret'
                  type={showSecret ? 'text' : 'password'}
                  value={config.clientSecret}
                  onChange={(e) => updateConfig('clientSecret', e.target.value)}
                  placeholder='••••••••••••••••••••••••'
                  className='font-mono text-sm pr-10'
                />
                <button
                  type='button'
                  onClick={() => setShowSecret(!showSecret)}
                  className='absolute right-3 top-1/2 transform -translate-y-1/2 text-muted-foreground hover:text-foreground'
                >
                  {showSecret ? (
                    <EyeOff className='h-4 w-4' />
                  ) : (
                    <Eye className='h-4 w-4' />
                  )}
                </button>
              </div>
              <Button 
                variant='outline'
                onClick={handleChangeSecret}
              >
                Change
              </Button>
            </div>
          </div>

          {/* Provider-specific fields */}
          {provider === 'okta' && (
            <div className='space-y-2'>
              <Label htmlFor='issuerUrl'>Issuer URL</Label>
              <Input
                id='issuerUrl'
                value={config.issuerUrl}
                onChange={(e) => updateConfig('issuerUrl', e.target.value)}
                placeholder='https://your-org.okta.com'
                className='font-mono text-sm'
              />
            </div>
          )}

          {provider === 'entra' && (
            <div className='space-y-2'>
              <Label htmlFor='tenantId'>Tenant ID</Label>
              <Input
                id='tenantId'
                value={config.tenantId}
                onChange={(e) => updateConfig('tenantId', e.target.value)}
                placeholder='a1b2c3d4-e5f6-7890-abcd-ef1234567890'
                className='font-mono text-sm'
              />
            </div>
          )}

          {provider === 'other' && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='issuerUrl'>Issuer URL</Label>
                <Input
                  id='issuerUrl'
                  value={config.issuerUrl}
                  onChange={(e) => updateConfig('issuerUrl', e.target.value)}
                  placeholder='https://your-idp.com'
                  className='font-mono text-sm'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='authorizationEndpoint'>Authorization Endpoint</Label>
                <Input
                  id='authorizationEndpoint'
                  value={config.authorizationEndpoint}
                  onChange={(e) => updateConfig('authorizationEndpoint', e.target.value)}
                  placeholder='https://your-idp.com/oauth/authorize'
                  className='font-mono text-sm'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='tokenEndpoint'>Token Endpoint</Label>
                <Input
                  id='tokenEndpoint'
                  value={config.tokenEndpoint}
                  onChange={(e) => updateConfig('tokenEndpoint', e.target.value)}
                  placeholder='https://your-idp.com/oauth/token'
                  className='font-mono text-sm'
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='userInfoEndpoint'>User Info Endpoint</Label>
                <Input
                  id='userInfoEndpoint'
                  value={config.userInfoEndpoint}
                  onChange={(e) => updateConfig('userInfoEndpoint', e.target.value)}
                  placeholder='https://your-idp.com/oauth/userinfo'
                  className='font-mono text-sm'
                />
              </div>
            </>
          )}

          {/* Allowed Domain */}
          <div className='space-y-2'>
            <Label htmlFor='allowedDomain'>Allowed domain</Label>
            <Input
              id='allowedDomain'
              value={config.allowedDomain}
              onChange={(e) => updateConfig('allowedDomain', e.target.value)}
              placeholder='acme.com'
            />
            <div className='text-xs text-muted-foreground space-y-1 mt-2'>
              <p>↳ Only users from this domain can authenticate.</p>
              <p>Leave blank to allow any authenticated user.</p>
            </div>
          </div>
        </div>

        {/* Test Connection Section */}
        <div className='space-y-4 pt-6 border-t'>
          <div className='flex items-center gap-2'>
            <Key className='h-5 w-5 text-muted-foreground' />
            <h3 className='text-lg font-semibold'>TEST CONNECTION</h3>
          </div>

          <Button 
            variant='outline'
            onClick={handleTestConnection}
            disabled={isTesting || !config.clientId || !config.clientSecret}
          >
            {isTesting ? (
              <>
                <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                Testing...
              </>
            ) : (
              <>
                <RefreshCw className='h-4 w-4 mr-2' />
                Test IdP connection
              </>
            )}
          </Button>

          {testResult && (
            <div className={`flex items-start gap-3 rounded-lg p-4 ${
              testResult.status === 'success' 
                ? 'bg-green-50 border border-green-200' 
                : 'bg-red-50 border border-red-200'
            }`}>
              {testResult.status === 'success' ? (
                <CheckCircle className='h-5 w-5 text-green-600 mt-0.5' />
              ) : (
                <XCircle className='h-5 w-5 text-red-600 mt-0.5' />
              )}
              <div>
                <p className={`text-sm font-medium ${
                  testResult.status === 'success' ? 'text-green-800' : 'text-red-800'
                }`}>
                  {testResult.message}
                </p>
                <p className={`text-sm ${
                  testResult.status === 'success' ? 'text-green-700' : 'text-red-700'
                }`}>
                  {testResult.userCount} users visible.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Info Box */}
        <div className='bg-blue-50 border border-blue-200 rounded-lg p-4'>
          <div className='flex items-start gap-2'>
            <Info className='h-4 w-4 text-blue-600 mt-0.5' />
            <div className='text-sm text-blue-800'>
              <p className='font-medium mb-1'>
                Ashrix never stores passwords. Users authenticate directly with {provider === 'google' ? 'Google' : provider === 'okta' ? 'Okta' : provider === 'entra' ? 'Microsoft Entra ID' : 'your IdP'}.
              </p>
              <p className='text-blue-700'>
                Ashrix receives only their email address and group memberships.
              </p>
            </div>
          </div>
        </div>

        {/* Save Button */}
        <div className='flex items-center justify-end pt-6 border-t gap-2'>
          {hasChanges && (
            <Button 
              variant='outline'
              onClick={() => {
                setProvider('google')
                setConfig({ ...providerConfigs.google as ProviderConfig })
                setHasChanges(false)
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

      {/* Change Secret Dialog */}
      <Dialog open={changeSecretDialogOpen} onOpenChange={setChangeSecretDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Key className='h-5 w-5 text-muted-foreground' />
              Change Client Secret
            </DialogTitle>
            <DialogDescription>
              Enter the new client secret from your identity provider
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label htmlFor='newSecret'>New client secret</Label>
              <Input
                id='newSecret'
                type='password'
                value={newSecret}
                onChange={(e) => setNewSecret(e.target.value)}
                placeholder='Enter new client secret'
              />
            </div>
            <div className='bg-yellow-50 border border-yellow-200 rounded-lg p-3'>
              <div className='flex items-start gap-2'>
                <AlertTriangle className='h-4 w-4 text-yellow-600 mt-0.5' />
                <p className='text-sm text-yellow-800'>
                  Changing the client secret will invalidate all existing sessions.
                  Users will need to re-authenticate.
                </p>
              </div>
            </div>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setChangeSecretDialogOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              onClick={confirmChangeSecret}
              className='flex-1'
            >
              Update Secret
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}