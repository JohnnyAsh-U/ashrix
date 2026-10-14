import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Loader2, Users as UsersIcon } from 'lucide-react'
import type { Group } from '@/types/user'

interface GroupsTableProps {
  groups: Group[]
  isLoading?: boolean
}

export function GroupsTable({ groups, isLoading }: GroupsTableProps) {
  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>No groups found</p>
      </div>
    )
  }

  return (
    <div className='border rounded-lg overflow-hidden'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Group Name</TableHead>
            <TableHead>Members</TableHead>
            <TableHead>Description</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {groups.map((group) => (
            <TableRow key={group.id}>
              <TableCell className='font-medium'>{group.name}</TableCell>
              <TableCell>
                <div className='flex items-center gap-2'>
                  <UsersIcon className='h-4 w-4 text-muted-foreground' />
                  {group.members}
                </div>
              </TableCell>
              <TableCell>{group.description}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}