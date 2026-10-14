import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { History, CheckCircle, Loader2 } from 'lucide-react'
import type { Policy } from '@/types/policy'
import { usePolicyHistoryQuery } from '@/hooks/policy.hooks'
import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface VersionHistoryProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  policy: Policy | null
}

export function VersionHistory({
  open,
  onOpenChange,
  policy,
}: VersionHistoryProps) {
  const { data: mutations, isLoading } = usePolicyHistoryQuery(policy?.id || '')
  const [selectedMutation, setSelectedMutation] = useState<any>(null)


  return (
    <>
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col'>
        <SheetHeader className='text-start'>
          <SheetTitle className='flex items-center gap-2'>
            <History className='h-5 w-5' />
            Historique des versions
          </SheetTitle>
          <SheetDescription>
            {policy?.ruleName}
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 overflow-y-auto py-4'>
          {isLoading ? (
            <div className='flex justify-center p-8'>
              <Loader2 className='animate-spin h-6 w-6 text-muted-foreground' />
            </div>
          ) : mutations?.map((mut: any, idx: number) => (
            <div 
              key={mut.Version} 
              className={`relative pb-8 cursor-pointer hover:bg-muted/30 transition-colors rounded-lg p-2 -ml-2 ${idx !== mutations.length - 1 ? 'border-l-2 border-gray-200 ml-4 pl-6' : 'ml-4 pl-6'}`}
              onClick={() => setSelectedMutation(mut)}
            >
              <div className='absolute left-[-9px] top-4 w-3 h-3 rounded-full bg-blue-500 border-2 border-white'></div>
              
              <div className='space-y-2 mt-2'>
                <div className='flex items-center gap-2'>
                  <h4 className='text-sm font-semibold'>
                    v{mut.Version}
                  </h4>
                  {idx === 0 && (
                    <span className='inline-flex items-center gap-1 text-xs text-green-600'>
                      <CheckCircle className='h-3 w-3' /> Actuelle
                    </span>
                  )}
                  <span className='text-xs font-semibold px-2 py-0.5 rounded bg-muted'>{mut.Op}</span>
                </div>
                <div className='text-xs text-muted-foreground'>
                  {new Date(mut.MutatedAt).toLocaleString('fr-FR')}
                </div>
                <p className='text-xs'>Cliquez pour voir les détails de cette version.</p>
                {/* Rollback button removed or hidden inside details for now */}
              </div>
            </div>
          ))}
        </div>
      </SheetContent>
    </Sheet>
    
    {/* Mutation Details Dialog */}
    <Dialog open={!!selectedMutation} onOpenChange={(o) => { if (!o) setSelectedMutation(null) }}>
      <DialogContent className='max-w-2xl max-h-[80vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>Détails de la version {selectedMutation?.Version}</DialogTitle>
          <DialogDescription>
            Opération: {selectedMutation?.Op} | Modifié le: {selectedMutation && new Date(selectedMutation.MutatedAt).toLocaleString('fr-FR')}
          </DialogDescription>
        </DialogHeader>

        {selectedMutation?.Snapshot && (
          <div className='space-y-4'>
            <div className='grid grid-cols-2 gap-4'>
              <div>
                <p className='text-xs text-muted-foreground'>Nom</p>
                <p className='text-sm font-medium'>{selectedMutation.Snapshot.name}</p>
              </div>
              <div>
                <p className='text-xs text-muted-foreground'>Effet</p>
                <p className='text-sm font-medium'>{selectedMutation.Snapshot.effect}</p>
              </div>
              <div>
                <p className='text-xs text-muted-foreground'>Priorité</p>
                <p className='text-sm font-medium'>{selectedMutation.Snapshot.priority}</p>
              </div>
            </div>
            
            <p className='text-xs text-muted-foreground'>Description</p>
            <p className='text-sm'>{selectedMutation.Snapshot.description}</p>
            
            <p className='text-sm font-medium mb-1'>Sujets (Subjects)</p>
            <pre className='bg-muted/50 rounded-lg p-4 text-xs whitespace-pre-wrap overflow-x-auto'>
              {JSON.stringify(selectedMutation.Snapshot.subject, null, 2)}
            </pre>

            <p className='text-sm font-medium mb-1'>Ressources (Resources)</p>
            <pre className='bg-muted/50 rounded-lg p-4 text-xs whitespace-pre-wrap overflow-x-auto'>
              {JSON.stringify(selectedMutation.Snapshot.resource, null, 2)}
            </pre>

            <p className='text-sm font-medium mb-1'>Conditions</p>
            <pre className='bg-muted/50 rounded-lg p-4 text-xs whitespace-pre-wrap overflow-x-auto'>
              {JSON.stringify(selectedMutation.Snapshot.conditions, null, 2)}
            </pre>
          </div>
        )}
      </DialogContent>
    </Dialog>
    </>
  )
}