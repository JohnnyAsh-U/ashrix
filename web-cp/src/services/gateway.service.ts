import api from '@/config/api'
import {
  DrainGatewayUrl,
  GatewaysUrl,
  GatewayUrl,
  RecreateGatewayUrl,
  RevokeGatewayUrl,
  RotateGatewayCertificateUrl,
} from '@/utils/constant'
import type {
  CreateGatewayRequest,
  GatewayApiResponse,
  RevokeGatewayRequest,
  UpdateGatewayRequest,
} from '@/types/gateway'

type GatewayResponse = { data: GatewayApiResponse }

export const gatewayService = {
  get: async (id: string): Promise<GatewayApiResponse> => {
    const { data } = await api.get<GatewayResponse>(GatewayUrl(id))
    return data.data
  },

  list: async (): Promise<GatewayApiResponse[]> => {
    const { data } = await api.get<GatewayApiResponse[] | { data: GatewayApiResponse[] }>(GatewaysUrl)
    return Array.isArray(data) ? data : data.data
  },

  create: async (payload: CreateGatewayRequest): Promise<GatewayApiResponse> => {
    const { data } = await api.post<GatewayResponse>(GatewaysUrl, payload)
    return data.data
  },

  update: async ({ id, ...payload }: UpdateGatewayRequest): Promise<GatewayApiResponse> => {
    const { data } = await api.put<GatewayResponse>(RecreateGatewayUrl(id), payload)
    return data.data
  },

  revoke: async ({ id, revoke_reason }: RevokeGatewayRequest) => {
    const { data } = await api.put<GatewayResponse>(RevokeGatewayUrl(id), { revoke_reason })
    return data.data
  },

  rotateCertificate: async (id: string) => {
    const { data } = await api.put<GatewayResponse>(RotateGatewayCertificateUrl(id))
    return data.data
  },

  drain: async (id: string) => {
    const { data } = await api.put<GatewayResponse>(DrainGatewayUrl(id))
    return data.data
  },

}