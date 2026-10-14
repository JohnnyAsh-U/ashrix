import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { UsersTable } from './components/users-table'
import { GroupsTable } from './components/groups-table'
import { UserDetail } from './components/user-detail'
import { 
  Loader2, 
  Search, 
  RefreshCw, 
  Users as UsersIcon,
  UserCog,
  Shield
} from 'lucide-react'
import { Input } from '@/components/ui/input'
import type { User, Group } from '@/types/user'

// Static data
const staticUsers: User[] = [
  {
    id: '1',
    name: 'Alice Chen',
    email: 'alice@acme.com',
    groups: ['engineering', 'hr'],
    lastSeen: '2 min ago',
    status: 'online',
    permittedApps: [
      { name: 'CRM', access: 'allowed', via: 'Engineering → CRM rule' },
      { name: 'Grafana', access: 'allowed', via: 'Engineering → Grafana rule' },
      { name: 'GitLab', access: 'allowed', via: 'Engineering → GitLab rule' },
      { name: 'HR Portal', access: 'allowed', via: 'HR → HR Portal rule' },
      { name: 'Wiki', access: 'allowed', via: 'All Staff → Wiki rule' },
      { name: 'ERP', access: 'denied', via: 'No matching policy rule' },
      { name: 'Jenkins', access: 'denied', via: 'No matching policy rule' }
    ],
    activeSessions: [
      {
        id: 's1',
        device: 'Chrome · macOS',
        ip: '196.10.45.23',
        location: 'London',
        gateway: 'London HQ',
        started: 'today 09:03',
        ttl: '8h TTL'
      }
    ],
    recentAccess: [
      { date: 'Today 09:14', app: 'CRM', result: 'allowed', device: 'Chrome · macOS' },
      { date: 'Today 09:13', app: 'Grafana', result: 'allowed', device: 'Chrome · macOS' },
      { date: 'Today 09:03', app: 'HR Portal', result: 'allowed', device: 'Chrome · macOS' },
      { date: 'May 19 14:21', app: 'ERP', result: 'denied', device: 'no matching policy' },
      { date: 'May 19 14:21', app: 'ERP', result: 'denied', device: 'no matching policy' },
      { date: 'May 19 09:11', app: 'CRM', result: 'allowed', device: 'Chrome · macOS' }
    ]
  },
  {
    id: '2',
    name: 'Bob Kumar',
    email: 'bob@acme.com',
    groups: ['finance'],
    lastSeen: '1 hr ago',
    status: 'offline',
    permittedApps: [
      { name: 'Wiki', access: 'allowed', via: 'All Staff → Wiki rule' },
      { name: 'ERP', access: 'allowed', via: 'Finance → ERP rule' },
      { name: 'CRM', access: 'denied', via: 'No matching policy rule' }
    ],
    activeSessions: [],
    recentAccess: [
      { date: 'Today 08:45', app: 'ERP', result: 'allowed', device: 'Firefox · Windows' },
      { date: 'Today 08:30', app: 'Wiki', result: 'allowed', device: 'Firefox · Windows' }
    ]
  },
  {
    id: '3',
    name: 'Carol Smith',
    email: 'carol@acme.com',
    groups: ['engineering'],
    lastSeen: '3 hr ago',
    status: 'offline',
    permittedApps: [
      { name: 'CRM', access: 'allowed', via: 'Engineering → CRM rule' },
      { name: 'Grafana', access: 'allowed', via: 'Engineering → Grafana rule' },
      { name: 'GitLab', access: 'allowed', via: 'Engineering → GitLab rule' },
      { name: 'Wiki', access: 'allowed', via: 'All Staff → Wiki rule' }
    ],
    activeSessions: [],
    recentAccess: [
      { date: 'Today 06:12', app: 'GitLab', result: 'allowed', device: 'Safari · macOS' },
      { date: 'Today 06:05', app: 'Grafana', result: 'allowed', device: 'Safari · macOS' }
    ]
  },
  {
    id: '4',
    name: 'David Jones',
    email: 'david@acme.com',
    groups: ['sre', 'it'],
    lastSeen: 'Online now',
    status: 'online',
    permittedApps: [
      { name: 'Jump Server', access: 'allowed', via: 'SRE → Jump Server rule' },
      { name: 'Grafana', access: 'allowed', via: 'SRE → Grafana rule' },
      { name: 'Wiki', access: 'allowed', via: 'All Staff → Wiki rule' },
      { name: 'CRM', access: 'denied', via: 'No matching policy rule' }
    ],
    activeSessions: [
      {
        id: 's2',
        device: 'Firefox · Linux',
        ip: '196.10.45.67',
        location: 'London',
        gateway: 'London HQ',
        started: 'today 08:15',
        ttl: '12h TTL'
      }
    ],
    recentAccess: [
      { date: 'Today 09:20', app: 'Jump Server', result: 'allowed', device: 'Firefox · Linux' },
      { date: 'Today 09:10', app: 'Grafana', result: 'allowed', device: 'Firefox · Linux' }
    ]
  },
  {
    id: '5',
    name: 'Emma Wilson',
    email: 'emma@acme.com',
    groups: ['hr'],
    lastSeen: 'Today 08:44',
    status: 'offline',
    permittedApps: [
      { name: 'HR Portal', access: 'allowed', via: 'HR → HR Portal rule' },
      { name: 'CRM', access: 'allowed', via: 'HR → CRM rule' },
      { name: 'Wiki', access: 'allowed', via: 'All Staff → Wiki rule' }
    ],
    activeSessions: [],
    recentAccess: [
      { date: 'Today 08:44', app: 'HR Portal', result: 'allowed', device: 'Chrome · Windows' },
      { date: 'Today 08:30', app: 'CRM', result: 'allowed', device: 'Chrome · Windows' }
    ]
  }
]

