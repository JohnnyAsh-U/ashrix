import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { policyService } from '@/services/policy.service'
import type { CreatePolicyRequest, UpdatePolicyRequest } from '@/types/policy-api'

export const policiesQueryKey = ['policies']

export const usePoliciesQuery = () => useQuery({ queryKey: policiesQueryKey, queryFn: policyService.list })

export const useCreatePolicyMutation = () => {
  const client = useQueryClient()
  return useMutation({ mutationFn: (payload: CreatePolicyRequest) => policyService.create(payload), onSuccess: () => client.invalidateQueries({ queryKey: policiesQueryKey }) })
}

export const useUpdatePolicyMutation = () => {
  const client = useQueryClient()
  return useMutation({ mutationFn: ({ id, payload }: { id: string; payload: UpdatePolicyRequest }) => policyService.update(id, payload), onSuccess: () => client.invalidateQueries({ queryKey: policiesQueryKey }) })
}

export const useDeletePolicyMutation = () => {
  const client = useQueryClient()
  return useMutation({ mutationFn: policyService.remove, onSuccess: () => client.invalidateQueries({ queryKey: policiesQueryKey }) })
}

export const usePolicyQuery = (id: string) => useQuery({ queryKey: ['policy', id], queryFn: () => policyService.get(id), enabled: !!id })
export const usePolicyHistoryQuery = (id: string) => useQuery({ queryKey: ['policy-history', id], queryFn: () => policyService.getHistory(id), enabled: !!id })