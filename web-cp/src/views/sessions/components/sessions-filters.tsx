import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'

interface SessionFiltersProps {
  onFilterChange: (type: 'user' | 'app' | 'gateway', value: string) => void
  users: string[]
  gateways: string[]
}

export function SessionFilters({
  onFilterChange,
  users,
  gateways
}: SessionFiltersProps) {
  return (
    <div className='flex flex-wrap items-center gap-4'>
      <div className='flex items-center gap-2'>
        <Label className='text-sm'>User</Label>
        <Label className='text-sm'>Utilisateur</Label>
        <Select onValueChange={(value) => onFilterChange('user', value)}>
          <SelectTrigger className='w-48'>
            <SelectValue placeholder="All users" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>All users</SelectItem>
            {users.map((user) => (
              <SelectItem key={user} value={user}>
                {user}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className='flex items-center gap-2'>
        <Label className='text-sm'>Passerelle</Label>
        <Select onValueChange={(value) => onFilterChange('gateway', value)}>
          <SelectTrigger className='w-44'>
            <SelectValue placeholder="All gateways" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>All gateways</SelectItem>
            {gateways.map((gateway) => (
              <SelectItem key={gateway} value={gateway}>
                {gateway}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  )
}