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
import { Badge } from '@/components/ui/badge'
import {
  ChevronDown,
  ChevronRight,
  Edit,
  Trash2,
  CheckCircle,
  XCircle,
  Lock,
  Globe,
  Plus,
  Loader2,
  TriangleAlert,
  AlertTriangle
} from 'lucide-react'
import type { ApplicationApiResponse } from '@/types/application'
import { useDeleteApplicationMutation, useUpdateApplicationMutation } from '@/hooks/application.hooks'
import { toast } from 'sonner'
import { useConnectorsQuery } from '@/hooks/connector.hooks'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'


const displayStatus = (status: string) => {
  if (status === "healthy") {
    return (<p className={`text-sm flex items-center gap-1 text-green-600`}>
      <CheckCircle className='h-4 w-4' />Opérationnelle
    </p>)
  } else if (status === "unhealthy") {
    return (<p className={`text-sm flex items-center gap-1 text-red-600`}>
      <XCircle className='h-4 w-4' />Indisponible
    </p>)
  } else {
    return (<p className={`text-sm flex items-center gap-1 text-orange-600`}>
      <TriangleAlert className='h-4 w-4' />Inconnu
    </p>)
  }
}


interface ApplicationsTableProps {
  applications: ApplicationApiResponse[]
  isLoading?: boolean
}



