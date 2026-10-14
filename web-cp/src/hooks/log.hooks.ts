import { useQuery } from '@tanstack/react-query'
import { logService } from '@/services/log.service'
import type { AccessLogQuery } from '@/types/access-log'

export const accessLogsQueryKey = ['logs', 'access']

export const useAccessLogsQuery = (query: AccessLogQuery = {}) =>
  useQuery({
    queryKey: [...accessLogsQueryKey, query],
    queryFn: () => logService.listAccess(query),
  })