import { useState } from 'react'
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Info } from 'lucide-react'
import { useCreateApplicationMutation } from '@/hooks/application.hooks'
import { useConnectorsQuery } from '@/hooks/connector.hooks'

interface AddApplicationFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPublicSelect: () => void
}

export function AddApplicationForm({
  open,
  onOpenChange,
  onPublicSelect,
}: AddApplicationFormProps) {
  const [formData, setFormData] = useState({
    name: '',
    subdomain: '',
    upstream: '',
    protocol: 'http',
    connector_id: '',
    is_public: '0',
    check_health: '0',
    check_interval: 0,
    health_endpoint: '',
    sock_pass: '',
    enable_security_headers: '1',
  })
  const createApplication = useCreateApplicationMutation()
  const { data: connectors = [] } = useConnectorsQuery()

  const handleSubmit = async () => {
    try {
      await createApplication.mutateAsync({
        name: formData.name,
        subdomain: formData.subdomain,
        upstream: formData.upstream,
        protocol: 'http',
        is_public: formData.is_public == '1',
        check_health: formData.check_health == '1',
        check_interval: formData.check_interval,
        health_endpoint: formData.health_endpoint,
        sock_pass: formData.sock_pass,
        connector_id: formData.connector_id,
        enable_security_headers: formData.enable_security_headers == '1'
      })
      onOpenChange(false)
    } catch {
      // The form remains open so the user can correct the invalid fields.
    }
  }

  const handleVisibilityChange = (value: string) => {
    setFormData({ ...formData, is_public: value })
    if (value == '1') {
      onPublicSelect()
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='flex flex-col'>
        <SheetHeader className='text-start'>
          <SheetTitle>Ajouter une application</SheetTitle>
          <SheetDescription>
            Configurez une nouvelle application protégée
          </SheetDescription>
        </SheetHeader>

        <div className='flex-1 space-y-6 overflow-y-auto px-4 py-4'>
          {/* Name */}
          <div className='space-y-2'>
            <Label htmlFor='name'>Nom</Label>
            <Input
              id='name'
              placeholder='CRM'
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            />
          </div>

          {/* Public Hostname */}
          <div className='space-y-2'>
            <Label htmlFor='hostname'>Sousdomaine public</Label>
            <div className='flex items-center gap-2'>
              <Input
                id='hostname'
                placeholder='crm'
                value={formData.subdomain}
                onChange={(e) => setFormData({ ...formData, subdomain: e.target.value })}
                className='flex-1'
              />
              <span className='text-sm text-muted-foreground'>.acme.com</span>
            </div>
            <p className='text-xs text-muted-foreground'>
              ↳ Le certificat sera émis automatiquement
            </p>
          </div>

          {/* Internal Upstream URL */}
          <div className='space-y-2'>
            <Label htmlFor='upstream'>URL interne de destination</Label>
            <Input
              id='upstream'
              placeholder='localhost:8080'
              value={formData.upstream}
              onChange={(e) => setFormData({ ...formData, upstream: e.target.value })}
            />
          </div>

          {/* Protocol */}
          <div className='space-y-2'>
            <Label>Protocole</Label>
            <RadioGroup
              value={formData.protocol}
              onValueChange={(value) => setFormData({ ...formData, protocol: value })}
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
              value={formData.connector_id}
              onValueChange={(value) => setFormData({ ...formData, connector_id: value })}
            >
              <SelectTrigger id='connector'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {connectors.map((connector) => (
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
              value={formData.is_public}
              onValueChange={handleVisibilityChange}
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
              value={formData.enable_security_headers}
              onValueChange={(value) => setFormData({ ...formData, enable_security_headers: value  })}
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
              value={formData.check_health}
              onValueChange={(val) => setFormData({ ...formData, check_health: val })}
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
              value={formData.health_endpoint}
              onChange={(e) => setFormData({ ...formData, health_endpoint: e.target.value })}
            />
          </div>

          <div className='space-y-2'>
            <Label>Intervale</Label>
            <Input
              id=''
              placeholder='/health'
              value={formData.check_interval}
              onChange={(e) => setFormData({ ...formData, check_interval: Number(e.target.value) })}
            />
          </div>

          <div className='space-y-2'>
            <Label>Mot de Passe SOCK5</Label>
            <Input
              id=''
              placeholder='Mot de passe...'
              value={formData.sock_pass}
              onChange={(e) => setFormData({ ...formData, sock_pass: e.target.value })}
            />
          </div>



          {/* Info Box */}
          <div className='bg-blue-50 border border-blue-200 rounded-lg p-4'>
            <div className='flex items-start gap-2'>
              <Info className='h-4 w-4 text-blue-600 mt-0.5' />
              <div className='text-sm text-blue-800'>
                <p className='font-medium'>La politique par défaut est TOUT REFUSER.</p>
                <p className='text-blue-700'>Vous pourrez ajouter des règles d’accès après l’enregistrement.</p>
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
              Annuler
            </Button>
            <Button type='button' className='flex-1' onClick={handleSubmit} disabled={createApplication.isPending}>
              {createApplication.isPending ? 'Enregistrement…' : 'Ajouter l’application'}
            </Button>
          </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}