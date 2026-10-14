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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Copy, Clock, Loader2, CheckCircle } from 'lucide-react'
import { toast } from 'sonner'
import { useConnectorQuery, useCreateConnectorMutation } from '@/hooks/connector.hooks'
import { useGatewaysQuery } from '@/hooks/gateway.hooks'
import { Checkbox } from '@/components/ui/checkbox'
import type { ConnectorInstruction } from '@/types/connector'

interface GenerateTokenFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function GenerateTokenForm({
  open,
  onOpenChange,
}: GenerateTokenFormProps) {
  const [formData, setFormData] = useState({
     name: '',
    gateway_id: '',
    secondary_gateway_id: '',
    open_sock: false,
  })
  const [createdConnectorId, setCreatedConnectorId] = useState<string | null>(null)
  const [instructions, setInstruction] = useState<ConnectorInstruction | null>(null)

  const [generatedToken, setGeneratedToken] = useState<string | null>(null)
  const [timeRemaining, setTimeRemaining] = useState(600) // 59:59 in seconds
  const [isGenerating, setIsGenerating] = useState(false)
  const createConnector = useCreateConnectorMutation()
  const { data: gateways = [] } = useGatewaysQuery()
    const { data: createdConnector } = useConnectorQuery(createdConnectorId, Boolean(generatedToken))
    const connectionConnected = createdConnector?.status === 'connected'
  

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setFormData({ name: '', gateway_id: '', secondary_gateway_id: '', open_sock: false })
      setGeneratedToken(null)
      setTimeRemaining(600)
      setIsGenerating(false)
    }
    onOpenChange(nextOpen)
  }

  useEffect(() => {
    if (generatedToken && timeRemaining > 0) {
      const timer = setInterval(() => {
        setTimeRemaining(prev => prev - 1)
      }, 1000)
      return () => clearInterval(timer)
    }
  }, [generatedToken, timeRemaining])

  const handleGenerateToken = async () => {
    setIsGenerating(true)
    try {
      const connector = await createConnector.mutateAsync({
        name: formData.name,
        gateway_id: formData.gateway_id,
        secondary_gateway_id: formData.secondary_gateway_id == '' ? undefined : formData.secondary_gateway_id,
        open_sock: true,
      })
      if (connector.token) {
        setGeneratedToken(connector.token)
        setCreatedConnectorId(connector.id)
        setInstruction(connector.instruction)
        toast.success('Jeton généré avec succès')
      } else {
        toast.success('Connecteur créé')
        onOpenChange(false)
      }
    } catch {
      toast.error('Impossible de créer le connecteur')
    } finally {
      setIsGenerating(false)
    }
  }

  const handleCopyDownload = () => {
       const command = createdConnector?.download_cmd ?? ''

    
    navigator.clipboard.writeText(command)
    toast.success('Commande de téléchargement copiée')
  }


    const handleCopyEnroll = () => {
    const command = [createdConnector?.start_cmd].filter(Boolean).join('\n\n')
    
    navigator.clipboard.writeText(command)
    toast.success('Commande d’enrôlement copiée')
  }

  const formatTime = (seconds: number) => {
    const minutes = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className='flex flex-col'>
        <SheetHeader className='text-start'>
          <SheetTitle>Enrôler un connecteur</SheetTitle>
          <SheetDescription>
            Créez un jeton d’enrôlement à usage unique pour un nouveau connecteur
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
          {!generatedToken ? (
            <>
              {/* Connector Name */}
              <div className='space-y-2'>
                <Label htmlFor='connectorName'>Nom du connecteur</Label>
                <Input
                  id='connectorName'
                  placeholder='crm-connector-2'
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                />
              </div>

              {/* Gateway */}
                         <div className='space-y-2'>
                  <Label htmlFor='passerelle'>Passerelle 1</Label>
                  <Select
                    value={formData.gateway_id}
                    onValueChange={(value) => setFormData({ ...formData, gateway_id: value })}
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
                    value={formData.secondary_gateway_id}
                    onValueChange={(value) => setFormData({ ...formData, secondary_gateway_id: value })}
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
                    checked={formData.open_sock}
                    onCheckedChange={(checked) => setFormData({ ...formData, open_sock: checked === true })}
                  />
                  <Label htmlFor='reenroll-log-to-cp'>Utiliser SOCK5 (Port: 1080)</Label>
                </div>
            </>
          ) : !connectionConnected ? (
            <>
              {/* Generated Token Display */}
              <div className='space-y-4'>
                 <div>
                  <p className='text-sm font-medium mb-2'>1. Téléchargez le binaire :</p>
                  <div className='bg-gray-900 rounded-lg p-4 relative'>
                    <pre className='text-sm text-green-400 font-mono whitespace-pre-wrap'>{instructions?.download_cmd}</pre>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={handleCopyDownload}
                      className='absolute top-2 right-2 text-gray-400 hover:text-white'
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                </div>

                  <div>
                  <p className='text-sm font-medium mb-2'>2. Démarrez :</p>
                  <div className='bg-gray-900 rounded-lg p-4 relative'>
                    <pre className='text-sm text-green-400 font-mono whitespace-pre-wrap'>{[instructions?.start_cmd].filter(Boolean).join('\n\n')}</pre>
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={handleCopyEnroll}
                      className='absolute top-2 right-2 text-gray-400 hover:text-white'
                    >
                      <Copy className='h-4 w-4' />
                    </Button>
                  </div>
                </div>

                  <div className='flex items-center justify-between'>
                  <Button 
                    variant='outline' 
                    size='sm'
                    onClick={handleCopyEnroll}
                  >
                    <Copy className='h-4 w-4 mr-1' /> Copier
                  </Button>
                  <span className='text-sm text-muted-foreground flex items-center gap-1'>
                    <Clock className='h-4 w-4' /> Expiration du jeton : {formatTime(timeRemaining)}
                  </span>
                </div>


              

                <div className='bg-blue-50 border border-blue-200 rounded-lg p-4 space-y-2'>
                  <div className='flex items-center gap-2 text-blue-800'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                    <span className='text-sm font-medium'>En attente de la connexion du connecteur…</span>
                  </div>
                  <p className='text-sm text-blue-700'>
                    Le jeton est à usage unique.
                  </p>
                  <p className='text-sm text-blue-700'>
                    Il est invalidé dès son utilisation.
                  </p>
                    <p className='text-sm text-blue-700 mt-1'>
                      (l’état sera mis à jour lorsque le connecteur apparaîtra)
                  </p>
                </div>
              </div>
            </>
          ) : (
          <>
              {/* Connection Success */}
              <div className='space-y-4'>
                <div className='bg-green-50 border border-green-200 rounded-lg p-6 text-center'>
                  <CheckCircle className='h-12 w-12 text-green-600 mx-auto mb-3' />
                  <h3 className='text-lg font-semibold text-green-800 mb-1'>
                    {formData.name} est connectée.
                  </h3>
                  <p className='text-sm text-green-700'>
                    Version : v1.0.0 · 0 application · 0 session
                  </p>
                </div>
                <Button 
                  variant='outline' 
                  className='w-full'
                  onClick={() => handleOpenChange(false)}
                >
                  Ouvrir le connecteur →
                </Button>
              </div>
            </>
        )
        }
        </div>

        <SheetFooter>
          {!generatedToken ? (
            <div className='flex gap-2 pt-4'>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleOpenChange(false)}
                className='flex-1'
              >
                Annuler
              </Button>
              <Button 
                type='submit' 
                className='flex-1'
                onClick={handleGenerateToken}
                disabled={isGenerating || !formData.name || !formData.gateway_id}
              >
                {isGenerating ? (
                  <>
                    <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                    Génération…
                  </>
                ) : (
                  'Générer le jeton d’enrôlement'
                )}
              </Button>
            </div>
          ) : (
            <div className='flex gap-2 pt-4'>
              <Button
                type='button'
                variant='outline'
                onClick={() => handleOpenChange(false)}
                className='flex-1'
              >
                Fermer
              </Button>
            </div>
          )}
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}