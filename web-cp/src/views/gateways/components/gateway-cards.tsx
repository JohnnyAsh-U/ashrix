import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  // DropdownMenuContent,
  // DropdownMenuItem,
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  CheckCircle,
  XCircle,
  Copy,
  MoreVertical,
  AlertTriangle,
  FileText,
  RefreshCw,
  Trash2,
  Loader2,
  Activity,
  Clock,
  Layers,
  Users,
  Shield,
  Globe,
  Globe2,
  File,
  List,
  Rotate3D,
  Loader,
  TriangleAlert
} from 'lucide-react'
import type { GatewayApiResponse, GatewayInstruction } from '@/types/gateway'
import { toast } from 'sonner'
import { formatSeconds } from '@/utils/datetime'
import {
  useRotateGatewayCertificateMutation,
  useRevokeGatewayMutation,
  useGatewayQuery,
  useUpdateGatewayMutation,
  useDrainGatewayMutation,
} from '@/hooks/gateway.hooks'

const revokeReasons = [
  { value: 'keyCompromise', label: 'Compromission de la clé' },
  { value: 'superseded', label: 'Certificat remplacé' },
  { value: 'cessationOfOperation', label: "Cessation d'activité" },
  { value: 'affiliationChanged', label: "Changement d'affiliation" },
] as const

interface GatewayCardsProps {
  gateways: GatewayApiResponse[]
  isLoading?: boolean
}

