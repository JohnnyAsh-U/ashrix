import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Loader2, Circle } from 'lucide-react'
import type { User } from '@/types/user'

interface UsersTableProps {
  users: User[]
  isLoading?: boolean
  onUserClick: (user: User) => void
}

export function UsersTable({ users, isLoading, onUserClick }: UsersTableProps) {
  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (users.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>No users found</p>
      </div>
    )
  }

  return (
    <div className='border rounded-lg overflow-hidden'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Email</TableHead>
            <TableHead>Groups</TableHead>
            <TableHead>Last Seen</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {users.map((user) => (
            <TableRow 
              key={user.id} 
              className='cursor-pointer hover:bg-muted/50'
              onClick={() => onUserClick(user)}
            >
              <TableCell className='font-medium'>
                <div className='flex items-center gap-2'>
                  <Circle 
                    className={`h-2 w-2 ${user.status === 'online' ? 'text-green-500 fill-green-500' : 'text-gray-300 fill-gray-300'}`} 
                  />
                  {user.name}
                </div>
              </TableCell>
              <TableCell>{user.email}</TableCell>
              <TableCell>
                <div className='flex flex-wrap gap-1'>
                  {user.groups.map((group, idx) => (
                    <Badge key={idx} variant='secondary' className='text-xs'>
                      {group}
                    </Badge>
                  ))}
                </div>
              </TableCell>
              <TableCell>
                <span className={user.status === 'online' ? 'text-green-600 font-medium' : 'text-muted-foreground'}>
                  {user.lastSeen}
                </span>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}