export function ApplicationsTable({ applications, isLoading }: ApplicationsTableProps) {
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())
  const [application, setApplication] = useState<ApplicationApiResponse>()
  const deleteApplication = useDeleteApplicationMutation()
  const editApplication = useUpdateApplicationMutation()
  const [actionDialog, setActionDialog] = useState<'edit' | 'delete' | null>(null)
  const { data: connectors } = useConnectorsQuery()
  const [editForm, setEditForm] = useState({
    name: '',
    sock_pass: '',
    check_health: false,
    check_interval: 0,
    connector_id: "",
    enable_security_headers: true,
    health_endpoint: "",
    is_public: true,
    protocol: "http" as "http" | "ssh" | "tcp",
    subdomain: "string",
    upstream: ""
  })


  const openActionDialog = (app: ApplicationApiResponse, action: 'edit' | 'delete') => {
    setApplication(app)
    if (action === 'edit') {
      setEditForm({
        name: app.name,
        sock_pass: '',
        check_health: app.check_health,
        check_interval: app.check_interval,
        connector_id: app?.connector_id ?? "",
        enable_security_headers: app.enable_security_headers,
        health_endpoint: app.health_endpoint,
        is_public: app.is_public,
        protocol: app.protocol as "http" | "ssh" | "tcp",
        subdomain: app.subdomain,
        upstream: app.upstream
      })
      setActionDialog(action)
    }
  }

  const closeActionDialog = () => {
    if (editApplication.isPending || deleteApplication.isPending) return
    setActionDialog(null)
    setApplication(undefined)
  }

  const toggleExpand = (appId: string) => {
    const newExpanded = new Set(expandedRows)
    if (newExpanded.has(appId)) {
      newExpanded.delete(appId)
    } else {
      newExpanded.add(appId)
    }
    setExpandedRows(newExpanded)
  }

  const confirmAction = async () => {
    if (!application) return

    try {
      if (actionDialog === 'delete') {
        await deleteApplication.mutateAsync(application.id)
        toast.success(`Appli ${application.name} supprimé`)
      } else if (actionDialog === 'edit') {
        await editApplication.mutateAsync({
          id: application.id,
          ...editForm,
        })
        toast.success(`${application.name} Modifié`)
      }
      closeActionDialog()
    } catch {
      toast.error("Impossible de modifier/supprimer l'appli")
    }
  }


  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (applications.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>Aucune application trouvée</p>
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
              <TableHead>Nom d’hôte</TableHead>
              <TableHead>État</TableHead>
              <TableHead>Politiques</TableHead>
              <TableHead>Utilisateurs</TableHead>
              <TableHead>Groupes</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {applications.map((app) => (
              <>
                <TableRow
                  key={app.id}
                  className='cursor-pointer hover:bg-muted/50 h-12'
                  onClick={() => toggleExpand(app.id)}
                >
                  <TableCell>
                    {expandedRows.has(app.id) ? (
                      <ChevronDown className='h-4 w-4' />
                    ) : (
                      <ChevronRight className='h-4 w-4' />
                    )}
                  </TableCell>
                  <TableCell className='font-medium'>
                    <div className='flex items-center gap-2'>
                      {app.name}
                      {app.protocol === 'SSH' && (
                        <Badge variant='secondary' className='text-xs'>SSH</Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>{app.name}</TableCell>
                  <TableCell>
                    {displayStatus(app.health_status)}
                  </TableCell>
                  <TableCell>
                    {app.policies.length} {app.policies.length === 1 ? 'règle' : 'règles'}
                  </TableCell>
                  <TableCell>{app.number_of_users}</TableCell>
                  <TableCell>{app.number_of_groups}</TableCell>
                </TableRow>
                {expandedRows.has(app.id) && (
                  <TableRow key={`${app.id}-expanded`}>
                    <TableCell colSpan={7} className='bg-muted/30 p-0'>
                      <ExpandedAppDetails app={app} openActionDialog={openActionDialog} />
                    </TableCell>
                  </TableRow>
                )}
              </>
            ))}
          </TableBody>
        </Table>
      </div>



      {/* Revoke Confirmation Dialog */}
      <Dialog open={actionDialog == 'delete'} onOpenChange={(open) => !open && closeActionDialog()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5 text-red-600' />Supprimer l'appli
            </DialogTitle>
            <DialogDescription>
              Voulez-vous vraiment supprimer l'appli ?
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <p className='text-sm'>Connecteur : <span className='font-medium'>{application?.name}</span></p>
            <div className='bg-red-50 border border-red-200 rounded-lg p-3'>
              <p className='text-sm text-red-700'>Cette action est irréversible.</p>

            </div>
          </div>
          <DialogFooter className='flex gap-2'>
            <Button variant='outline' onClick={closeActionDialog} className='flex-1'>
              Annuler
            </Button>
            <Button
              variant={'destructive'}
              onClick={confirmAction}
              disabled={!application || deleteApplication.isPending}
              className='flex-1'
            >
              {(deleteApplication.isPending) && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
              Supprimer
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Sheet open={actionDialog == 'edit'} onOpenChange={(open) => !open && closeActionDialog()}>
        <SheetContent className='flex flex-col'>
          <SheetHeader className='text-start'>
            <SheetTitle>Réenrôler le Connecteur</SheetTitle>
            <SheetDescription>
              Modifiez les informations de du connecteur avant de lancer son réenrôlement.
            </SheetDescription>
          </SheetHeader>

          <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
            <>
              <div className='space-y-2'>
                <Label htmlFor='reenroll-name'>Nom du connecteur</Label>
                <Input
                  id='reenroll-name'
                  value={editForm.name}
                  onChange={(event) => setEditForm({ ...editForm, name: event.target.value })}
                />
              </div>

              {/* Public Hostname */}
              <div className='space-y-2'>
                <Label htmlFor='hostname'>Sousdomaine public</Label>
                <div className='flex items-center gap-2'>
                  <Input
                    id='hostname'
                    placeholder='crm'
                    value={editForm.subdomain}
                    onChange={(e) => setEditForm({ ...editForm, subdomain: e.target.value })}
                    className='flex-1'
                  />
                  {/* <span className='text-sm text-muted-foreground'>.acme.com</span> */}
                </div>

              </div>

              {/* Internal Upstream URL */}
              <div className='space-y-2'>
                <Label htmlFor='upstream'>URL interne de destination</Label>
                <Input
                  id='upstream'
                  placeholder='localhost:8080'
                  value={editForm.upstream}
                  onChange={(e) => setEditForm({ ...editForm, upstream: e.target.value })}
                />
              </div>


              {/* Protocol */}
              <div className='space-y-2'>
                <Label>Protocole</Label>
                <RadioGroup
                  value={editForm.protocol}
                  onValueChange={(value) => setEditForm({ ...editForm, protocol: value as "http" | "ssh" | "tcp" })}
                  className='space-y-2'
                >
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='http' id='http' />
                    <Label htmlFor='http'>HTTP / HTTPS</Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='tcp' id='tcp' />
                    <Label htmlFor='tcp'>TCP</Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='ssh' id='ssh' />
                    <Label htmlFor='ssh'>SSH</Label>
                  </div>
                </RadioGroup>
              </div>

              {/* Connector */}
              <div className='space-y-2'>
                <Label htmlFor='connector'>Connecteur</Label>
                <Select
                  value={editForm.connector_id}
                  onValueChange={(value) => setEditForm({ ...editForm, connector_id: value })}
                >
                  <SelectTrigger id='connector'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {connectors?.map((connector) => (
                      <SelectItem key={connector.id} value={connector.id}>
                        {connector.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {/* Visibility */}
              <div className='space-y-2'>
                <Label>Visibilité</Label>
                <RadioGroup
                  defaultValue={editForm.is_public ? '1' : '0'}
                  onValueChange={(value) => setEditForm({ ...editForm, is_public: value === '1' })}
                  className='space-y-2'
                >
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='0' id='private' />
                    <Label htmlFor='private'>Privée (connexion requise)</Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='1' id='public' />
                    <Label htmlFor='public'>Publique (sans connexion)</Label>
                  </div>
                </RadioGroup>
              </div>


              {/* Connector */}
              <div className='space-y-2'>
                <Label htmlFor='connector'>EN-TÊTES DE SÉCURITÉ</Label>
                <Select
                  defaultValue={editForm.enable_security_headers ? '1' : '0'}
                  onValueChange={(value) => setEditForm({ ...editForm, enable_security_headers: value == '1' })}
                >
                  <SelectTrigger id='connector'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={'1'}>
                      Oui
                    </SelectItem>
                    <SelectItem value={'0'}>
                      Non
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>


              {/* Check Health */}
              <div className='space-y-2'>
                <Label>Verifier le statut</Label>
                <RadioGroup
                  defaultValue={editForm.check_health ? '1' : '0'}
                  onValueChange={(val) => setEditForm({ ...editForm, check_health: val === '1' })}
                  className='space-y-2'
                >
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='1' id='private' />
                    <Label htmlFor='1'>Oui</Label>
                  </div>
                  <div className='flex items-center space-x-2'>
                    <RadioGroupItem value='0' id='public' />
                    <Label htmlFor='0'>Non</Label>
                  </div>
                </RadioGroup>
              </div>

              <div className='space-y-2'>
                <Label>Path de la sante</Label>
                <Input
                  id=''
                  placeholder='health'
                  value={editForm.health_endpoint}
                  onChange={(e) => setEditForm({ ...editForm, health_endpoint: e.target.value })}
                />
              </div>
              <div className='space-y-2'>
                <Label>Intervale</Label>
                <Input
                  id=''
                  placeholder='/health'
                  value={editForm.check_interval}
                  onChange={(e) => setEditForm({ ...editForm, check_interval: Number(e.target.value) })}
                />
              </div>

              <div className='space-y-2'>
                <Label>Mot de Passe SOCK5</Label>
                <Input
                  id=''
                  placeholder='Mot de passe...'
                  value={""}
                  onChange={(e) => setEditForm({ ...editForm, sock_pass: e.target.value })}
                />
              </div>
            </>
          </div>
          <SheetFooter>
            <Button variant='outline' onClick={closeActionDialog} disabled={editApplication.isPending}>
              Annuler
            </Button>
            <Button
              onClick={confirmAction}
              disabled={editApplication.isPending}
            >
              {editApplication.isPending && <Loader2 className='h-4 w-4 mr-2 animate-spin' />}
              Réenrôler
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}

function ExpandedAppDetails({ app, openActionDialog }: { app: ApplicationApiResponse, openActionDialog: (app: ApplicationApiResponse, action: "edit" | "delete") => void }) {
  return (
    <div className='p-6 space-y-6'>
      {/* Basic Info */}
      <div>
        <h3 className='text-lg font-semibold mb-4'>{app.name}</h3>
        <div className='grid grid-cols-2 gap-4'>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Sous Domaine</p>
            <p className='text-sm'>{app.subdomain}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Destination</p>
            <p className='text-sm'>{app.upstream}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Protocole</p>
            <p className='text-sm'>{app.protocol}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Passerelle</p>
            <p className='text-sm'>{app.gateway_name}</p>
          </div>
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Connecteur</p>
            <p className='text-sm'>{app.connector_name}</p>
          </div>
          {app.check_health && <div>
            <p className='text-sm font-medium text-muted-foreground'>Sante</p>
            <p className='text-sm flex items-center gap-1'>
              <CheckCircle className='h-4 w-4 text-green-600' /> {app.health_endpoint} @ {app.check_interval}s
            </p>
          </div>}
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Visibilité</p>
            <p className='text-sm flex items-center gap-1'>
              {!app.is_public ? (
                <>
                  <Lock className='h-4 w-4' /> Privée (connexion requise)
                </>
              ) : (
                <>
                  <Globe className='h-4 w-4' /> Publique (sans connexion)
                </>
              )}
            </p>
          </div>
        </div>
      </div>

      <div className='grid grid-cols-2 gap-4'>
        <div>
          <h4 className='text-sm font-semibold mb-2'>EN-TÊTES DE SÉCURITÉ</h4>
          <div className='flex items-center gap-2'>
            <span className='text-sm flex items-center gap-1'>
              {app.enable_security_headers && <><CheckCircle className='h-4 w-4 text-green-600' /> activés</>}
              {!app.enable_security_headers && <><XCircle className='h-4 w-4 text-orange-600' /> </>}
            </span>
          </div>
        </div>
      </div>



      {/* Connector Status */}
      {app.health_status === 'healthy' && (
        <div className='flex items-center gap-2 text-sm'>
          <CheckCircle className='h-4 w-4 text-green-600' />
          <span className='font-medium'>{app.name}</span>
          <span className='text-muted-foreground'>· dernier contrôle {app.last_seen ? new Date(app.last_seen).toLocaleString(): "-"}</span>
        </div>
      )}

      {app.health_status === 'unknown' && (

        <div className='bg-red-50 border border-red-200 rounded-lg p-4 space-y-3'>
          <div className='flex items-center gap-2 text-sm text-red-700'>
            <TriangleAlert className='h-4 w-4' />
            <span className='font-medium'>{app.name} · INCONNU depuis {app.last_seen ? new Date(app.last_seen).toLocaleString() : "-"}</span>
          </div>
        </div>
      )}

      {app.health_status === 'unhealthy' && (
        <div className='bg-red-50 border border-red-200 rounded-lg p-4 space-y-3'>
          <div className='flex items-center gap-2 text-sm text-red-700'>
            <XCircle className='h-4 w-4' />
            <span className='font-medium'>{app.name} · DÉCONNECTÉ depuis {app.last_seen ? new Date(app.last_seen).toLocaleString(): "-"}</span>
          </div>
          <p className='text-sm text-red-600'>
            Les utilisateurs de {app.name} voient « Application indisponible ».
          </p>
        </div>
      )}



      {/* Policies */}
      <div>
        <div className='flex items-center justify-between mb-3'>
          <h4 className='text-sm font-semibold'>POLITIQUES</h4>
          <Button variant='outline' size='sm'>
            <Plus className='h-4 w-4 mr-1' /> Ajouter une règle
          </Button>
        </div>
        <div className='space-y-2'>
          {app.policies.map((policy) => (
            <div key={policy.id} className='flex items-center justify-between py-2 border-b last:border-0'>
              <div className='flex items-center gap-2'>
                <CheckCircle className='h-4 w-4 text-green-600' />
                <span className='text-sm'>{policy.name} → {policy.priority}</span>
                <span className='text-xs text-muted-foreground'>(priorité {policy.priority})</span>
              </div>
              <div className='flex items-center gap-2'>
                <Button variant='ghost' size='sm'>Modifier</Button>
                <Button variant='ghost' size='sm'>···</Button>
              </div>
            </div>
          ))}
          <div className='text-sm text-muted-foreground py-2'>
            ─ Par défaut : TOUT REFUSER
          </div>
        </div>
      </div>

      {/* Traffic */}
      {/* {app.health_status === 'healthy'  && ( */}
      <div>
        <h4 className='text-sm font-semibold mb-2'>TRAFIC (dernière heure)</h4>
        <p className='text-sm text-muted-foreground'>
          {app.number_of_traffic_today} requêtes
        </p>
      </div>
      {/* )} */}


      {/* Action Buttons */}
      <div className='flex gap-2 pt-4 border-t'>
        <Button variant='outline' size='sm' onClick={() => openActionDialog(app, "edit")}>
          <Edit className='h-4 w-4 mr-1' /> Modifier
        </Button>
        {/* <Button variant='outline' size='sm'>
          <FileText className='h-4 w-4 mr-1' /> Journaux d’accès
        </Button> */}
        <Button variant='outline' size='sm' className='text-red-600' onClick={() => openActionDialog(app, "delete")}>
          <Trash2 className='h-4 w-4 mr-1' /> Supprimer
        </Button>
      </div>
    </div>
  )
}