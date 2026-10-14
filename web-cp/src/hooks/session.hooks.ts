import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { sessionService } from '@/services/session.service'

export const activeSessionsQueryKey = ['sessions', 'active']

export const useActiveSessionsQuery = (params: { gateway?: string; email?: string } = {}) =>
  useQuery({
    queryKey: [...activeSessionsQueryKey, params],
    queryFn: () => sessionService.listActive(params),
  })

export const useRevokeSessionMutation = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: sessionService.revoke,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: activeSessionsQueryKey }),
  })
}