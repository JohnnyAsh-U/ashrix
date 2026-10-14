import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { AdminUsersTable } from './components/admin-users-table'
import { InviteAdminForm } from './components/invite-admin-form'
import {
  Loader2,
  Plus,
  Shield,
  UserCog,
  Eye,
  Building2,
  Key,
  Users as UsersIcon
} from 'lucide-react'
import type { AdminUser } from '@/types/admin-user'
import OrganisationSettingsWithDetection from './components/settings'
import IdentitySettingsEnhanced from './components/identity'

type SettingsTab = 'organisation' | 'identity' | 'admin-users'

// Static data
const staticAdminUsers: AdminUser[] = [
  {
    id: '1',
    name: 'James Kofi',
    email: 'james@acme.com',
    role: 'Owner',
    joined: 'May 10, 2026',
    canRemove: false
  },
  {
    id: '2',
    name: 'Sarah Mensah',
    email: 'sarah@acme.com',
    role: 'Admin',
    joined: 'May 12, 2026',
    canRemove: true
  },
  {
    id: '3',
    name: 'Kwame Osei',
    email: 'kwame@acme.com',
    role: 'Auditor',
    joined: 'May 15, 2026',
    canRemove: true
  }
]

const roles = [
  {
    name: 'Owner',
    description: 'Full access. Can invite/remove admins.',
    icon: Shield,
    color: 'text-purple-600'
  },
  {
    name: 'Admin',
    description: 'Manage apps, policies, gateways. View all logs.',
    icon: UserCog,
    color: 'text-blue-600'
  },
  {
    name: 'Auditor',
    description: 'Read-only. Logs and audit trail only.',
    icon: Eye,
    color: 'text-gray-600'
  }
]

export default function Settings() {
  const [activeTab, setActiveTab] = useState<SettingsTab>('organisation')
  const [inviteAdminOpen, setInviteAdminOpen] = useState(false)
  const [adminUsers] = useState<AdminUser[]>(staticAdminUsers)
  const [isLoading] = useState(false)

  const handleOpenInviteAdmin = () => {
    setInviteAdminOpen(true)
  }

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-6'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Settings</h2>
          <p className='text-muted-foreground'>
            Admin user management
          </p>
        </div>
      </div>

      {/* Settings Tabs */}
      <div className='flex gap-4 mb-6 border-b'>
        <Button
          variant={activeTab === 'organisation' ? 'default' : 'ghost'}
          onClick={() => setActiveTab('organisation')}
          className='relative'
        >
          <Building2 className='h-4 w-4 mr-2' />
          Organisation
          {activeTab === 'organisation' && (
            <span className='absolute bottom-0 left-0 right-0 h-0.5 bg-primary'></span>
          )}
        </Button>
        <Button
          variant={activeTab === 'identity' ? 'default' : 'ghost'}
          onClick={() => setActiveTab('identity')}
          className='relative'
        >
          <Key className='h-4 w-4 mr-2' />
          Identity (IdP)
          {activeTab === 'identity' && (
            <span className='absolute bottom-0 left-0 right-0 h-0.5 bg-primary'></span>
          )}
        </Button>
        <Button
          variant={activeTab === 'admin-users' ? 'default' : 'ghost'}
          onClick={() => setActiveTab('admin-users')}
          className='relative'
        >
          <UsersIcon className='h-4 w-4 mr-2' />
          Admin Users
          {activeTab === 'admin-users' && (
            <span className='absolute bottom-0 left-0 right-0 h-0.5 bg-primary'></span>
          )}
        </Button>
      </div>
      {activeTab === 'organisation' && (
        <>
          {isLoading ? (
            <div className='flex items-center justify-center h-64'>
              <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
            </div>
          ) : (
            <OrganisationSettingsWithDetection />
          )}
        </>
      )}

      {activeTab === 'identity' && (
         <>
          {isLoading ? (
            <div className='flex items-center justify-center h-64'>
              <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
            </div>
          ) : (
            <IdentitySettingsEnhanced />
          )}
        </>
      )}


      {activeTab === 'admin-users' && (
        <>
          {/* Header with Invite Button */}
          <div className='flex items-center justify-between mb-4'>
            <p className='text-muted-foreground'>
              Manage who has access to the admin console
            </p>
            <Button onClick={handleOpenInviteAdmin}>
              <Plus className='h-4 w-4 mr-2' />
              Invite Admin
            </Button>
          </div>

          {isLoading ? (
            <div className='flex items-center justify-center h-64'>
              <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
            </div>
          ) : (
            <AdminUsersTable
              adminUsers={adminUsers}
              isLoading={isLoading}
            />
          )}

          {/* Roles Section */}
          <div className='mt-8'>
            <h3 className='text-lg font-semibold mb-4'>Roles</h3>
            <div className='space-y-3'>
              {roles.map((role) => (
                <div
                  key={role.name}
                  className='flex items-start gap-3 border rounded-lg p-4'
                >
                  <role.icon className={`h-5 w-5 ${role.color} mt-0.5`} />
                  <div>
                    <p className='text-sm font-medium'>{role.name}:</p>
                    <p className='text-sm text-muted-foreground'>
                      {role.description}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </>
      )}


      {/* Invite Admin Drawer */}
      <InviteAdminForm
        open={inviteAdminOpen}
        onOpenChange={setInviteAdminOpen}
      />
    </Main>
  )
}