export function GatewayCards({ gateways, isLoading }: GatewayCardsProps) {
  const [actionDialog, setActionDialog] = useState<'rotate' | 'revoke' | 'drain' | null>(null)
  const [selectedGateway, setSelectedGateway] = useState<GatewayApiResponse | null>(null)
  const [reenrollOpen, setReenrollOpen] = useState(false)
  const [reenrollForm, setReenrollForm] = useState({
    name: '',
    ip_address: '',
    public_url: '',
    log_to_cp: false,
  })
  const [reenrolledGatewayId, setReenrolledGatewayId] = useState<string | null>(null)
  const [revokeReason, setRevokeReason] = useState('')
  const rotateGatewayCert = useRotateGatewayCertificateMutation()
  const revokeGateway = useRevokeGatewayMutation()
  const reenrollGateway = useUpdateGatewayMutation()
  const drainGateway = useDrainGatewayMutation()
  const { data: reenrolledGateway } = useGatewayQuery(
    reenrolledGatewayId,
    Boolean(reenrolledGatewayId),
  )
  const [instructions, setInstructions] = useState<GatewayInstruction | null>(null)
  const reenrollHealthy = reenrolledGateway?.status === 'healthy'

  const openActionDialog = (gateway: GatewayApiResponse, action: 'rotate' | 'revoke' | 'drain') => {
    setSelectedGateway(gateway)
    setRevokeReason('')
    setActionDialog(action)
  }

  const closeActionDialog = () => {
    if (rotateGatewayCert.isPending || revokeGateway.isPending || drainGateway.isPending) return
    setActionDialog(null)
    setSelectedGateway(null)
    setRevokeReason('')
  }

  const displayStatus = (status: string, revokedAt?: string | null) => {
    if (revokedAt != null) {
      return (
        <p className={`text-sm flex items-center gap-1 text-red-600`}>
          <XCircle className='h-4 w-4' />Révoqué
        </p>
      )
    } else if (status === "draining") {
      return (
        <p className={`text-sm flex items-center gap-1 text-orange-600`}>
          <Rotate3D className='h-4 w-4' />Mode Maintenance
        </p>
      )
    } else if (status === "offline") {
      return (
        <p className={`text-sm flex items-center gap-1 text-red-600`}>
          <XCircle className='h-4 w-4' />Hor Ligne
        </p>
      )
    } else if (status === "pending") {
      return (<p className={`text-sm flex items-center gap-1 text-orange-600`}>
        <Loader className='h-4 w-4' />En Attente
      </p>)
    } else if (status === "healthy") {
      return (<p className={`text-sm flex items-center gap-1 text-green-600`}>
        <CheckCircle className='h-4 w-4' />Opérationnelle
      </p>)
    } else {
      return (<p className={`text-sm flex items-center gap-1 text-red-600`}>
        <CheckCircle className='h-4 w-4' />Indisponible
      </p>)
    }
  }

  const confirmAction = async () => {
    if (!selectedGateway) return

    try {
      if (actionDialog === 'rotate') {
        await rotateGatewayCert.mutateAsync(selectedGateway.id)
        toast.success(`Certificat de ${selectedGateway.name} renouvelé`)
      } else if (actionDialog === 'drain') {
        await drainGateway.mutateAsync(selectedGateway.id)
        toast.success(`Passerelle en Mode Maintenance`)
      } else if (actionDialog === 'revoke' && revokeReason) {
        await revokeGateway.mutateAsync({
          id: selectedGateway.id,
          revoke_reason: revokeReason,
        })
        toast.success(`Passerelle ${selectedGateway.name} révoqué`)
      }
      closeActionDialog()
    } catch {
      toast.error(actionDialog === 'rotate'
        ? 'Impossible de renouveler le certificat'
        : 'Impossible de révoquer le certificat')
    }
  }

  const openReenrollSheet = (gateway: GatewayApiResponse) => {
    setSelectedGateway(gateway)
    setReenrolledGatewayId(null)
    setReenrollForm({
      name: gateway.name,
      ip_address: gateway.ip_address,
      public_url: gateway.public_url,
      log_to_cp: gateway.log_to_cp,
    })
    setReenrollOpen(true)
  }

  const closeReenrollSheet = () => {
    if (reenrollGateway.isPending) return
    setReenrollOpen(false)
    setSelectedGateway(null)
    setReenrolledGatewayId(null)
  }

  const handleReenroll = async () => {
    if (!selectedGateway) return

    try {
      const gateway = await reenrollGateway.mutateAsync({
        id: selectedGateway.id,
        ...reenrollForm,
      })
      setReenrolledGatewayId(gateway.id)
      setInstructions(gateway.instruction)
      toast.success(`Réenrôlement de ${selectedGateway.name} lancé`)
    } catch {
      toast.error('Impossible de réenrôler la passerelle')
    }
  }


  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (gateways.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>Aucune passerelle trouvée</p>
      </div>
    )
  }

  return (
    <>
      <div className='space-y-6'>
        {gateways.map((gateway) => (
          <div
            key={gateway.id}
            className='border rounded-lg overflow-hidden bg-white'
          >
            {/* Header */}
            <div className='flex items-center justify-between p-6 pb-4'>
              <div className='flex items-center gap-3'>
                <div className='relative'>
                  {gateway.status == "healthy" ?
                    <>
                      <div className='w-3 h-3 bg-green-500 rounded-full'></div>
                      <div className='absolute inset-0 w-3 h-3 bg-green-500 rounded-full animate-ping opacity-75'></div>

                    </>
                    : <>
                      <div className='w-3 h-3 bg-red-500 rounded-full'></div>

                    </>
                  }
                </div>
                <h3 className='text-lg font-semibold'>{gateway.name}</h3>
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant='ghost' size='sm'>
                    Gérer <MoreVertical className='h-4 w-4 ml-1' />
                  </Button>
                </DropdownMenuTrigger>
                {/* <DropdownMenuContent align='end' className='w-48'>
                  <DropdownMenuItem>
                    <Settings className='h-4 w-4 mr-2' /> Mettre a jour
                  </DropdownMenuItem>
                  {gateway.revoked_at == null && (
                    <DropdownMenuItem
                      onClick={() => openActionDialog(gateway, 'revoke')}
                      className='text-red-600'
                    >
                      <Trash2 className='h-4 w-4 mr-2' /> Révoquer
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent> */}
              </DropdownMenu>
            </div>

            {/* Body */}
            <div className='px-6 pb-4'>
              <div className='border-t pt-4'>
                <div className='grid grid-cols-2 lg:grid-cols-3 gap-4 mb-4'>
                  <div className='flex items-center gap-2'>
                    <Shield className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>État</p>
                      {displayStatus(gateway.status, gateway.revoked_at)}
                    </div>
                  </div>

                  {gateway.revoked_at != null && gateway.revoke_reason && (
                    <div className='flex items-center gap-2'>
                      <AlertTriangle className='h-4 w-4 text-red-600' />
                      <div>
                        <p className='text-sm font-medium'>Motif de révocation</p>
                        <p className='text-sm text-red-600'>
                          {revokeReasons.find(({ value }) => value === gateway.revoke_reason)?.label ?? gateway.revoke_reason}
                        </p>
                      </div>
                    </div>
                  )}

                  <div className='flex items-center gap-2'>
                    <Globe2 className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>URL Publique</p>
                      <a href={`https://${gateway.public_url}`} className='text-sm'>https://{gateway.public_url}</a>
                    </div>
                  </div>

                  <div className='flex items-center gap-2'>
                    <Globe2 className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Adresse IP</p>
                      <p className='text-sm'>{gateway.ip_address}</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Globe className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Version</p>
                      <p className='text-sm'>{gateway.version}</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Activity className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Latence</p>
                      {/* <p className='text-sm'>{gateway.latency} ({gateway.latencyType})</p> */}
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Clock className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Disponibilité</p>
                      <p className='text-sm'>{formatSeconds(gateway.uptime)}</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Globe className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Version de la Politique</p>
                      <p className='text-sm'>v{gateway.current_policy_version}/v{gateway.policy_version}</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Layers className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Applications</p>
                      <p className='text-sm'>{gateway.number_of_apps} connectée(s)</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Users className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Sessions</p>
                      <p className='text-sm'>{gateway.number_of_session_active} active(s)</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <RefreshCw className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Dernière synchronisation</p>
                      <p className='text-sm'>{gateway.last_heartbeat ? new Date(gateway.last_heartbeat).toLocaleString() : '-'}</p>
                    </div>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Shield className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Certificat Emis le</p>
                      <p className='text-sm'>{gateway.certificate_issued_at ? new Date(gateway.certificate_issued_at).toLocaleDateString() : '-'}</p>
                    </div>

                  </div>

                  <div className='flex items-center gap-2'>
                    <Shield className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Certificat Expire le</p>
                      <p className='text-sm'>{gateway.certificate_expires_at ? new Date(gateway.certificate_expires_at).toLocaleDateString() : '-'}</p>
                    </div>

                  </div>

                  <div className='flex items-center gap-2'>
                    <File className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Enregistree le</p>
                      <p className='text-sm'>{gateway.enrolled_at ? new Date(gateway.enrolled_at).toLocaleDateString() : '-'}</p>
                    </div>

                  </div>

                  <div className='flex items-center gap-2'>
                    <List className='h-4 w-4 text-muted-foreground' />
                    <div>
                      <p className='text-sm font-medium'>Envoyer les logs au CP</p>
                      <p className='text-sm flex items-center gap-1 text-green-600'>
                        {gateway.log_to_cp ? <CheckCircle className='h-4 w-4' /> : <XCircle className='h-4 w-4 text-red-600' />}
                      </p>
                    </div>

                  </div>


                </div>

                {/* App Health */}
                <div>
                  <p className='text-sm font-medium mb-2'>État des applications :</p>
                  <div className='flex flex-wrap items-center gap-2'>
                    {gateway.apps.slice(0, 5).map((app, idx) => (
                      <span key={idx} className='inline-flex items-center gap-1 text-sm'>
                        {app.name} {app.health_status === 'healthy' ? (
                          <CheckCircle className='h-4 w-4 text-green-600' />
                        ) : app.health_status === "unknown" ? (
                          <TriangleAlert className='h-4 w-4 text-orange-600' />
                        ) : <XCircle className='h-4 w-4 text-red-600' />}
                      </span>
                    ))}
                    {gateway.apps.length > 5 && (
                      <span className='text-sm text-muted-foreground'>
                        (+{gateway.apps.length - 5} autres)
                      </span>
                    )}
                  </div>
                </div>
              </div>
            </div>

            {/* Footer */}
            <div className='px-6 py-4 bg-muted/30 border-t flex gap-2'>

              <Button
                variant='outline'
                size='sm'
                className='text-green-600'
                onClick={() => openReenrollSheet(gateway)}
                disabled={reenrollGateway.isPending}
              >
                {reenrollGateway.isPending ? <Loader2 className='h-4 w-4 mr-1 animate-spin' /> : <RefreshCw className='h-4 w-4 mr-1' />}
                Réenrôler
              </Button>

              <>
                {gateway.status === "healthy" && <Button
                  variant='outline'
                  size='sm'
                  onClick={() => openActionDialog(gateway, 'rotate')}
                >
                  <FileText className='h-4 w-4 mr-1' /> Renouveler le certificat
                </Button>}
                {gateway.revoked_at == null && <Button
                  variant='outline'
                  size='sm'
                  className='text-orange-600'
                  onClick={() => openActionDialog(gateway, 'drain')}
                >
                  <Rotate3D className='h-4 w-4 mr-1' /> Mode Maintenance
                </Button>}
                {gateway.revoked_at == null && <Button
                  variant='outline'
                  size='sm'
                  className='text-red-600'
                  onClick={() => openActionDialog(gateway, 'revoke')}
                >
                  <Trash2 className='h-4 w-4 mr-1' /> Révoquer
                </Button>}
              </>

            </div>
          </div>
        ))}
      </div>

      <Dialog open={actionDialog !== null} onOpenChange={(open) => !open && closeActionDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              {actionDialog === 'revoke' && <><AlertTriangle className='h-5 w-5 text-red-600' />Révoquer le certificat </>}
              {actionDialog === 'rotate' && <> <AlertTriangle className='h-5 w-5 text-red-600' /> Renouveler le certificat </>}
              {actionDialog === 'drain' && <> <AlertTriangle className='h-5 w-5 text-red-600' /> Mode Maintenance </>}
            </DialogTitle>
            <DialogDescription>
              {actionDialog === 'revoke' && <>Voulez-vous vraiment révoquer le certificat de cette passerelle ? </>}
              {actionDialog === 'rotate' && <> Voulez-vous vraiment renouveler le certificat de cette passerelle ? </>}
              {actionDialog === 'drain' && <> Voulez-vous vraiment passer en Mode Maintenance </>}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <p className='text-sm'>Passerelle : <span className='font-medium'>{selectedGateway?.name}</span></p>
            {actionDialog === 'revoke' && (
              <div className='space-y-2'>
                <Label htmlFor='revoke-reason'>Motif de révocation</Label>
                <Select value={revokeReason} onValueChange={setRevokeReason}>
                  <SelectTrigger id='revoke-reason' className='w-full'>
                    <SelectValue placeholder='Sélectionner un motif' />
                  </SelectTrigger>
                  <SelectContent>
                    {revokeReasons.map((reason) => (
                      <SelectItem key={reason.value} value={reason.value}>{reason.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
            {actionDialog === 'revoke' && (
              <div className='bg-red-50 border border-red-200 rounded-lg p-3'>
                <p className='text-sm text-red-700'>Cette action est irréversible.</p>
              </div>
            )}
          </div>

          <DialogFooter className='flex gap-2'>
            <Button variant='outline' onClick={closeActionDialog} className='flex-1'>
              Annuler
            </Button>
            <Button
              variant={actionDialog === 'revoke' ? 'destructive' : 'default'}
              onClick={confirmAction}
              disabled={!selectedGateway || (actionDialog === 'revoke' && !revokeReason) || rotateGatewayCert.isPending || revokeGateway.isPending}
              className='flex-1'
            >
              {(rotateGatewayCert.isPending || revokeGateway.isPending || drainGateway.isPending) && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
              {actionDialog === 'revoke' && <>Révoquer </>}
              {actionDialog === 'rotate' && <> Renouveler </>}
              {actionDialog === 'drain' && <>Mode Maintenance </>}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={reenrollOpen} onOpenChange={(open) => !open ? closeReenrollSheet() : setReenrollOpen(true)}>
        <SheetContent className='flex flex-col'>
          <SheetHeader className='text-start'>
            <SheetTitle>Réenrôler la passerelle</SheetTitle>
            <SheetDescription>
              Modifiez les informations de la passerelle avant de lancer son réenrôlement.
            </SheetDescription>
          </SheetHeader>

          <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
            {!reenrolledGatewayId ? (
              <>
                <div className='space-y-2'>
                  <Label htmlFor='reenroll-name'>Nom de la passerelle</Label>
                  <Input
                    id='reenroll-name'
                    value={reenrollForm.name}
                    onChange={(event) => setReenrollForm({ ...reenrollForm, name: event.target.value })}
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='reenroll-ip-address'>Adresse IP</Label>
                  <Input
                    id='reenroll-ip-address'
                    value={reenrollForm.ip_address}
                    onChange={(event) => setReenrollForm({ ...reenrollForm, ip_address: event.target.value })}
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='reenroll-public-url'>URL publique</Label>
                  <Input
                    id='reenroll-public-url'
                    value={reenrollForm.public_url}
                    onChange={(event) => setReenrollForm({ ...reenrollForm, public_url: event.target.value })}
                  />
                </div>

                <div className='flex items-center gap-2'>
                  <Checkbox
                    id='reenroll-log-to-cp'
                    checked={reenrollForm.log_to_cp}
                    onCheckedChange={(checked) => setReenrollForm({ ...reenrollForm, log_to_cp: checked === true })}
                  />
                  <Label htmlFor='reenroll-log-to-cp'>Envoyer les logs au CP</Label>
                </div>
              </>
            ) : reenrollHealthy ? (
              <div className='rounded-lg border border-green-200 bg-green-50 p-6 text-center'>
                <CheckCircle className='mx-auto mb-3 h-12 w-12 text-green-600' />
                <h3 className='text-lg font-semibold text-green-800'>Passerelle réenrôlée</h3>
                <p className='mt-1 text-sm text-green-700'>La passerelle est maintenant opérationnelle.</p>
              </div>
            ) : (
              <div className='space-y-4'>
                <div>
                  <p className='mb-2 text-sm font-medium'>1. Téléchargez le binaire :</p>
                  <div className='relative rounded-lg bg-gray-900 p-4'>
                    <pre className='whitespace-pre-wrap text-sm font-mono text-green-400'>{instructions?.download_cmd}</pre>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => navigator.clipboard.writeText(reenrolledGateway?.download_cmd ?? '')}
                      className='absolute right-2 top-2 text-gray-400 hover:text-white'
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                </div>

                <div>
                  <p className='mb-2 text-sm font-medium'>2. Réenrôlez et démarrez :</p>
                  <div className='relative rounded-lg bg-gray-900 p-4'>
                    <pre className='whitespace-pre-wrap text-sm font-mono text-green-400'>
                      {[instructions?.enroll_cmd, instructions?.start_cmd].filter(Boolean).join('\n\n')}
                    </pre>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => navigator.clipboard.writeText([reenrolledGateway?.enroll_cmd, reenrolledGateway?.start_cmd].filter(Boolean).join('\n\n'))}
                      className='absolute right-2 top-2 text-gray-400 hover:text-white'
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                </div>

                <div className='rounded-lg border border-blue-200 bg-blue-50 p-4'>
                  <div className='flex items-center gap-2 text-blue-800'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    <span className='text-sm font-medium'>En attente de la connexion de la passerelle…</span>
                  </div>
                  <p className='mt-1 text-sm text-blue-700'>L’état sera mis à jour automatiquement.</p>
                </div>
              </div>
            )}
          </div>

          <SheetFooter>
            <Button variant='outline' onClick={closeReenrollSheet} disabled={reenrollGateway.isPending}>
              Annuler
            </Button>
            {!reenrolledGatewayId && (
              <Button
                onClick={handleReenroll}
                disabled={
                  reenrollGateway.isPending ||
                  !reenrollForm.name.trim() ||
                  !reenrollForm.ip_address.trim() ||
                  !reenrollForm.public_url.trim()
                }
              >
                {reenrollGateway.isPending && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
                Réenrôler
              </Button>
            )}
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}