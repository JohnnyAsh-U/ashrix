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
  ChevronDown, 
  ChevronRight, 
  CheckCircle,
  XCircle,
  Loader2,
  User as UserIcon,
  Shield,
  ArrowRight
} from 'lucide-react'
import type { AccessLogEntry } from '@/types/access-log'
import { toast } from 'sonner'

interface AccessLogTableProps {
  logs: AccessLogEntry[]
  isLoading?: boolean
}

export function AccessLogTable({ logs, isLoading }: AccessLogTableProps) {
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  const toggleExpand = (logId: string) => {
    const newExpanded = new Set(expandedRows)
    if (newExpanded.has(logId)) {
      newExpanded.delete(logId)
    } else {
      newExpanded.add(logId)
    }
    setExpandedRows(newExpanded)
  }

  const handleViewUser = (log: AccessLogEntry) => {
    console.log('Navigate to user:', log.user)
    toast.info(`Opening ${log.userName || log.user}'s profile`)
  }

  const handleAddPolicy = (log: AccessLogEntry) => {
    console.log('Creating policy for:', log.app, 'with groups:', log.groups)
    toast.success(`Creating policy for ${log.app} with suggested groups: ${log.groups.join(', ')}`)
    // Navigate to Create Rule form with pre-filled data
  }

  if (isLoading) {
    return (
      <div className='flex items-center justify-center h-64'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (logs.length === 0) {
    return (
      <div className='border rounded-lg p-8 text-center'>
        <p className='text-muted-foreground'>No access logs found</p>
      </div>
    )
  }

  return (
    <div className='border rounded-lg overflow-hidden'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='w-8'></TableHead>
            <TableHead>Time</TableHead>
            <TableHead>User</TableHead>
            <TableHead>App</TableHead>
            <TableHead>Method</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Latency</TableHead>
            <TableHead>Result</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {logs.map((log) => (
            <>
              <TableRow 
                key={log.id} 
                className='cursor-pointer hover:bg-muted/50'
                onClick={() => toggleExpand(log.id)}
              >
                <TableCell>
                  {expandedRows.has(log.id) ? (
                    <ChevronDown className='h-4 w-4' />
                  ) : (
                    <ChevronRight className='h-4 w-4' />
                  )}
                </TableCell>
                <TableCell className='font-mono text-sm'>{log.time}</TableCell>
                <TableCell className='font-medium'>{log.user}</TableCell>
                <TableCell>{log.app}</TableCell>
                <TableCell>
                  {log.method ? (
                    <Badge variant='outline' className='font-mono'>
                      {log.method}
                    </Badge>
                  ) : (
                    <span className='text-muted-foreground'>—</span>
                  )}
                </TableCell>
                <TableCell>
                  {log.status ? (
                    <span className={`font-mono ${
                      log.status < 400 ? 'text-green-600' : 'text-red-600'
                    }`}>
                      {log.status}
                    </span>
                  ) : (
                    <span className='text-muted-foreground'>—</span>
                  )}
                </TableCell>
                <TableCell>
                  {log.latency ? (
                    <span className='font-mono text-sm'>{log.latency}</span>
                  ) : (
                    <span className='text-muted-foreground'>—</span>
                  )}
                </TableCell>
                <TableCell>
                  {log.result === 'allowed' ? (
                    <span className='inline-flex items-center gap-1 text-green-600'>
                      <CheckCircle className='h-4 w-4' /> allowed
                    </span>
                  ) : (
                    <span className='inline-flex items-center gap-1 text-red-600'>
                      <XCircle className='h-4 w-4' /> denied
                    </span>
                  )}
                </TableCell>
              </TableRow>
              {expandedRows.has(log.id) && (
                <TableRow key={`${log.id}-expanded`}>
                  <TableCell colSpan={8} className='bg-muted/30 p-0'>
                    <ExpandedLogDetails log={log} onViewUser={handleViewUser} onAddPolicy={handleAddPolicy} />
                  </TableCell>
                </TableRow>
              )}
            </>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function ExpandedLogDetails({ 
  log, 
  onViewUser, 
  onAddPolicy 
}: { 
  log: AccessLogEntry
  onViewUser: (log: AccessLogEntry) => void
  onAddPolicy: (log: AccessLogEntry) => void
}) {
  return (
    <div className='p-6 space-y-4'>
      {/* Header */}
      <div className='flex items-center gap-2'>
        <span className='font-mono text-sm'>{log.time}</span>
        <ArrowRight className='h-4 w-4 text-muted-foreground' />
        <span className='font-medium'>{log.user}</span>
        <ArrowRight className='h-4 w-4 text-muted-foreground' />
        <span className='font-medium'>{log.app}</span>
      </div>

      {/* Details Grid */}
      <div className='grid grid-cols-2 gap-4'>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Result</p>
          {log.result === 'allowed' ? (
            <p className='text-sm font-medium text-green-600'>ALLOWED</p>
          ) : (
            <p className='text-sm font-medium text-red-600'>DENIED</p>
          )}
        </div>
        {log.reason && (
          <div>
            <p className='text-sm font-medium text-muted-foreground'>Reason</p>
            <p className='text-sm font-mono'>{log.reason}</p>
          </div>
        )}
        <div>
          <p className='text-sm font-medium text-muted-foreground'>User</p>
          <p className='text-sm'>{log.user}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Groups</p>
          <div className='flex flex-wrap gap-1'>
            {log.groups.length > 0 ? (
              log.groups.map((group, idx) => (
                <Badge key={idx} variant='secondary' className='text-xs'>
                  {group}
                </Badge>
              ))
            ) : (
              <span className='text-sm text-muted-foreground'>None</span>
            )}
          </div>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>App</p>
          <p className='text-sm'>
            {log.app} ({log.appHostname})
          </p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Method</p>
          <p className='text-sm font-mono'>{log.method || '—'}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Path</p>
          <p className='text-sm font-mono'>{log.path || '—'}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>IP</p>
          <p className='text-sm font-mono'>{log.ip}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Gateway</p>
          <p className='text-sm'>{log.gateway}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Session</p>
          <p className='text-sm font-mono'>{log.sessionId || '—'}</p>
        </div>
        <div>
          <p className='text-sm font-medium text-muted-foreground'>Timestamp</p>
          <p className='text-sm font-mono'>{log.timestamp}</p>
        </div>
      </div>

      {/* Action Buttons */}
      <div className='flex gap-2 pt-4 border-t'>
        {log.user !== 'unknown' && (
          <Button 
            variant='outline' 
            size='sm'
            onClick={() => onViewUser(log)}
          >
            <UserIcon className='h-4 w-4 mr-1' /> View user {log.userName?.split(' ')[0] || log.user} →
          </Button>
        )}
        {log.result === 'denied' && log.reason === 'no_matching_policy' && (
          <Button 
            variant='outline' 
            size='sm'
            onClick={() => onAddPolicy(log)}
            className='text-blue-600'
          >
            <Shield className='h-4 w-4 mr-1' /> Add policy for {log.app} →
          </Button>
        )}
      </div>
    </div>
  )
}