const staticGroups: Group[] = [
  { id: 'g1', name: 'engineering', members: 34, description: 'Engineering team' },
  { id: 'g2', name: 'hr', members: 12, description: 'Human Resources' },
  { id: 'g3', name: 'finance', members: 8, description: 'Finance department' },
  { id: 'g4', name: 'sre', members: 2, description: 'Site Reliability Engineering' },
  { id: 'g5', name: 'it', members: 6, description: 'IT Support' },
  { id: 'g6', name: 'contractors', members: 8, description: 'External contractors' },
  { id: 'g7', name: 'devops', members: 8, description: 'DevOps team' },
  { id: 'g8', name: 'management', members: 5, description: 'Management team' },
  { id: 'g9', name: 'sales', members: 15, description: 'Sales team' },
  { id: 'g10', name: 'marketing', members: 10, description: 'Marketing team' },
  { id: 'g11', name: 'support', members: 20, description: 'Customer support' },
  { id: 'g12', name: 'legal', members: 3, description: 'Legal department' }
]

type TabType = 'users' | 'groups'

export default function Users() {
  const [activeTab, setActiveTab] = useState<TabType>('users')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [users] = useState<User[]>(staticUsers)
  const [groups] = useState<Group[]>(staticGroups)
  const [isLoading] = useState(false)
  const [lastSynced] = useState('3 minutes ago')

  const handleSyncNow = () => {
    console.log('Syncing with IdP...')
    // Simulate sync
  }

  const handleUserClick = (user: User) => {
    setSelectedUser(user)
  }

  const handleBack = () => {
    setSelectedUser(null)
  }

  const filteredUsers = users.filter(user => {
    return (
      user.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      user.email.toLowerCase().includes(searchQuery.toLowerCase())
    )
  })

  if (selectedUser) {
    return (
      <Main>
        <UserDetail user={selectedUser} onBack={handleBack} />
      </Main>
    )
  }

  return (
    <Main>
      <div className='flex flex-wrap items-end justify-between gap-2 mb-4'>
        <div>
          <h2 className='text-2xl font-bold tracking-tight'>Users</h2>
          <div className='flex items-center gap-2 mt-1'>
            <Shield className='h-4 w-4 text-muted-foreground' />
            <p className='text-muted-foreground'>
              Connected IdP: Google Workspace (acme.com) · 247 users · 12 groups
            </p>
          </div>
          <p className='text-sm text-muted-foreground mt-1'>
            Last synced: {lastSynced}
          </p>
        </div>
        <div className='flex items-center space-x-2'>
          <Button variant='outline' onClick={handleSyncNow}>
            <RefreshCw className='h-4 w-4 mr-2' />
            Sync now
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className='flex gap-4 mb-4 border-b'>
        <Button
          variant={activeTab === 'users' ? 'default' : 'ghost'}
          onClick={() => setActiveTab('users')}
          className='relative'
        >
          <UsersIcon className='h-4 w-4 mr-2' />
          Users
          {activeTab === 'users' && (
            <span className='absolute bottom-0 left-0 right-0 h-0.5 bg-primary'></span>
          )}
        </Button>
        <Button
          variant={activeTab === 'groups' ? 'default' : 'ghost'}
          onClick={() => setActiveTab('groups')}
          className='relative'
        >
          <UserCog className='h-4 w-4 mr-2' />
          Groups
          {activeTab === 'groups' && (
            <span className='absolute bottom-0 left-0 right-0 h-0.5 bg-primary'></span>
          )}
        </Button>
      </div>

      {/* Search */}
      {activeTab === 'users' && (
        <div className='relative mb-4'>
          <Search className='absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground' />
          <Input
            placeholder='Search users...'
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className='pl-10 max-w-md'
          />
        </div>
      )}

      {isLoading ? (
        <div className='flex items-center justify-center h-64'>
          <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
        </div>
      ) : activeTab === 'users' ? (
        <UsersTable
          users={filteredUsers}
          isLoading={isLoading}
          onUserClick={handleUserClick}
        />
      ) : (
        <GroupsTable
          groups={groups}
          isLoading={isLoading}
        />
      )}
    </Main>
  )
}