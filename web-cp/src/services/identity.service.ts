import api from '@/config/api'
import { IdentityConfigsUrl } from '@/utils/constant'
import type { CreateIdentityConfigRequest, IdentityConfig } from '@/types/identity'

export const identityService = {
  list: async (): Promise<IdentityConfig[]> => {
    const { data } = await api.get<IdentityConfig[] | { data: IdentityConfig[] }>(IdentityConfigsUrl)
    return Array.isArray(data) ? data : data.data
  },

  create: async (payload: CreateIdentityConfigRequest): Promise<IdentityConfig> => {
    const { data } = await api.post<{ data: IdentityConfig }>(IdentityConfigsUrl, payload)
    return data.data
  },
}