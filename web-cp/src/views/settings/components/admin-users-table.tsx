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
import { Badge } from '@/components/ui/badge'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { 
  MoreHorizontal,
  Shield,
  UserCog,
  Eye,
  AlertTriangle,
  Loader2,
  Crown
} from 'lucide-react'
import type { AdminUser } from '@/types/admin-user'
import { toast } from 'sonner'
import { Label } from '@/components/ui/label'
// import { Label } from 'radix-ui'

interface AdminUsersTableProps {
  adminUsers: AdminUser[]
  isLoading?: boolean
}

export function AdminUsersTable({ adminUsers, isLoading }: AdminUsersTableProps) {
  const [changeRoleDialogOpen, setChangeRoleDialogOpen] = useState(false)
  const [removeDialogOpen, setRemoveDialogOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<AdminUser | null>(null)
  const [newRole, setNewRole] = useState<string>('')

  const handleChangeRole = (user: AdminUser) => {
    setSelectedUser(user)
    setNewRole(user.role)
    setChangeRoleDialogOpen(true)
  }

  const handleRemove = (user: AdminUser) => {
    setSelectedUser(user)
    setRemoveDialogOpen(true)
  }

  const confirmChangeRole = () => {
    console.log('Changing role:', selectedUser?.name, 'to', newRole)
    toast.success(`Role updated for ${selectedUser?.name}`)
    setChangeRoleDialogOpen(false)
    setSelectedUser(null)
  }

  const confirmRemove = () => {
    console.log('Removing admin:', selectedUser?.name)
    toast.success(`${selectedUser?.name} removed from admin users`)
    setRemoveDialogOpen(false)
    setSelectedUser(null)
  }

  const getRoleBadge = (role: string) => {
    switch (role) {
      case 'Owner':
        return (
          <Badge className='bg-purple-100 text-purple-800'>
            <Crown className='h-3 w-3 mr-1' />
            Owner
          </Badge>
        )
      case 'Admin':
        return (
          <Badge className='bg-blue-100 text-blue-800'>
            <Shield className='h-3 w-3 mr-1' />
            Admin
          </Badge>
        )
      case 'Auditor':
        return (
          <Badge className='bg-gray-100 text-gray-800'>
            <Eye className='h-3 w-3 mr-1' />
            Auditor
          </Badge>
        )
      default:
        return <Badge>{role}</Badge>
    }
  }

  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (adminUsers.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>No admin users found</p>
      </div>
    )
  }

  return (
    <>
      <div className='border rounded-lg overflow-hidden'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>Joined</TableHead>
              <TableHead className='text-right'>Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {adminUsers.map((user) => (
              <TableRow key={user.id}>
                <TableCell className='font-medium'>{user.name}</TableCell>
                <TableCell>{user.email}</TableCell>
                <TableCell>{getRoleBadge(user.role)}</TableCell>
                <TableCell>{user.joined}</TableCell>
                <TableCell className='text-right'>
                  {user.canRemove ? (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant='ghost' size='icon'>
                          <MoreHorizontal className='h-4 w-4' />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align='end' className='w-48'>
                        <DropdownMenuItem onClick={() => handleChangeRole(user)}>
                          <UserCog className='h-4 w-4 mr-2' /> Change role
                        </DropdownMenuItem>
                        <DropdownMenuItem 
                          onClick={() => handleRemove(user)}
                          className='text-red-600'
                        >
                          <AlertTriangle className='h-4 w-4 mr-2' /> Remove
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  ) : (
                    <span className='text-xs text-muted-foreground'>—</span>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Change Role Dialog */}
      <Dialog open={changeRoleDialogOpen} onOpenChange={setChangeRoleDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Change Role</DialogTitle>
            <DialogDescription>
              Update the role for {selectedUser?.name}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='bg-muted/50 rounded-lg p-4'>
              <p className='text-sm font-medium'>{selectedUser?.name}</p>
              <p className='text-sm text-muted-foreground'>{selectedUser?.email}</p>
            </div>

            <div className='space-y-2'>
                <Label>New role</Label>
              <Select value={newRole} onValueChange={setNewRole}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='Admin'>Admin</SelectItem>
                  <SelectItem value='Auditor'>Auditor</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className='bg-yellow-50 border border-yellow-200 rounded-lg p-3'>
              <p className='text-sm text-yellow-800'>
                Role changes take effect immediately.
              </p>
            </div>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setChangeRoleDialogOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              onClick={confirmChangeRole}
              className='flex-1'
            >
              Update Role
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove Admin Dialog */}
      <Dialog open={removeDialogOpen} onOpenChange={setRemoveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <AlertTriangle className='h-5 w-5 text-red-600' />
              Remove Admin
            </DialogTitle>
            <DialogDescription>
              Are you sure you want to remove this admin user?
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            <div className='bg-muted/50 rounded-lg p-4'>
              <p className='text-sm font-medium'>{selectedUser?.name}</p>
              <p className='text-sm text-muted-foreground'>{selectedUser?.email}</p>
              <p className='text-sm text-muted-foreground mt-1'>
                Role: {selectedUser?.role}
              </p>
            </div>
            <div className='bg-red-50 border border-red-200 rounded-lg p-3'>
              <p className='text-sm text-red-700'>
                {selectedUser?.name} will immediately lose access to the admin console.
              </p>
            </div>
          </div>

          <DialogFooter className='flex gap-2'>
            <Button
              variant='outline'
              onClick={() => setRemoveDialogOpen(false)}
              className='flex-1'
            >
              Cancel
            </Button>
            <Button
              variant='destructive'
              onClick={confirmRemove}
              className='flex-1'
            >
              Remove Admin
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}