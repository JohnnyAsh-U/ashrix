import api from '@/config/api'
import { ApplicationUrl, ApplicationsUrl } from '@/utils/constant'
import type {
  ApplicationApiResponse,
  CreateApplicationRequest,
  UpdateApplicationRequest,
} from '@/types/application'

type ApplicationResponse = { data: ApplicationApiResponse }

export const applicationService = {
  list: async (): Promise<ApplicationApiResponse[]> => {
    const { data } = await api.get<ApplicationApiResponse[] | { data: ApplicationApiResponse[] }>(ApplicationsUrl)
    return Array.isArray(data) ? data : data.data
  },

  create: async (payload: CreateApplicationRequest): Promise<ApplicationApiResponse> => {
    const { data } = await api.post<ApplicationResponse>(ApplicationsUrl, payload)
    return data.data
  },

  update: async ({ id, ...payload }: UpdateApplicationRequest): Promise<ApplicationApiResponse> => {
    const { data } = await api.put<ApplicationResponse>(ApplicationUrl(id), payload)
    return data.data
  },

  remove: async (id: string): Promise<ApplicationApiResponse> => {
    const { data } = await api.delete<ApplicationResponse>(ApplicationUrl(id))
    return data.data
  },
}