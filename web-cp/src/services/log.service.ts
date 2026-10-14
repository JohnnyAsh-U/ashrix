import api from '@/config/api'
import { AccessLogsUrl } from '@/utils/constant'
import type { AccessLogApiResponse, AccessLogQuery } from '@/types/access-log'

export const logService = {
  listAccess: async (query: AccessLogQuery = {}): Promise<AccessLogApiResponse[]> => {
    const { data } = await api.get<AccessLogApiResponse[] | { data: AccessLogApiResponse[] }>(AccessLogsUrl, {
      params: query,
    })
    return Array.isArray(data) ? data : data.data
  },
}