/* eslint-disable react-hooks/set-state-in-effect */
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { Check, Clock, Laptop, Layers, Loader2, Network, Plus, SendToBack, Shield, ShieldAlert, Trash2, Users } from 'lucide-react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { CreatePolicyRequest, PolicyResource, PolicySubject, TimeRule } from '@/types/policy-api'
import { Badge } from '@/components/ui/badge'
import { useApplicationsQuery } from '@/hooks/application.hooks'
import { Switch } from '@/components/ui/switch'
import { toast } from 'sonner'
import { useUpdatePolicyMutation, usePolicyQuery } from '@/hooks/policy.hooks'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useEffect } from 'react'

interface CreateRuleFormProps {
    open?: boolean
    onOpenChange?: (open: boolean) => void
    onCancel?: () => void
}



const DAYS_OF_WEEK = [
    { value: 'monday', label: 'Lundi' },
    { value: 'tuesday', label: 'Mardi' },
    { value: 'wednesday', label: 'Mercredi' },
    { value: 'thursday', label: 'Jeudi' },
    { value: 'friday', label: 'Vendredi' },
    { value: 'saturday', label: 'Samedi' },
    { value: 'sunday', label: 'Dimanche' },
]


export default function UpdatePolicy({ onCancel, onOpenChange }: CreateRuleFormProps) {

    const navigate = useNavigate()
    const { id } = useParams({ strict: false }) as { id: string }
    const { data: policy } = usePolicyQuery(id)
    
    const [name, setName] = useState('')
    const [description, setDescription] = useState('')
    const [effect, setEffect] = useState<'ALLOW' | 'DENY'>('ALLOW')
    const [priority, setPriority] = useState('100')


    // Subjects (Multiple subjects of type group, app, user)
    const [subjects, setSubjects] = useState<PolicySubject[]>([])
    const [subjectTypeInput, setSubjectTypeInput] = useState<'group' | 'app' | 'user'>('group')
    const [subjectValueInput, setSubjectValueInput] = useState('')
    const [selectedAppSubject, setSelectedAppSubject] = useState('')


    // Resources (Multiple resources of type app, path, method)
    const [resources, setResources] = useState<PolicyResource[]>([])
    const [resourceTypeInput, setResourceTypeInput] = useState<'app' | 'path' | 'method'>('app')
    const [resourceValueInput, setResourceValueInput] = useState('')
    const [selectedAppResource, setSelectedAppResource] = useState('')


    // Conditions - MFA
    const [mfaEnabled, setMfaEnabled] = useState(false)
    const [mfaMinLevel, setMfaMinLevel] = useState('totp')

    // Conditions - Network
    const [networkEnabled, setNetworkEnabled] = useState(false)
    const [allowedCidrs, setAllowedCidrs] = useState<string[]>([])
    const [cidrInput, setCidrInput] = useState('')
    const [blockedCidrs, setBlockedCidrs] = useState<string[]>([])
    const [blockedCidrInput, setBlockedCidrInput] = useState('')
    const [allowedCountries, setAllowedCountries] = useState<string[]>([])
    const [countryInput, setCountryInput] = useState('')
    const [blockedCountries, setBlockedCountries] = useState<string[]>([])
    const [blockedCountryInput, setBlockedCountryInput] = useState('')
    const [blockTor, setBlockTor] = useState(true)



    // Conditions - Device
    const [deviceEnabled, setDeviceEnabled] = useState(false)
    const [postures, setPostures] = useState<string[]>([])
    const [postureInput, setPostureInput] = useState('')


    // Conditions - Time Schedule
    const [timeEnabled, setTimeEnabled] = useState(false)
    const [scheduleName, setScheduleName] = useState('business-hours')
    const [timezone, setTimezone] = useState('UTC')
    const [timeRules, setTimeRules] = useState<TimeRule[]>([
        { day: 'monday', start: '09:00', end: '18:00' },
        { day: 'tuesday', start: '09:00', end: '18:00' },
        { day: 'wednesday', start: '09:00', end: '18:00' },
        { day: 'thursday', start: '09:00', end: '18:00' },
        { day: 'friday', start: '09:00', end: '18:00' }
    ])
    const [ruleDay, setRuleDay] = useState('monday')
    const [ruleStart, setRuleStart] = useState('09:00')
    const [ruleEnd, setRuleEnd] = useState('18:00')
    const { data: applications = [] } = useApplicationsQuery()


    // Summary Modal
    const [showSummaryModal, setShowSummaryModal] = useState(false)

    const updatePolicy = useUpdatePolicyMutation()

    useEffect(() => {
        if (policy) {
            setName(policy.name || '')
            setDescription(policy.description || '')
            setEffect(policy.effect || 'ALLOW')
            setPriority(policy.priority?.toString() || '100')
            setSubjects(policy.subjects || [])
            setResources(policy.resources || [])
            
            if (policy.conditions) {
                if (policy.conditions.mfa) {
                    setMfaEnabled(true)
                    setMfaMinLevel(policy.conditions.mfa.min_level || 'totp')
                }
                if (policy.conditions.device) {
                    setDeviceEnabled(true)
                    setPostures(policy.conditions.device.postures || [])
                }
                if (policy.conditions.network) {
                    setNetworkEnabled(true)
                    setAllowedCidrs(policy.conditions.network.allowed_cidrs || [])
                    setBlockedCidrs(policy.conditions.network.blocked_cidrs || [])
                    setAllowedCountries(policy.conditions.network.allowed_countries || [])
                    setBlockedCountries(policy.conditions.network.blocked_countries || [])
                    setBlockTor(policy.conditions.network.block_tor ?? true)
                }
                if (policy.conditions.time) {
                    setTimeEnabled(true)
                    setScheduleName(policy.conditions.time.schedule_name || 'business-hours')
                    setTimezone(policy.conditions.time.timezone || 'UTC')
                    setTimeRules(policy.conditions.time.rules || [])
                }
            }
        }
    }, [policy])

    const handleClose = () => {
        if (onCancel) onCancel()
        if (onOpenChange) onOpenChange(false)
    }

    // Handlers for Subject addition
    const addSubject = () => {
        let val = ''
        if (subjectTypeInput === 'app') {
            val = selectedAppSubject
        } else {
            val = subjectValueInput.trim()
        }
        if (!val) return
        if (!subjects.some(s => s.type === subjectTypeInput && s.value === val)) {
            setSubjects([...subjects, { type: subjectTypeInput, value: val }])
        }
        setSubjectValueInput('')
        setSelectedAppSubject('')
    }
    const removeSubject = (index: number) => {
        setSubjects(subjects.filter((_, i) => i !== index))
    }


    // Handlers for Resource addition
    const addResource = () => {
        let val = ''
        if (resourceTypeInput === 'app') {
            val = selectedAppResource
        } else {
            val = resourceValueInput.trim()
        }
        if (!val) return
        if (!resources.some(r => r.type === resourceTypeInput && r.value === val)) {
            setResources([...resources, { type: resourceTypeInput, value: val }])
        }
        setResourceValueInput('')
        setSelectedAppResource('')
    }
    const removeResource = (index: number) => {
        setResources(resources.filter((_, i) => i !== index))
    }


    // Handlers for Tag-based fields
    const addTag = (
        input: string,
        list: string[],
        setList: (val: string[]) => void,
        setInput: (val: string) => void
    ) => {
        const val = input.trim()
        if (val && !list.includes(val)) {
            setList([...list, val])
        }
        setInput('')
    }
    const removeTag = (item: string, list: string[], setList: (val: string[]) => void) => {
        setList(list.filter(i => i !== item))
    }


    // Handlers for Time Rules
    const addTimeRule = () => {
        if (!ruleDay || !ruleStart || !ruleEnd) return
        setTimeRules([...timeRules, { day: ruleDay, start: ruleStart, end: ruleEnd }])
    }
    const removeTimeRule = (index: number) => {
        setTimeRules(timeRules.filter((_, i) => i !== index))
    }



    // Build Payload
    const buildPayload = (): CreatePolicyRequest => {
        const conditionsPayload: CreatePolicyRequest['conditions'] = {}

        if (mfaEnabled) {
            conditionsPayload.mfa = {
                required: true,
                min_level: mfaMinLevel || 'totp',
            }
        }

        if (deviceEnabled && postures.length > 0) {
            conditionsPayload.device = {
                postures: postures,
            }
        }

        if (networkEnabled) {
            conditionsPayload.network = {
                allowed_cidrs: allowedCidrs.length > 0 ? allowedCidrs : undefined,
                blocked_cidrs: blockedCidrs.length > 0 ? blockedCidrs : undefined,
                allowed_countries: allowedCountries.length > 0 ? allowedCountries : undefined,
                blocked_countries: blockedCountries.length > 0 ? blockedCountries : undefined,
                block_tor: blockTor,
            }
        }

        if (timeEnabled) {
            conditionsPayload.time = {
                schedule_name: scheduleName || 'business-hours',
                timezone: timezone || 'UTC',
                rules: timeRules,
            }
        }

        return {
            name,
            description,
            effect,
            priority: Number(priority) || 100,
            subjects: subjects.length > 0 ? subjects : [{ type: 'group', value: 'everyone' }],
            resources: resources.length > 0 ? resources : [{ type: 'app', value: '*' }],
            conditions: conditionsPayload,
        }
    }

    const validateAndShowSummary = () => {
        if (!name.trim()) {
            toast.error('Le nom de la politique est requis.')
            return
        }
        if (subjects.length === 0) {
            toast.error('Veuillez ajouter au moins un sujet (groupe, utilisateur ou application).')
            return
        }
        if (resources.length === 0) {
            toast.error('Veuillez ajouter au moins une ressource ciblée.')
            return
        }
        setShowSummaryModal(true)
    }

    const handleConfirmCreate = async () => {
        const payload = buildPayload()
        try {
            await updatePolicy.mutateAsync({ id, payload })
            toast.success('Politique d’accès modifiée avec succès')
            setShowSummaryModal(false)
            handleClose()
        } catch {
            toast.error('Impossible de modifier la politique d’accès')
        }
    }

    const payloadPreview = buildPayload()

    return (
        <Main>
            <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
                <div>
                    <h2 className='text-2xl font-bold tracking-tight'>Modifier la politique d’accès</h2>
                    <p className='text-muted-foreground'>Mettre à jour les règles d’accès</p>
                </div>
                <Button onClick={() => navigate({ to: "/policies" })}>
                    <SendToBack className='h-4 w-4 mr-2' />
                    Les Politiques
                </Button>
            </div>

            {/* Section 1: Informations Générales */}
            <section className='rounded-xl border p-6 mt-5 space-y-6 mb-3'>
                <div className='flex items-center gap-3 border-b pb-4'>
                    <div className='p-2 bg-blue-50 text-blue-600 rounded-lg dark:bg-blue-950 dark:text-blue-400'>
                        <Shield className='h-5 w-5' />
                    </div>
                    <div>
                        <h2 className='text-lg font-semibold'>1. Informations Générales</h2>
                        <p className='text-xs text-muted-foreground'>Identifiez la politique, son action par défaut et sa priorité d'évaluation.</p>
                    </div>
                </div>

                <div className='grid grid-cols-1 md:grid-cols-2 gap-6'>
                    <div className='space-y-2'>
                        <Label htmlFor='policy-name' className='font-medium'>Nom de la politique <span className='text-red-500'>*</span></Label>
                        <Input
                            id='policy-name'
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder='ex: Accès Production Équipe DevOps'
                        />
                        <p className='text-xs text-muted-foreground'>Intitulé clair décrivant le périmètre ou l'objectif de la règle.</p>
                    </div>

                    <div className='space-y-2'>
                        <Label htmlFor='priority' className='font-medium'>Priorité d’évaluation</Label>
                        <Input
                            id='priority'
                            type='number'
                            value={priority}
                            onChange={(e) => setPriority(e.target.value)}
                            placeholder='100'
                        />
                        <p className='text-xs text-muted-foreground'>Priorité numérique (ex: 100). Les règles à priorité élevée sont évaluées en premier.</p>
                    </div>
                </div>

                <div className='space-y-2'>
                    <Label htmlFor='description' className='font-medium'>Description</Label>
                    <Textarea
                        id='description'
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                        placeholder='Description détaillée des conditions d’accès et des personnes concernées...'
                        rows={2}
                    />
                </div>

                <div className='space-y-3 pt-2'>
                    <Label className='font-medium'>Effet de la règle (Action)</Label>
                    <RadioGroup
                        value={effect}
                        onValueChange={(val) => setEffect(val as 'ALLOW' | 'DENY')}
                        className='grid grid-cols-1 sm:grid-cols-2 gap-4'
                    >
                        <div className={`flex items-start space-x-3 border p-4 rounded-lg cursor-pointer transition-colors ${effect === 'ALLOW' ? 'border-green-600 bg-green-50/50 dark:bg-green-950/20' : 'hover:bg-muted/50'}`}>
                            <RadioGroupItem value='ALLOW' id='effect-allow' className='mt-1' />
                            <label htmlFor='effect-allow' className='cursor-pointer space-y-1'>
                                <span className='font-semibold text-green-700 dark:text-green-400 block'>AUTORISER (ALLOW)</span>
                                <span className='text-xs text-muted-foreground block'>Accorde l'accès aux ressources cibles si toutes les conditions requises sont validées.</span>
                            </label>
                        </div>

                        <div className={`flex items-start space-x-3 border p-4 rounded-lg cursor-pointer transition-colors ${effect === 'DENY' ? 'border-red-600 bg-red-50/50 dark:bg-red-950/20' : 'hover:bg-muted/50'}`}>
                            <RadioGroupItem value='DENY' id='effect-deny' className='mt-1' />
                            <label htmlFor='effect-deny' className='cursor-pointer space-y-1'>
                                <span className='font-semibold text-red-700 dark:text-red-400 block'>REFUSER (DENY)</span>
                                <span className='text-xs text-muted-foreground block'>Bloque catégoriquement l'accès dès que les critères cibles correspondent.</span>
                            </label>
                        </div>
                    </RadioGroup>
                </div>
            </section>



            {/* Section 2: Sujets (Subjects) */}
            <section className='bg-card border rounded-xl p-6 space-y-6 mb-3'>
                <div className='flex items-center gap-3 border-b pb-4'>
                    <div className='p-2 bg-indigo-50 text-indigo-600 rounded-lg dark:bg-indigo-950 dark:text-indigo-400'>
                        <Users className='h-5 w-5' />
                    </div>
                    <div>
                        <h2 className='text-lg font-semibold'>2. Sujets d'Identité (Subjects)</h2>
                        <p className='text-xs text-muted-foreground'>Définissez qui ou quel groupe/application initie la requête d'accès (groupes IdP, utilisateurs spécifiques ou identités d'applications).</p>
                    </div>
                </div>

                <div className='space-y-4'>
                    <div className='flex flex-wrap gap-2 items-end bg-muted/30 p-4 rounded-lg border'>
                        <div className='space-y-1 w-40'>
                            <Label className='text-xs'>Type de sujet</Label>
                            <Select value={subjectTypeInput} onValueChange={(val: 'group' | 'app' | 'user') => setSubjectTypeInput(val)}>
                                <SelectTrigger className='bg-background'>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value='group'>Groupe (IdP)</SelectItem>
                                    <SelectItem value='app'>Application</SelectItem>
                                    <SelectItem value='user'>Utilisateur</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        {subjectTypeInput === 'app' ? (
                            <div className='space-y-1 flex-1 min-w-[200px]'>
                                <Label className='text-xs'>Sélectionner l'application</Label>
                                <Select value={selectedAppSubject} onValueChange={setSelectedAppSubject}>
                                    <SelectTrigger className='bg-background'>
                                        <SelectValue placeholder='Choisir une application...' />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {applications.map((app) => (
                                            <SelectItem key={app.id} value={app.id}>
                                                {app.name} ({app.subdomain})
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        ) : (
                            <div className='space-y-1 flex-1 min-w-[200px]'>
                                <Label className='text-xs'>
                                    {subjectTypeInput === 'group' ? 'Nom du groupe IdP' : 'Identifiant / E-mail utilisateur'}
                                </Label>
                                <Input
                                    className='bg-background'
                                    value={subjectValueInput}
                                    onChange={(e) => setSubjectValueInput(e.target.value)}
                                    placeholder={subjectTypeInput === 'group' ? 'ex: engineering, admins' : 'ex: alice@company.com'}
                                    onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addSubject(); } }}
                                />
                            </div>
                        )}

                        <Button type='button' variant='secondary' onClick={addSubject} className='gap-1'>
                            <Plus className='h-4 w-4' /> Ajouter
                        </Button>
                    </div>

                    {/* List of Added Subjects */}
                    <div className='space-y-2'>
                        <Label className='text-xs text-muted-foreground'>Sujets configurés ({subjects.length}) :</Label>
                        {subjects.length === 0 ? (
                            <p className='text-sm text-muted-foreground italic bg-muted/20 p-3 rounded-md text-center'>
                                Aucun sujet configuré. Ajoutez au moins un groupe, une application ou un utilisateur.
                            </p>
                        ) : (
                            <div className='flex flex-wrap gap-2'>
                                {subjects.map((sub, idx) => (
                                    <Badge key={idx} variant='outline' className='flex items-center gap-2 py-1 px-3 bg-background text-sm font-normal'>
                                        <span className='font-semibold text-xs text-indigo-600 uppercase bg-indigo-50 px-1.5 py-0.5 rounded dark:bg-indigo-950 dark:text-indigo-400'>
                                            {sub.type}
                                        </span>
                                        <span>{sub.type == 'app' ? applications.find((app) => app.id == sub.value)?.name : sub.value}</span>
                                        <button type='button' onClick={() => removeSubject(idx)} className='text-muted-foreground hover:text-red-500 ml-1'>
                                            <Trash2 className='h-3.5 w-3.5' />
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </section>


            {/* Section 3: Ressources (Resources) */}
            <section className='bg-card border rounded-xl p-6 space-y-6 mb-3'>
                <div className='flex items-center gap-3 border-b pb-4'>
                    <div className='p-2 bg-purple-50 text-purple-600 rounded-lg dark:bg-purple-950 dark:text-purple-400'>
                        <Layers className='h-5 w-5' />
                    </div>
                    <div>
                        <h2 className='text-lg font-semibold'>3. Ressources Cibles (Resources)</h2>
                        <p className='text-xs text-muted-foreground'>Sélectionnez les applications cibles ou spécifiez des restrictions par sous-chemins URL (path) et méthodes HTTP.</p>
                    </div>
                </div>

                <div className='space-y-4'>
                    <div className='flex flex-wrap gap-2 items-end bg-muted/30 p-4 rounded-lg border'>
                        <div className='space-y-1 w-40'>
                            <Label className='text-xs'>Type de ressource</Label>
                            <Select value={resourceTypeInput} onValueChange={(val: 'app' | 'path' | 'method') => setResourceTypeInput(val)}>
                                <SelectTrigger className='bg-background'>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value='app'>Application (app)</SelectItem>
                                    <SelectItem value='path'>Chein URL (path)</SelectItem>
                                    <SelectItem value='method'>Méthode HTTP</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        {resourceTypeInput === 'app' ? (
                            <div className='space-y-1 flex-1 min-w-[200px]'>
                                <Label className='text-xs'>Sélectionner l'application</Label>
                                <Select value={selectedAppResource} onValueChange={setSelectedAppResource}>
                                    <SelectTrigger className='bg-background'>
                                        <SelectValue placeholder='Choisir une application...' />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {applications.map((app) => (
                                            <SelectItem key={app.id} value={app.id}>
                                                {app.name} ({app.subdomain})
                                            </SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                        ) : (
                            <div className='space-y-1 flex-1 min-w-[200px]'>
                                <Label className='text-xs'>
                                    {resourceTypeInput === 'path' ? 'Valeur du chemin (ex: /api/v1/*)' : 'Méthode HTTP (ex: GET, POST, DELETE)'}
                                </Label>
                                <Input
                                    className='bg-background'
                                    value={resourceValueInput}
                                    onChange={(e) => setResourceValueInput(e.target.value)}
                                    placeholder={resourceTypeInput === 'path' ? '/admin/*' : 'GET'}
                                    onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); addResource(); } }}
                                />
                            </div>
                        )}

                        <Button type='button' variant='secondary' onClick={addResource} className='gap-1'>
                            <Plus className='h-4 w-4' /> Ajouter
                        </Button>
                    </div>

                    {/* List of Added Resources */}
                    <div className='space-y-2'>
                        <Label className='text-xs text-muted-foreground'>Ressources configurées ({resources.length}) :</Label>
                        {resources.length === 0 ? (
                            <p className='text-sm text-muted-foreground italic bg-muted/20 p-3 rounded-md text-center'>
                                Aucune ressource ciblée. Ajoutez des applications, des chemins ou des méthodes HTTP.
                            </p>
                        ) : (
                            <div className='flex flex-wrap gap-2'>
                                {resources.map((res, idx) => (
                                    <Badge key={idx} variant='outline' className='flex items-center gap-2 py-1 px-3 bg-background text-sm font-normal'>
                                        <span className='font-semibold text-xs text-purple-600 uppercase bg-purple-50 px-1.5 py-0.5 rounded dark:bg-purple-950 dark:text-purple-400'>
                                            {res.type}
                                        </span>
                                        <span>{res.type == 'app' ? applications.find((app) => app.id == res.value)?.name : res.value}</span>
                                        <button type='button' onClick={() => removeResource(idx)} className='text-muted-foreground hover:text-red-500 ml-1'>
                                            <Trash2 className='h-3.5 w-3.5' />
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </section>



            {/* Section 4: Conditions Contextuelles */}
            <div className='rounded-xl border p-6 space-y-6'>
                <div className='flex items-center gap-3 border-b pb-4'>
                    <div className='p-2 bg-blue-50 text-blue-600 rounded-lg dark:bg-blue-950 dark:text-blue-400'>
                        <Shield className='h-5 w-5' />
                    </div>
                    <div>
                        <h2 className='text-lg font-semibold'>4. Conditions d'Accès Contextuelles</h2>
                        <p className='text-xs text-muted-foreground'>Définissez les exigences d'authentification forte, de posture d'appareil, de sécurité réseau et d'horaires.</p>
                    </div>
                </div>

                {/* Condition: MFA */}
                <div className='border rounded-xl p-6 space-y-4'>
                    <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-3'>
                            <div className='p-2 bg-emerald-50 text-emerald-600 rounded-lg dark:bg-emerald-950 dark:text-emerald-400'>
                                <ShieldAlert className='h-5 w-5' />
                            </div>
                            <div>
                                <h3 className='font-semibold text-base'>Authentification Forte (MFA)</h3>
                                <p className='text-xs text-muted-foreground'>Exige un deuxième facteur d'authentification pour valider l'accès.</p>
                            </div>
                        </div>
                        <Switch checked={mfaEnabled} onCheckedChange={setMfaEnabled} />
                    </div>

                    {mfaEnabled && (
                        <div className='pt-3 border-t grid grid-cols-1 md:grid-cols-2 gap-4'>
                            <div className='space-y-2'>
                                <Label className='text-xs font-medium'>Niveau MFA minimum requis</Label>
                                <Select value={mfaMinLevel} onValueChange={setMfaMinLevel}>
                                    <SelectTrigger className='bg-background'>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value='totp'>TOTP (Application Authenticator)</SelectItem>
                                        <SelectItem value='webauthn'>WebAuthn / FIDO2 (Clé de sécurité hardware)</SelectItem>
                                        <SelectItem value='push'>Notification Push mobile</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                    )}
                </div>

                {/* Condition: Appareil (Device Posture) */}
                <div className='border rounded-xl p-6 space-y-4'>
                    <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-3'>
                            <div className='p-2 bg-orange-50 text-orange-600 rounded-lg dark:bg-orange-950 dark:text-orange-400'>
                                <Laptop className='h-5 w-5' />
                            </div>
                            <div>
                                <h3 className='font-semibold text-base'>Posture de l’Appareil (Device Posture)</h3>
                                <p className='text-xs text-muted-foreground'>Vérifiez l'état de conformité de la machine cliente (conforme, chiffrement de disque, antivirus...).</p>
                            </div>
                        </div>
                        <Switch checked={deviceEnabled} onCheckedChange={setDeviceEnabled} />
                    </div>

                    {deviceEnabled && (
                        <div className='pt-3 border-t space-y-4'>
                            <div className='flex items-end gap-2'>
                                <div className='space-y-1 flex-1'>
                                    <Label className='text-xs'>Ajouter une posture requise</Label>
                                    <Input
                                        className='bg-background'
                                        value={postureInput}
                                        onChange={(e) => setPostureInput(e.target.value)}
                                        placeholder='ex: compliant, disk-encrypted, antivirus-active'
                                        onKeyDown={(e) => {
                                            if (e.key === 'Enter') {
                                                e.preventDefault()
                                                addTag(postureInput, postures, setPostures, setPostureInput)
                                            }
                                        }}
                                    />
                                </div>
                                <Button type='button' variant='secondary' onClick={() => addTag(postureInput, postures, setPostures, setPostureInput)}>
                                    <Plus className='h-4 w-4 mr-1' /> Ajouter
                                </Button>
                            </div>

                            <div className='flex flex-wrap gap-2'>
                                {postures.map((p, i) => (
                                    <Badge key={i} variant='secondary' className='flex items-center gap-2 py-1 px-3'>
                                        <span>{p}</span>
                                        <button type='button' onClick={() => removeTag(p, postures, setPostures)} className='hover:text-red-500'>
                                            <Trash2 className='h-3 w-3' />
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        </div>
                    )}
                </div>

                {/* Condition: Réseau (Network) */}
                <div className='bg-card border rounded-xl p-6 space-y-4'>
                    <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-3'>
                            <div className='p-2 bg-teal-50 text-teal-600 rounded-lg dark:bg-teal-950 dark:text-teal-400'>
                                <Network className='h-5 w-5' />
                            </div>
                            <div>
                                <h3 className='font-semibold text-base'>Restrictions Réseau & Géolocalisation</h3>
                                <p className='text-xs text-muted-foreground'>Filtrez l'accès par sous-réseaux IP (CIDR), pays d'origine et blocage des nœuds anonymes Tor.</p>
                            </div>
                        </div>
                        <Switch checked={networkEnabled} onCheckedChange={setNetworkEnabled} />
                    </div>

                    {networkEnabled && (
                        <div className='pt-4 border-t space-y-6'>
                            {/* Block Tor */}
                            <div className='flex items-center justify-between bg-muted/30 p-3 rounded-lg border'>
                                <div>
                                    <Label className='font-medium text-sm'>Bloquer le réseau d'anonymisation Tor</Label>
                                    <p className='text-xs text-muted-foreground'>Rejette automatiquement les connexions en provenance de relais ou nœuds de sortie Tor.</p>
                                </div>
                                <Switch checked={blockTor} onCheckedChange={setBlockTor} />
                            </div>

                            {/* CIDR Allowed & Blocked */}
                            <div className='grid grid-cols-1 md:grid-cols-2 gap-6'>
                                <div className='space-y-3'>
                                    <Label className='text-xs font-medium'>Plages IP autorisées (CIDR Allowed)</Label>
                                    <div className='flex items-end gap-2'>
                                        <Input
                                            className='bg-background text-sm'
                                            value={cidrInput}
                                            onChange={(e) => setCidrInput(e.target.value)}
                                            placeholder='ex: 102.68.0.0/16, 192.168.1.0/24'
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter') {
                                                    e.preventDefault()
                                                    addTag(cidrInput, allowedCidrs, setAllowedCidrs, setCidrInput)
                                                }
                                            }}
                                        />
                                        <Button type='button' size='sm' variant='outline' onClick={() => addTag(cidrInput, allowedCidrs, setAllowedCidrs, setCidrInput)}>
                                            +
                                        </Button>
                                    </div>
                                    <div className='flex flex-wrap gap-1.5'>
                                        {allowedCidrs.map((cidr, i) => (
                                            <Badge key={i} variant='outline' className='bg-green-50 text-green-700 border-green-200 dark:bg-green-950 dark:text-green-300 gap-1 text-xs'>
                                                {cidr}
                                                <button type='button' onClick={() => removeTag(cidr, allowedCidrs, setAllowedCidrs)} className='hover:text-red-500 ml-1'>×</button>
                                            </Badge>
                                        ))}
                                    </div>
                                </div>

                                <div className='space-y-3'>
                                    <Label className='text-xs font-medium'>Plages IP bloquées (CIDR Blocked)</Label>
                                    <div className='flex items-end gap-2'>
                                        <Input
                                            className='bg-background text-sm'
                                            value={blockedCidrInput}
                                            onChange={(e) => setBlockedCidrInput(e.target.value)}
                                            placeholder='ex: 185.220.101.0/24'
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter') {
                                                    e.preventDefault()
                                                    addTag(blockedCidrInput, blockedCidrs, setBlockedCidrs, setBlockedCidrInput)
                                                }
                                            }}
                                        />
                                        <Button type='button' size='sm' variant='outline' onClick={() => addTag(blockedCidrInput, blockedCidrs, setBlockedCidrs, setBlockedCidrInput)}>
                                            +
                                        </Button>
                                    </div>
                                    <div className='flex flex-wrap gap-1.5'>
                                        {blockedCidrs.map((cidr, i) => (
                                            <Badge key={i} variant='outline' className='bg-red-50 text-red-700 border-red-200 dark:bg-red-950 dark:text-red-300 gap-1 text-xs'>
                                                {cidr}
                                                <button type='button' onClick={() => removeTag(cidr, blockedCidrs, setBlockedCidrs)} className='hover:text-red-500 ml-1'>×</button>
                                            </Badge>
                                        ))}
                                    </div>
                                </div>
                            </div>

                            {/* Countries Allowed & Blocked */}
                            <div className='grid grid-cols-1 md:grid-cols-2 gap-6'>
                                <div className='space-y-3'>
                                    <Label className='text-xs font-medium'>Pays autorisés (Codes ISO Alpha-2)</Label>
                                    <div className='flex items-end gap-2'>
                                        <Input
                                            className='bg-background text-sm'
                                            value={countryInput}
                                            onChange={(e) => setCountryInput(e.target.value)}
                                            placeholder='ex: CI, GH, FR, US'
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter') {
                                                    e.preventDefault()
                                                    addTag(countryInput, allowedCountries, setAllowedCountries, setCountryInput)
                                                }
                                            }}
                                        />
                                        <Button type='button' size='sm' variant='outline' onClick={() => addTag(countryInput, allowedCountries, setAllowedCountries, setCountryInput)}>
                                            +
                                        </Button>
                                    </div>
                                    <div className='flex flex-wrap gap-1.5'>
                                        {allowedCountries.map((c, i) => (
                                            <Badge key={i} variant='outline' className='bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-300 gap-1 text-xs uppercase'>
                                                {c}
                                                <button type='button' onClick={() => removeTag(c, allowedCountries, setAllowedCountries)} className='hover:text-red-500 ml-1'>×</button>
                                            </Badge>
                                        ))}
                                    </div>
                                </div>

                                <div className='space-y-3'>
                                    <Label className='text-xs font-medium'>Pays bloqués (Codes ISO Alpha-2)</Label>
                                    <div className='flex items-end gap-2'>
                                        <Input
                                            className='bg-background text-sm'
                                            value={blockedCountryInput}
                                            onChange={(e) => setBlockedCountryInput(e.target.value)}
                                            placeholder='ex: RU, CN'
                                            onKeyDown={(e) => {
                                                if (e.key === 'Enter') {
                                                    e.preventDefault()
                                                    addTag(blockedCountryInput, blockedCountries, setBlockedCountries, setBlockedCountryInput)
                                                }
                                            }}
                                        />
                                        <Button type='button' size='sm' variant='outline' onClick={() => addTag(blockedCountryInput, blockedCountries, setBlockedCountries, setBlockedCountryInput)}>
                                            +
                                        </Button>
                                    </div>
                                    <div className='flex flex-wrap gap-1.5'>
                                        {blockedCountries.map((c, i) => (
                                            <Badge key={i} variant='outline' className='bg-red-50 text-red-700 border-red-200 dark:bg-red-950 dark:text-red-300 gap-1 text-xs uppercase'>
                                                {c}
                                                <button type='button' onClick={() => removeTag(c, blockedCountries, setBlockedCountries)} className='hover:text-red-500 ml-1'>×</button>
                                            </Badge>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}
                </div>

                {/* Condition: Horaire (Time Schedule) */}
                <div className='bg-card border rounded-xl p-6 space-y-4'>
                    <div className='flex items-center justify-between'>
                        <div className='flex items-center gap-3'>
                            <div className='p-2 bg-amber-50 text-amber-600 rounded-lg dark:bg-amber-950 dark:text-amber-400'>
                                <Clock className='h-5 w-5' />
                            </div>
                            <div>
                                <h3 className='font-semibold text-base'>Horaire & Planning d'Accès</h3>
                                <p className='text-xs text-muted-foreground'>Limitez l'accès à des fenêtres temporelles quotidiennes (ex: heures de bureau).</p>
                            </div>
                        </div>
                        <Switch checked={timeEnabled} onCheckedChange={setTimeEnabled} />
                    </div>

                    {timeEnabled && (
                        <div className='pt-4 border-t space-y-6'>
                            <div className='grid grid-cols-1 md:grid-cols-2 gap-4'>
                                <div className='space-y-1.5'>
                                    <Label className='text-xs font-medium'>Nom de l’horaire (Schedule Name)</Label>
                                    <Input
                                        className='bg-background'
                                        value={scheduleName}
                                        onChange={(e) => setScheduleName(e.target.value)}
                                        placeholder='heures-de-bureau'
                                    />
                                </div>

                                <div className='space-y-1.5'>
                                    <Label className='text-xs font-medium'>Fuseau Horaire (Timezone)</Label>
                                    <Input
                                        className='bg-background'
                                        value={timezone}
                                        onChange={(e) => setTimezone(e.target.value)}
                                        placeholder='UTC, Africa/Abidjan, Europe/Paris'
                                    />
                                </div>
                            </div>

                            {/* Add Rule Form */}
                            <div className='bg-muted/30 p-4 rounded-lg border space-y-3'>
                                <Label className='text-xs font-semibold block'>Ajouter une règle journalière :</Label>
                                <div className='flex flex-wrap items-center gap-3'>
                                    <div className='w-36 space-y-1'>
                                        <Label className='text-[10px] text-muted-foreground'>Jour de la semaine</Label>
                                        <Select value={ruleDay} onValueChange={setRuleDay}>
                                            <SelectTrigger className='bg-background h-9 text-xs'>
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {DAYS_OF_WEEK.map((d) => (
                                                    <SelectItem key={d.value} value={d.value}>{d.label}</SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </div>

                                    <div className='w-28 space-y-1'>
                                        <Label className='text-[10px] text-muted-foreground'>Heure de début</Label>
                                        <Input
                                            type='time'
                                            className='bg-background h-9 text-xs'
                                            value={ruleStart}
                                            onChange={(e) => setRuleStart(e.target.value)}
                                        />
                                    </div>

                                    <div className='w-28 space-y-1'>
                                        <Label className='text-[10px] text-muted-foreground'>Heure de fin</Label>
                                        <Input
                                            type='time'
                                            className='bg-background h-9 text-xs'
                                            value={ruleEnd}
                                            onChange={(e) => setRuleEnd(e.target.value)}
                                        />
                                    </div>

                                    <Button type='button' variant='secondary' size='sm' onClick={addTimeRule} className='mt-5 gap-1'>
                                        <Plus className='h-4 w-4' /> Ajouter
                                    </Button>
                                </div>
                            </div>

                            {/* Table of Daily Rules */}
                            <div className='space-y-2'>
                                <Label className='text-xs text-muted-foreground'>Règles d'horaires configurées ({timeRules.length}) :</Label>
                                {timeRules.length === 0 ? (
                                    <p className='text-xs text-muted-foreground italic text-center p-3 bg-muted/20 rounded-md'>
                                        Aucune règle journalière. L'accès sera refusé hors des plages configurées.
                                    </p>
                                ) : (
                                    <div className='border rounded-lg overflow-hidden bg-background'>
                                        <table className='w-full text-xs text-left'>
                                            <thead className='bg-muted/50 border-b font-medium text-muted-foreground'>
                                                <tr>
                                                    <th className='p-2.5'>Jour</th>
                                                    <th className='p-2.5'>Début</th>
                                                    <th className='p-2.5'>Fin</th>
                                                    <th className='p-2.5 text-right'>Action</th>
                                                </tr>
                                            </thead>
                                            <tbody className='divide-y'>
                                                {timeRules.map((r, idx) => (
                                                    <tr key={idx} className='hover:bg-muted/20'>
                                                        <td className='p-2.5 font-medium capitalize'>
                                                            {DAYS_OF_WEEK.find(d => d.value === r.day)?.label || r.day}
                                                        </td>
                                                        <td className='p-2.5'>{r.start}</td>
                                                        <td className='p-2.5'>{r.end}</td>
                                                        <td className='p-2.5 text-right'>
                                                            <button type='button' onClick={() => removeTimeRule(idx)} className='text-red-500 hover:text-red-700 p-1'>
                                                                <Trash2 className='h-3.5 w-3.5' />
                                                            </button>
                                                        </td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                )}
                            </div>
                        </div>
                    )}
                </div>
            </div>


            {/* Footer Actions */}
            <div className='flex items-center justify-end gap-4 pt-6 border-t'>
                <Button variant='outline' onClick={handleClose}>
                    Annuler
                </Button>
                <Button onClick={validateAndShowSummary} size='lg' className='gap-2 px-8'>
                    <Check className='h-5 w-5' />
                    Vérifier & Enregistrer la Politique
                </Button>
            </div>

            {/* Confirmation Summary Modal */}
            <Dialog open={showSummaryModal} onOpenChange={setShowSummaryModal}>
                <DialogContent className='max-w-2xl max-h-[85vh] flex flex-col'>
                    <DialogHeader>
                        <DialogTitle className='flex items-center gap-2 text-lg'>
                            <Shield className='h-5 w-5 text-blue-600' />
                            Résumé de la Politique d’Accès
                        </DialogTitle>
                        <DialogDescription>
                            Vérifiez la structure JSON et la configuration avant la transmission à l'API Control Plane.
                        </DialogDescription>
                    </DialogHeader>

                    <div className='flex-1 overflow-y-auto space-y-4 py-2 pr-1'>
                        <div className='bg-muted/40 p-4 rounded-lg space-y-3 border text-sm'>
                            <div className='flex justify-between border-b pb-2'>
                                <span className='text-muted-foreground'>Nom :</span>
                                <span className='font-semibold'>{name}</span>
                            </div>
                            <div className='flex justify-between border-b pb-2'>
                                <span className='text-muted-foreground'>Effet :</span>
                                <Badge className={effect === 'ALLOW' ? 'bg-green-600' : 'bg-red-600'}>{effect}</Badge>
                            </div>
                            <div className='flex justify-between border-b pb-2'>
                                <span className='text-muted-foreground'>Priorité :</span>
                                <span className='font-semibold'>{priority}</span>
                            </div>
                            <div className='flex justify-between border-b pb-2'>
                                <span className='text-muted-foreground'>Sujets ({subjects.length}) :</span>
                                <span className='font-mono text-xs'>{subjects.map(s => `${s.type}:${s.value}`).join(', ')}</span>
                            </div>
                            <div className='flex justify-between pb-1'>
                                <span className='text-muted-foreground'>Ressources ({resources.length}) :</span>
                                <span className='font-mono text-xs'>{resources.map(r => `${r.type}:${r.value}`).join(', ')}</span>
                            </div>
                        </div>

                        <div className='space-y-1.5'>
                            <Label className='text-xs font-semibold text-muted-foreground'>Payload JSON final envoyé à l'API :</Label>
                            <pre className='bg-slate-950 text-slate-50 p-4 rounded-lg text-xs font-mono overflow-x-auto max-h-60 border'>
                                {JSON.stringify(payloadPreview, null, 2)}
                            </pre>
                        </div>
                    </div>

                    <DialogFooter className='gap-2 pt-2 border-t'>
                        <Button variant='outline' onClick={() => setShowSummaryModal(false)}>
                            Modifier
                        </Button>
                        <Button onClick={handleConfirmCreate} disabled={updatePolicy.isPending} className='gap-2'>
                            {updatePolicy.isPending && <Loader2 className='h-4 w-4 animate-spin' />}
                            Confirmer et Transmettre
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </Main>
    )
}
