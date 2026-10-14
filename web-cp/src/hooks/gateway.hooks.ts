import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { gatewayService } from '@/services/gateway.service'
import type {
  CreateGatewayRequest,
  GatewayApiResponse,
  RevokeGatewayRequest,
  UpdateGatewayRequest,
} from '@/types/gateway'

export const gatewayQueryKey = ['gateways']

export const useGatewaysQuery = () =>
  useQuery({
    queryKey: gatewayQueryKey,
    queryFn: gatewayService.list,
    refetchInterval: 10000
  })

export const useGatewayQuery = (id: string | null, enabled = true) =>
  useQuery({
    queryKey: [...gatewayQueryKey, id],
    queryFn: () => gatewayService.get(id as string),
    enabled: enabled && Boolean(id),
    refetchInterval: (query) => query.state.data?.status === 'healthy' ? false : 3000,
  })

export const useCreateGatewayMutation = () => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateGatewayRequest) => gatewayService.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: gatewayQueryKey })
    },
  })
}

const useGatewayMutation = <TPayload, TResult>(mutationFn: (payload: TPayload) => Promise<TResult>) => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: gatewayQueryKey })
    },
  })
}

export const useUpdateGatewayMutation = () =>
  useGatewayMutation<UpdateGatewayRequest, GatewayApiResponse>((payload) => gatewayService.update(payload))

export const useRevokeGatewayMutation = () =>
  useGatewayMutation<RevokeGatewayRequest, GatewayApiResponse>((payload) => gatewayService.revoke(payload))

export const useRotateGatewayCertificateMutation = () =>
  useGatewayMutation<string, GatewayApiResponse>((id) => gatewayService.rotateCertificate(id))

export const useDrainGatewayMutation = () =>
  useGatewayMutation<string, GatewayApiResponse>((id) => gatewayService.drain(id))
