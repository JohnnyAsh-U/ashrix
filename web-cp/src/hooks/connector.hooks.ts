import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { connectorService } from '@/services/connector.service'
import type {
  CreateConnectorRequest,
  RevokeConnectorRequest,
  UpdateConnectorRequest,
} from '@/types/connector'

export const connectorQueryKey = ['connectors']

export const useConnectorsQuery = () =>
  useQuery({
    queryKey: connectorQueryKey,
    queryFn: connectorService.list,
    refetchInterval: 10000
  })

  export const useConnectorQuery = (id: string | null, enabled = true) =>
    useQuery({
      queryKey: [...connectorQueryKey, id],
      queryFn: () => connectorService.get(id as string),
      enabled: enabled && Boolean(id),
      refetchInterval: (query) => query.state.data?.status === 'connected' ? false : 3000,
    })



const useConnectorMutation = <TPayload, TResult>(mutationFn: (payload: TPayload) => Promise<TResult>) => {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: connectorQueryKey })
    },
  })
}

export const useCreateConnectorMutation = () =>
  useConnectorMutation((payload: CreateConnectorRequest) => connectorService.create(payload))

export const useUpdateConnectorMutation = () =>
  useConnectorMutation((payload: UpdateConnectorRequest) => connectorService.update(payload))

export const useRevokeConnectorMutation = () =>
  useConnectorMutation((payload: RevokeConnectorRequest) => connectorService.revoke(payload))

export const useRotateConnectorCertificateMutation = () =>
  useConnectorMutation((id: string) => connectorService.rotateCertificate(id))

