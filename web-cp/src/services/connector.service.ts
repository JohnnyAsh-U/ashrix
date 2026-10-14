import api from '@/config/api'
import {
  ConnectorsUrl,
  ConnectorUrl,
  RecreateConnectorUrl,
  RevokeConnectorUrl,
  RotateConnectorCertificateUrl,
} from '@/utils/constant'
import type {
  ConnectorApiResponse,
  CreateConnectorRequest,
  RevokeConnectorRequest,
  UpdateConnectorRequest,
} from '@/types/connector'

type ConnectorResponse = { data: ConnectorApiResponse }

export const connectorService = {
    get: async (id: string): Promise<ConnectorApiResponse> => {
    const { data } = await api.get<ConnectorResponse>(ConnectorUrl(id))
    return data.data
  },

  list: async (): Promise<ConnectorApiResponse[]> => {
    const { data } = await api.get<ConnectorApiResponse[] | { data: ConnectorApiResponse[] }>(ConnectorsUrl)
    return Array.isArray(data) ? data : data.data
  },

  create: async (payload: CreateConnectorRequest): Promise<ConnectorApiResponse> => {
    const { data } = await api.post<ConnectorResponse>(ConnectorsUrl, payload)
    return data.data
  },

  update: async ({ id, ...payload }: UpdateConnectorRequest) => {
    const { data } = await api.put<ConnectorResponse>(RecreateConnectorUrl(id), payload)
    return data.data
  },

  revoke: async ({ id, revoke_reason }: RevokeConnectorRequest) => {
    const { data } = await api.put<ConnectorResponse>(RevokeConnectorUrl(id), { revoke_reason })
    return data.data
  },

  rotateCertificate: async (id: string) => {
    const { data } = await api.put<ConnectorResponse>(RotateConnectorCertificateUrl(id))
    return data.data
  },
}