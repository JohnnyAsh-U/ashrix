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
  DropdownMenuContent,
  DropdownMenuItem,
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
  MoreHorizontal,
  CheckCircle,
  XCircle,
  Eye,
  History,
  Edit,
  Trash2,
  Loader2,
} from 'lucide-react'
import type { Policy } from '@/types/policy'
import { toast } from 'sonner'
import { useDeletePolicyMutation } from '@/hooks/policy.hooks'
import { useNavigate } from '@tanstack/react-router'

interface PoliciesTableProps {
  policies: Policy[]
  isLoading?: boolean
  onViewVersionHistory: (policy: Policy) => void
}

export function PoliciesTable({ 
  policies, 
  isLoading, 
  onViewVersionHistory 
}: PoliciesTableProps) {
  const [selectedPolicy, setSelectedPolicy] = useState<Policy | null>(null)
  const [viewDialogOpen, setViewDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const deletePolicy = useDeletePolicyMutation()

  const navigate = useNavigate()

  const handleViewPolicy = (policy: Policy) => {
    setSelectedPolicy(policy)
    setViewDialogOpen(true)
  }

  const handleDeletePolicy = (policy: Policy) => {
    setSelectedPolicy(policy)
    setDeleteDialogOpen(true)
  }

  const confirmDelete = async () => {
    if (!selectedPolicy) return
    try {
      await deletePolicy.mutateAsync(selectedPolicy.id)
      toast.success('Politique supprimée')
      setDeleteDialogOpen(false)
      setSelectedPolicy(null)
    } catch { toast.error('Impossible de supprimer la politique') }
  }


  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (policies.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>Aucune politique trouvée</p>
      </div>
    )
  }

  return (
    <>
      <div className='border rounded-lg overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Nom</TableHead>
              <TableHead>Application</TableHead>
              <TableHead>Groupes</TableHead>
              <TableHead>Utilisateurs</TableHead>
              <TableHead>Effet</TableHead>
              <TableHead>État</TableHead>
              <TableHead className='text-right'>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {policies.map((policy) => (
              <TableRow 
                key={policy.id} 
                className='cursor-pointer hover:bg-muted/50'
                onClick={() => handleViewPolicy(policy)}
              >
                <TableCell className='font-medium'>{policy.ruleName}</TableCell>
                <TableCell>{policy.app}</TableCell>
                <TableCell>{policy.groups?.join(', ') || '-'}</TableCell>
                <TableCell title={policy.users?.join(', ')}>
                  {policy.users 
                    ? (policy.users.join(', ').length > 30 ? policy.users.join(', ').substring(0, 30) + '...' : policy.users.join(', '))
                    : '-'}
                </TableCell>
                <TableCell>
                  <span className={`px-2 py-1 rounded text-xs font-medium ${policy.effect === 'ALLOW' ? 'bg-green-100 text-green-800' : policy.effect === 'DENY' ? 'bg-red-100 text-red-800' : 'bg-gray-100 text-gray-800'}`}>
                    {policy.effect || '-'}
                  </span>
                </TableCell>
                <TableCell>
                  {policy.status === 'active' ? (
                    <span className='inline-flex items-center gap-1 text-green-600'>
                      <CheckCircle className='h-4 w-4' /> active
                    </span>
                  ) : (
                    <span className='inline-flex items-center gap-1 text-gray-600'>
                      <XCircle className='h-4 w-4' /> désactivée
                    </span>
                  )}
                </TableCell>
                <TableCell className='text-right' onClick={(e) => e.stopPropagation()}>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant='ghost' size='icon'>
                        <MoreHorizontal className='h-4 w-4' />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align='end' className='w-56'>
                      <DropdownMenuItem onClick={() => handleViewPolicy(policy)}>
                        <Eye className='h-4 w-4 mr-2' /> Voir
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => navigate({to: "/update-policy/"+policy.id})}>
                        <Edit className='h-4 w-4 mr-2' /> Modifier
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => onViewVersionHistory(policy)}>
                        <History className='h-4 w-4 mr-2' /> Historique des versions
                      </DropdownMenuItem>
                      <DropdownMenuItem 
                        onClick={() => handleDeletePolicy(policy)}
                        className='text-red-600'
                      >
                        <Trash2 className='h-4 w-4 mr-2' /> Supprimer
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* View Policy Dialog */}
      <Dialog open={viewDialogOpen} onOpenChange={setViewDialogOpen}>
        <DialogContent className='max-w-2xl'>
          <DialogHeader>
            <DialogTitle>{selectedPolicy?.ruleName}</DialogTitle>
            <DialogDescription>
              Détails et configuration de la politique
            </DialogDescription>
          </DialogHeader>

          {selectedPolicy && (
            <div className='space-y-6'>
              <div className='grid grid-cols-2 gap-4'>
                <div>
                  <p className='text-sm font-medium text-muted-foreground'>Application</p>
                  <p className='text-sm'>{selectedPolicy.app}</p>
                </div>
                <div>
                  <p className='text-sm font-medium text-muted-foreground'>Priorité</p>
                  <p className='text-sm'>{selectedPolicy.priority}</p>
                </div>
                <div>
                  <p className='text-sm font-medium text-muted-foreground'>Effet</p>
                  <p className='text-sm font-medium'>{selectedPolicy.effect}</p>
                </div>
                <div>
                  <p className='text-sm font-medium text-muted-foreground'>État</p>
                  <p className='text-sm capitalize'>{selectedPolicy.status}</p>
                </div>
                {selectedPolicy.description && (
                  <div className='col-span-2'>
                    <p className='text-sm font-medium text-muted-foreground'>Description</p>
                    <p className='text-sm'>{selectedPolicy.description}</p>
                  </div>
                )}
              </div>

              <div>
                <p className='text-sm font-medium mb-2'>Sujets (Subjects)</p>
                <div className='bg-muted/50 rounded-lg p-4 space-y-4'>
                  {selectedPolicy.originalPolicy?.subjects?.map((s: any, idx: number) => (
                    <div key={idx} className='flex gap-2 items-center text-sm'>
                      <span className='font-semibold capitalize text-indigo-600 bg-indigo-50 px-1.5 py-0.5 rounded'>
                        {s.type}
                      </span>
                      <span>{s.value}</span>
                    </div>
                  ))}
                </div>
              </div>

              <div>
                <p className='text-sm font-medium mb-2'>Ressources (Resources)</p>
                <div className='bg-muted/50 rounded-lg p-4 space-y-4'>
                  {selectedPolicy.originalPolicy?.resources?.map((r: any, idx: number) => (
                    <div key={idx} className='flex gap-2 items-center text-sm'>
                      <span className='font-semibold capitalize text-purple-600 bg-purple-50 px-1.5 py-0.5 rounded'>
                        {r.type}
                      </span>
                      <span>{r.value}</span>
                    </div>
                  ))}
                </div>
              </div>
              
              <div>
                <p className='text-sm font-medium mb-2'>Conditions</p>
                <div className='bg-muted/50 rounded-lg p-4 text-sm'>
                   <pre className='whitespace-pre-wrap'>{JSON.stringify(selectedPolicy.originalPolicy?.conditions, null, 2)}</pre>
                </div>
              </div>

              <div className='flex items-center justify-between text-sm text-muted-foreground border-t pt-4'>
                <div className='flex items-center gap-2'>
                  <History className='h-4 w-4' />
                  <span>Version {selectedPolicy.version}</span>
                </div>
                <div className='flex flex-col text-right text-xs'>
                  <span>Créé le: {new Date(selectedPolicy.originalPolicy?.created_at).toLocaleString('fr-FR')}</span>
                  <span>Modifié le: {selectedPolicy.lastModified} par {selectedPolicy.modifiedBy}</span>
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setViewDialogOpen(false)}>
              Fermer
            </Button>
            <Button onClick={() => {
              setViewDialogOpen(false)
              if (selectedPolicy) {
                onViewVersionHistory(selectedPolicy)
              }
            }}>
              Voir l’historique des versions
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Policy</DialogTitle>
            <DialogDescription>
              Are you sure you want to delete this policy?
            </DialogDescription>
          </DialogHeader>

          <div className='bg-muted/50 rounded-lg p-4'>
            <p className='text-sm font-medium'>{selectedPolicy?.ruleName}</p>
            <p className='text-sm text-muted-foreground mt-1'>
              This action cannot be undone.
            </p>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setDeleteDialogOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              variant='destructive'
              onClick={confirmDelete}
              className='flex-1'
            >
              Supprimer la politique
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}