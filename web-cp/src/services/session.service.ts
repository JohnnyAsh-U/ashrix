import api from '@/config/api'
import { ActiveSessionsUrl, RevokeSessionUrl } from '@/utils/constant'
import type { ActiveSessionApiResponse } from '@/types/session'

export const sessionService = {
  listActive: async (params: { gateway?: string; email?: string } = {}): Promise<ActiveSessionApiResponse[]> => {
    const { data } = await api.get<{ data: ActiveSessionApiResponse[] }>(ActiveSessionsUrl, { params })
    console.log(data)
    return data.data
  },

  revoke: async (id: string): Promise<void> => {
    await api.delete(RevokeSessionUrl(id))
  },
}