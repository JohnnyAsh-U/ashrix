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
  ChevronDown,
  ChevronRight,
  MoreHorizontal,
  CheckCircle,
  XCircle,
  AlertTriangle,
  RefreshCw,
  Ban,
  // Eye,
  Loader2,
  Copy,
  Loader,
  FileText,
  TriangleAlert
} from 'lucide-react'
import type { ConnectorApiResponse, ConnectorInstruction } from '@/types/connector'
import { toast } from 'sonner'
import { useConnectorQuery, useRevokeConnectorMutation, useRotateConnectorCertificateMutation, useUpdateConnectorMutation } from '@/hooks/connector.hooks'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { useGatewaysQuery } from '@/hooks/gateway.hooks'


const revokeReasons = [
  { value: 'keyCompromise', label: 'Compromission de la clé' },
  { value: 'superseded', label: 'Certificat remplacé' },
  { value: 'cessationOfOperation', label: "Cessation d'activité" },
  { value: 'affiliationChanged', label: "Changement d'affiliation" },
] as const



const displayStatus = (status: string, revokedAt?: string | null) => {
  if (revokedAt != null) {
    return (
      <p className={`text-sm flex items-center gap-1 text-red-600`}>
        <XCircle className='h-4 w-4' />Révoqué
      </p>
    )
  } else if (status === "disconnected") {
    return (
      <p className={`text-sm flex items-center gap-1 text-red-600`}>
        <XCircle className='h-4 w-4' />Hors Ligne
      </p>
    )
  } else if (status === "pending") {
    return (<p className={`text-sm flex items-center gap-1 text-orange-600`}>
      <Loader className='h-4 w-4' />En Attente
    </p>)
  } else if (status === "connected") {
    return (<p className={`text-sm flex items-center gap-1 text-green-600`}>
      <CheckCircle className='h-4 w-4' />Opérationnelle
    </p>)
  } else {
    return (<p className={`text-sm flex items-center gap-1 text-red-600`}>
      <CheckCircle className='h-4 w-4' />Indisponible
    </p>)
  }
}

interface ConnectorsTableProps {
  connectors: ConnectorApiResponse[]
  isLoading?: boolean
}

