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
import { Checkbox } from '@/components/ui/checkbox'
import { 
  Copy, 
  Clock, 
  Loader2, 
  CheckCircle,
} from 'lucide-react'
import { toast } from 'sonner'
import { useCreateGatewayMutation, useGatewayQuery } from '@/hooks/gateway.hooks'
import type { GatewayInstruction } from '@/types/gateway'

interface EnrollGatewayFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function EnrollGatewayForm({
  open,
  onOpenChange,
}: EnrollGatewayFormProps) {
  const [formData, setFormData] = useState({
    gatewayName: '',
    ipAddress: '',
    publicUrl: '',
    logToCp: true,
  })
  const [enrollmentToken, setEnrollmentToken] = useState<string | null>(null)
  const [createdGatewayId, setCreatedGatewayId] = useState<string | null>(null)
  const [instructions, setInstruction] = useState<GatewayInstruction | null>(null)
  const [timeRemaining, setTimeRemaining] = useState(600)
  const [isGenerating, setIsGenerating] = useState(false)
  const createGateway = useCreateGatewayMutation()
  const { data: createdGateway } = useGatewayQuery(createdGatewayId, Boolean(enrollmentToken))
  const connectionStatus = createdGateway?.status === 'healthy' ? 'connected' : 'pending'

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) {
      setFormData({ gatewayName: '', ipAddress: '', publicUrl: '', logToCp: true })
      setEnrollmentToken(null)
      setCreatedGatewayId(null)
      setTimeRemaining(600)
      setIsGenerating(false)
    }
    onOpenChange(nextOpen)
  }

  useEffect(() => {
    if (enrollmentToken && timeRemaining > 0) {
      const timer = setInterval(() => {
        setTimeRemaining(prev => prev - 1)
      }, 1000)
      return () => clearInterval(timer)
    }
  }, [enrollmentToken, timeRemaining])

  const handleGenerateToken = async () => {
    setIsGenerating(true)
    try {
      const gateway = await createGateway.mutateAsync({
        name: formData.gatewayName,
        ip_address: formData.ipAddress,
        public_url: formData.publicUrl,
        log_to_cp: formData.logToCp,
      })
      if (gateway.token) {
        setEnrollmentToken(gateway.token)
        setCreatedGatewayId(gateway.id)
        setInstruction(gateway.instruction)
        toast.success('Jeton d’enrôlement généré')
      } else {
        toast.success('Passerelle créée')
        onOpenChange(false)
      }
    } catch {
      toast.error('Impossible de créer la passerelle')
    } finally {
      setIsGenerating(false)
    }
  }

  const handleCopyDownload = () => {
    const command = createdGateway?.download_cmd ?? ''
    
    navigator.clipboard.writeText(command)
    toast.success('Commande de téléchargement copiée')
  }

  const handleCopyEnroll = () => {
    const command = [createdGateway?.enroll_cmd, createdGateway?.start_cmd].filter(Boolean).join('\n\n')
    
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
          <SheetTitle>Enrôler une passerelle</SheetTitle>
          <SheetDescription>
            Générez un jeton d’enrôlement et déployez une nouvelle passerelle
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
          {!enrollmentToken ? (
            <>
              {/* Gateway Name */}
              <div className='space-y-2'>
                <Label htmlFor='gatewayName'>Nom de la passerelle</Label>
                <Input
                  id='gatewayName'
                  placeholder='London HQ'
                  value={formData.gatewayName}
                  onChange={(e) => setFormData({ ...formData, gatewayName: e.target.value })}
                />
              </div>

              <div className='space-y-2'>
                <Label htmlFor='gateway-ip-address'>Adresse IP</Label>
                <Input
                  id='gateway-ip-address'
                  value={formData.ipAddress}
                  onChange={(event) => setFormData({ ...formData, ipAddress: event.target.value })}
                />
              </div>

              <div className='space-y-2'>
                <Label htmlFor='gateway-public-url'>URL publique</Label>
                <Input
                  id='gateway-public-url'
                  value={formData.publicUrl}
                  onChange={(event) => setFormData({ ...formData, publicUrl: event.target.value })}
                />
              </div>

              <div className='flex items-center gap-2'>
                <Checkbox
                  id='gateway-log-to-cp'
                  checked={formData.logToCp}
                  onCheckedChange={(checked) => setFormData({ ...formData, logToCp: checked === true })}
                />
                <Label htmlFor='gateway-log-to-cp'>Envoyer les logs au CP</Label>
              </div>
            </>
          ) : connectionStatus === 'pending' ? (
            <>
              {/* Enrollment Instructions */}
              <div className='space-y-4'>
                <div className='bg-blue-50 border border-blue-200 rounded-lg p-4'>
                  <p className='text-sm font-medium text-blue-800 mb-2'>
                    Jeton d’enrôlement généré
                  </p>
                  <div className='bg-gray-900 rounded-lg p-3'>
                    <code className='text-sm text-green-400 font-mono'>
                      {enrollmentToken}
                    </code>
                  </div>
                </div>

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
                  <p className='text-sm font-medium mb-2'>2. Enrôlez et démarrez :</p>
                  <div className='bg-gray-900 rounded-lg p-4 relative'>
                    <pre className='text-sm text-green-400 font-mono whitespace-pre-wrap'>{[instructions?.enroll_cmd, instructions?.start_cmd].filter(Boolean).join('\n\n')}</pre>
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

                <div className='bg-blue-50 border border-blue-200 rounded-lg p-4'>
                  <div className='flex items-center gap-2 text-blue-800'>
                    <Loader2 className='h-4 w-4 animate-spin' />
                      <span className='text-sm font-medium'>En attente de la connexion de la passerelle…</span>
                  </div>
                  <p className='text-sm text-blue-700 mt-1'>
                      (l’état sera mis à jour lorsque la passerelle apparaîtra)
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
                    {formData.gatewayName} est connectée.
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
                  Ouvrir la passerelle →
                </Button>
              </div>
            </>
          )}
        </div>

        <SheetFooter>
          {!enrollmentToken ? (
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
                disabled={isGenerating || !formData.gatewayName || !formData.ipAddress || !formData.publicUrl}
              >
                {isGenerating ? (
                  <>
                    <Loader2 className='h-4 w-4 mr-2 animate-spin' />
                    Génération…
                  </>
                ) : (
                  'Générer un jeton d’enrôlement'
                )}
              </Button>
            </div>
          ) : connectionStatus === 'pending' ? (
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