import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { identityService } from '@/services/identity.service'
import type { CreateIdentityConfigRequest } from '@/types/identity'

export const identityConfigsQueryKey = ['identity-configs']

export const useIdentityConfigsQuery = () =>
  useQuery({ queryKey: identityConfigsQueryKey, queryFn: identityService.list })

export const useCreateIdentityConfigMutation = () => {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateIdentityConfigRequest) => identityService.create(payload),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: identityConfigsQueryKey }),
  })
}