export function ConnectorsTable({ connectors, isLoading }: ConnectorsTableProps) {
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
  const [selectedConnector, setSelectedConnector] = useState<ConnectorApiResponse | null>(null)
  const [actionDialog, setActionDialog] = useState<'rotate' | 'revoke' | null>(null)
  const [revokeReason, setRevokeReason] = useState('')
  const [reenrollOpen, setReenrollOpen] = useState(false)
  const [instructions, setInstructions] = useState<ConnectorInstruction | null>(null)
  const { data: gateways } = useGatewaysQuery()
  const [reenrollForm, setReenrollForm] = useState({
    name: '',
    gateway_id: '',
    secondary_gateway_id: '',
    open_sock: false,
  })
  const [reenrolledConnectorId, setReenrolledConnectorId] = useState<string | null>(null)


  const rotateConnectorCert = useRotateConnectorCertificateMutation()
  const revokeConnector = useRevokeConnectorMutation()
  const reenrollConnector = useUpdateConnectorMutation()
  const { data: reenrolledConnector } = useConnectorQuery(
    reenrolledConnectorId,
    Boolean(reenrolledConnectorId)
  )

  const reenrollHealthy = reenrolledConnector?.status === 'connected'


  const openActionDialog = (gateway: ConnectorApiResponse, action: 'rotate' | 'revoke') => {
    setSelectedConnector(gateway)
    setRevokeReason('')
    setActionDialog(action)
  }

  const closeActionDialog = () => {
    if (rotateConnectorCert.isPending || revokeConnector.isPending) return
    setActionDialog(null)
    setSelectedConnector(null)
    setRevokeReason('')
  }

  const toggleExpand = (connectorId: string) => {
    const newExpanded = new Set(expandedRows)
    if (newExpanded.has(connectorId)) {
      newExpanded.delete(connectorId)
    } else {
      newExpanded.add(connectorId)
    }
    setExpandedRows(newExpanded)
  }

  const confirmAction = async () => {
    if (!selectedConnector) return

    try {
      if (actionDialog === 'rotate') {
        await rotateConnectorCert.mutateAsync(selectedConnector.id)
        toast.success(`Certificat de ${selectedConnector.name} renouvelé`)
      } else if (actionDialog === 'revoke' && revokeReason) {
        await revokeConnector.mutateAsync({
          id: selectedConnector.id,
          revoke_reason: revokeReason,
        })
        toast.success(`Connecteur ${selectedConnector.name} révoqué`)
      }
      closeActionDialog()
    } catch {
      toast.error('Impossible de renouveler/révoquer le certificat')
    }
  }


  const openReenrollSheet = (connector: ConnectorApiResponse) => {
    setSelectedConnector(connector)
    setReenrolledConnectorId(null)
    setReenrollForm({
      name: connector.name,
      gateway_id: connector.gateway_id,
      secondary_gateway_id: connector.secondary_gateway_id ?? '',
      open_sock: connector.open_sock,
    })
    setReenrollOpen(true)
  }

  const closeReenrollSheet = () => {
    if (reenrollConnector.isPending) return
    setReenrollOpen(false)
    setSelectedConnector(null)
    setReenrolledConnectorId(null)
  }

  const handleReenroll = async () => {
    if (!selectedConnector) return

    try {
      const connector = await reenrollConnector.mutateAsync({
        id: selectedConnector.id,
        ...reenrollForm,
        secondary_gateway_id: reenrollForm.secondary_gateway_id == ""? undefined : reenrollForm.secondary_gateway_id
      })
      setReenrolledConnectorId(connector.id)
      setInstructions(connector.instruction)
      toast.success(`Réenrôlement de ${selectedConnector.name} lancé`)
    } catch {
      toast.error('Impossible de réenrôler le connecteur')
    }
  }


  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (connectors.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>Aucun connecteur trouvé</p>
      </div>
    )
  }

  return (
    <>
      <div className='border rounded-lg overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className='w-8'></TableHead>
              <TableHead>Nom</TableHead>
              <TableHead>Applis</TableHead>
              <TableHead>Passerelle 1</TableHead>
              <TableHead>Passerelle 2</TableHead>
              <TableHead>Statut</TableHead>
              <TableHead>Dernière Sync</TableHead>
              <TableHead className='text-right'>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {connectors.map((connector) => (
              <>
                <TableRow
                  key={connector.id}
                  className='cursor-pointer hover:bg-muted/50'
                  onClick={() => toggleExpand(connector.id)}
                >
                  <TableCell>
                    {expandedRows.has(connector.id) ? (
                      <ChevronDown className='h-4 w-4' />
                    ) : (
                      <ChevronRight className='h-4 w-4' />
                    )}
                  </TableCell>
                  <TableCell className='font-medium'>{connector.name}</TableCell>
                  <TableCell>{connector.apps.length}</TableCell>
                  <TableCell>{connector.gateway_name}</TableCell>
                  <TableCell>{connector.secondary_gateway_name}</TableCell>
                  <TableCell>
                    {displayStatus(connector.status, connector.revoked_at)}

                  </TableCell>
                  <TableCell>{connector.last_seen ? new Date(connector.last_seen).toLocaleString() : '-'}</TableCell>
                  <TableCell className='text-right' onClick={(e) => e.stopPropagation()}>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant='ghost' size='icon'>
                          <MoreHorizontal className='h-4 w-4' />
                        </Button>
                      </DropdownMenuTrigger>
                      {/* <DropdownMenuContent align='end' className='w-48'>
                        <DropdownMenuItem onClick={() => toggleExpand(connector.id)}>
                          <Eye className='h-4 w-4 mr-2' /> View details
                        </DropdownMenuItem>
                        <DropdownMenuItem 
                          onClick={() => handleRevoke(connector)}
                          className='text-red-600'
                        >
                          <Ban className='h-4 w-4 mr-2' /> Revoke
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => handleReEnroll(connector)}>
                          <RefreshCw className='h-4 w-4 mr-2' /> Re-enroll
                        </DropdownMenuItem>
                      </DropdownMenuContent> */}
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
                {expandedRows.has(connector.id) && (
                  <TableRow key={`${connector.id}-expanded`}>
                    <TableCell colSpan={8} className='bg-muted/30 p-0'>
                      <ExpandedConnectorDetails connector={connector} openActionDialog={openActionDialog} openReenrollSheet={openReenrollSheet} />
                    </TableCell>
                  </TableRow>
                )}
              </>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Revoke Confirmation Dialog */}
      <Dialog open={actionDialog !== null} onOpenChange={(open) => !open && closeActionDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5 text-red-600' />
              {actionDialog === 'revoke' && <><AlertTriangle className='h-5 w-5 text-red-600' />Révoquer le connecteur </>}
              {actionDialog === 'rotate' && <> <AlertTriangle className='h-5 w-5 text-red-600' /> Renouveler le certificat </>}
            </DialogTitle>
            <DialogDescription>
              {actionDialog === 'revoke' && <>Voulez-vous vraiment révoquer le certificat de ce connecteur ? </>}
              {actionDialog === 'rotate' && <> Voulez-vous vraiment renouveler le certificat de ce connecteur ? </>}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <p className='text-sm'>Connecteur : <span className='font-medium'>{selectedConnector?.name}</span></p>

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
                <p className='text-sm text-red-700'>
                  Cet action va immediatement deconnecter le connecteur {selectedConnector?.apps.length} Applis. Les Utilisateur verront une page
                  "Application indisponible" jusqu'au connecteur soit reenroller.
                </p>
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
              disabled={!selectedConnector || (actionDialog === 'revoke' && !revokeReason) || rotateConnectorCert.isPending}
              className='flex-1'
            >
              {(rotateConnectorCert.isPending || revokeConnector.isPending) && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
              {actionDialog === 'revoke' && <>Révoquer </>}
              {actionDialog === 'rotate' && <> Renouveler </>}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={reenrollOpen} onOpenChange={(open) => !open ? closeReenrollSheet() : setReenrollOpen(true)}>
        <SheetContent className='flex flex-col'>
          <SheetHeader className='text-start'>
            <SheetTitle>Réenrôler le Connecteur</SheetTitle>
            <SheetDescription>
              Modifiez les informations de du connecteur avant de lancer son réenrôlement.
            </SheetDescription>
          </SheetHeader>

          <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
            {!reenrolledConnectorId ? (
              <>
                <div className='space-y-2'>
                  <Label htmlFor='reenroll-name'>Nom du connecteur</Label>
                  <Input
                    id='reenroll-name'
                    value={reenrollForm.name}
                    onChange={(event) => setReenrollForm({ ...reenrollForm, name: event.target.value })}
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='passerelle'>Passerelle 1</Label>
                  <Select
                    value={reenrollForm.gateway_id}
                    onValueChange={(value) => setReenrollForm({ ...reenrollForm, gateway_id: value })}
                  >
                    <SelectTrigger id='passerelle'>
                      <SelectValue placeholder='Sélectionner une passerelle' />
                    </SelectTrigger>
                    <SelectContent>
                      {gateways?.map(a => <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='passerelle1'>Passerelle 2</Label>
                  <Select
                    value={reenrollForm.secondary_gateway_id}
                    onValueChange={(value) => setReenrollForm({ ...reenrollForm, secondary_gateway_id: value })}
                  >
                    <SelectTrigger id='passerelle1'>
                      <SelectValue placeholder='Sélectionner une passerelle' />
                    </SelectTrigger>
                    <SelectContent>
                      {gateways?.map(a => <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>)}
                    </SelectContent>
                  </Select>
                </div>

                <div className='flex items-center gap-2'>
                  <Checkbox
                    id='reenroll-log-to-cp'
                    checked={reenrollForm.open_sock}
                    onCheckedChange={(checked) => setReenrollForm({ ...reenrollForm, open_sock: checked === true })}
                  />
                  <Label htmlFor='reenroll-log-to-cp'>Utiliser SOCK5(Port: 1080)</Label>
                </div>
              </>
            ) : reenrollHealthy ? (
              <div className='rounded-lg border border-green-200 bg-green-50 p-6 text-center'>
                <CheckCircle className='mx-auto mb-3 h-12 w-12 text-green-600' />
                <h3 className='text-lg font-semibold text-green-800'>Connecteur réenrôlé</h3>
                <p className='mt-1 text-sm text-green-700'>Le connecteur est maintenant opérationnelle.</p>
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
                      onClick={() => navigator.clipboard.writeText(instructions?.download_cmd ?? '')}
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
                      {[instructions?.start_cmd].filter(Boolean).join('\n\n')}
                    </pre>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => navigator.clipboard.writeText([instructions?.start_cmd].filter(Boolean).join('\n\n'))}
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
            <Button variant='outline' onClick={closeReenrollSheet} disabled={reenrollConnector.isPending}>
              Annuler
            </Button>
            {!reenrolledConnectorId && (
              <Button
                onClick={handleReenroll}
                disabled={
                  reenrollConnector.isPending ||
                  !reenrollForm.name.trim() ||
                  !reenrollForm.gateway_id.trim()
                }
              >
                {reenrollConnector.isPending && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
                Réenrôler
              </Button>
            )}
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}


function ExpandedConnectorDetails({ connector, openReenrollSheet, openActionDialog }: { connector: ConnectorApiResponse, openReenrollSheet: (connector: ConnectorApiResponse) => void, openActionDialog: (connector: ConnectorApiResponse, action: "rotate" | "revoke") => void }) {
  const handleCopyCommand = () => {
    const command = `./ashrix-connector start`
    navigator.clipboard.writeText(command)
    toast.success('Commande copiée dans le presse-papiers')
  }

  return (
    <div className='p-6 space-y-6'>
      {/* Connector Info */}
      <div>
        <h3 className='text-lg font-semibold mb-4'>{connector.name}</h3>
        <div className='grid grid-cols-2 gap-4'>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Nom</p>
            <p className='text-sm'>
              {connector.name}
            </p>
          </div>

            <div>
            <p className='text-sm font-medium text-muted-foreground'>Connexion QUIC</p>
            <p className='text-sm'>{connector.active_stream}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Passerelle 1</p>
            <p className='text-sm'>{connector.gateway_name}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Passerelle 2</p>
            <p className='text-sm'>{connector.secondary_gateway_name}</p>
          </div>

          <div>
            <p className='text-sm font-medium text-muted-foreground'>SOCKS5</p>
            <p className='text-sm'>{connector.open_sock ? <CheckCircle className='h-4 w-4 text-green-600' /> : <XCircle className='h-4 w-4 text-red-600' />}</p>
          </div>

       
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Statut</p>
            {displayStatus(connector.status, connector.revoked_at)}
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Dernière synchronisation</p>
            <p className='text-sm'>
              {connector.last_seen ? new Date(connector.last_seen).toLocaleString('fr-FR') : '-'}

            </p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Enrôlé le</p>
            <p className='text-sm'>
              {connector.enrolled_at ? new Date(connector.enrolled_at).toLocaleString('fr-FR') : '-'}
            </p>
          </div>
        </div>


      </div>
      {/* App Health */}
      <div>
        <p className='text-sm font-medium mb-2'>État des applications :</p>
        <div className='flex flex-wrap items-center gap-2'>
          {connector.apps.slice(0, 5).map((app, idx) => (
            <span key={idx} className='inline-flex items-center gap-1 text-sm'>
              {app.name} {app.health_status === 'healthy' ? (
                <CheckCircle className='h-4 w-4 text-green-600' />
              ) : app.health_status === "unknown" ? (
                <TriangleAlert className='h-4 w-4 text-orange-600' />
              ): <XCircle className='h-4 w-4 text-red-600'/>}
            </span>
          ))}
          {connector.apps.length > 5 && (
            <span className='text-sm text-muted-foreground'>
              (+{connector.apps.length - 5} autres)
            </span>
          )}
        </div>
      </div>

      {/* Disconnected Warning */}
      {connector.status === 'disconnected' && (
        <div className='bg-yellow-50 border border-yellow-200 rounded-lg p-4 space-y-3'>
          <div className='flex items-start gap-2'>
            <AlertTriangle className='h-5 w-5 text-yellow-600 mt-0.5' />
            <div className='text-sm text-yellow-800'>
              <p className='font-medium'>Les utilisateurs qui accèdent à {connector.name} voient</p>
                <p>« Connecteur indisponible. »</p>
            </div>
          </div>

          <div>
            <p className='text-sm font-medium mb-2'>POUR RESTAURER : lancez cette commande sur le serveur {connector.name}</p>
            <div className='bg-gray-900 rounded-lg p-3 flex items-center justify-between'>
              <code className='text-sm text-green-400 font-mono'>
                ./ashrix-connector start
              </code>
              <Button
                variant='ghost'
                size='sm'
                onClick={handleCopyCommand}
                className='text-gray-400 hover:text-white'
              >
                <Copy className='h-4 w-4' />
              </Button>
            </div>
          </div>

          <div>
            <p className='text-sm mb-2'>Si le binaire du connecteur a été supprimé, réenrôlez-le.</p>
          </div>
        </div>
      )}

      {/* Action Buttons */}

      <div className='flex gap-2 pt-4 border-t'>
        {connector.status === "connected" && <Button
          variant='outline'
          size='sm'
          onClick={() => openActionDialog(connector, 'rotate')}
        >
          <FileText className='h-4 w-4 mr-1' /> Renouveler le certificat
        </Button>}
        <Button variant='outline' size='sm' className='text-green-600' onClick={() => openReenrollSheet(connector)}>
          <RefreshCw className='h-4 w-4 mr-1' /> Réenrôler
        </Button>

        {connector.revoked_at == null && <Button variant='outline' size='sm' className='text-red-600'
          onClick={() => openActionDialog(connector, 'revoke')}
        >
          <Ban className='h-4 w-4 mr-1' /> Révoquer
        </Button>}

      </div>
    </div>
  )
}