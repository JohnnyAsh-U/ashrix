import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { applicationService } from '@/services/application.service'
import type { CreateApplicationRequest, UpdateApplicationRequest } from '@/types/application'

export const applicationQueryKey = ['applications']

export const useApplicationsQuery = () =>
  useQuery({
    queryKey: applicationQueryKey,
    queryFn: applicationService.list,
  })

const useApplicationMutation = <TPayload>(mutationFn: (payload: TPayload) => Promise<unknown>) => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: applicationQueryKey })
    },
  })
}

export const useCreateApplicationMutation = () =>
  useApplicationMutation((payload: CreateApplicationRequest) => applicationService.create(payload))

export const useUpdateApplicationMutation = () =>
  useApplicationMutation((payload: UpdateApplicationRequest) => applicationService.update(payload))

export const useDeleteApplicationMutation = () =>
  useApplicationMutation((id: string) => applicationService.remove(id))