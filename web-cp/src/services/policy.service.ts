import api from '@/config/api'
import { PoliciesUrl, PolicyUrl } from '@/utils/constant'
import type { CreatePolicyRequest, PolicyApiResponse, UpdatePolicyRequest } from '@/types/policy-api'

export const policyService = {
  list: async (): Promise<PolicyApiResponse> => {
    const { data } = await api.get<{ data: PolicyApiResponse }>(PoliciesUrl, { params: { page: 1, limit: 100 } })
    return data.data
  },
  create: async (payload: CreatePolicyRequest) => {
    const { data } = await api.post<{ data: PolicyApiResponse }>(PoliciesUrl, payload)
    return data.data
  },
  update: async (id: string, payload: UpdatePolicyRequest) => {
    const { data } = await api.put<{ data: PolicyApiResponse }>(PolicyUrl(id), payload)
    return data.data
  },
  remove: async (id: string) => {
    await api.delete(PolicyUrl(id))
  },
  get: async (id: string) => {
    const { data } = await api.get<{ data: any }>(PolicyUrl(id))
    return data.data
  },
  getHistory: async (id: string) => {
    const { data } = await api.get<{ data: any[] }>(`${PolicyUrl(id)}/history`)
    return data.data
  },
}