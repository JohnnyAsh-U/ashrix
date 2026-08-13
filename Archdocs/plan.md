# Implementation Plan - Gateway & Connector Revocation and Stream Verification

This plan outlines the changes required to support gateway and connector revocation, status updates, certificate revocation list (CRL) propagation, stream checks, and control plane commands.

## User Review Required

> [!IMPORTANT]
> - We will create a database migration to alter the CHECK constraints on the `gateways` and `connectors` tables so they can support `'revoked'` and `'draining'` statuses.
> - We will update the gRPC stream handshake on the CP so that if a gateway is revoked, or its certificate is in the CRL list, the connection is refused.
> - If another active connection already exists in the registry for the given `gateway_id`, the new stream connection will be rejected with an `AlreadyExists` error.

## Open Questions

> [!NOTE]
> No immediate open questions. We will use the standard gRPC status codes for connection rejections.

## Proposed Changes

### Database Migration

#### [NEW] [20260813170000_add_revoked_draining_statuses.sql](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/cp/database/migrations/20260813170000_add_revoked_draining_statuses.sql)
- Drops the old check constraints `gateways_status_check` and `connectors_status_check`.
- Re-adds them to include `'revoked'` and `'draining'` for gateways, and `'revoked'` for connectors.

### Protobuf Updates

#### [MODIFY] [ashrix-gateway.proto](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/proto/ashrix-gateway.proto)
- Define new command messages:
  - `RotateGatewayCertCmd`, `RevokeGatewayCertCmd`, `RevokeGatewayCmd`, `DrainGatewayCmd`
  - `RevokeSessionCmd` (containing `session_id`)
  - `RevokeConnectorCmd`, `RotateConnectorCertCmd`, `RevokeConnectorCertCmd` (each containing `connector_id`)
  - `CrlSyncCmd` (containing list of revoked certificate serial numbers)
  - `ConnectorSyncCmd` (containing list of authorized connectors and their statuses)
- Update `CPEnvelope` payload oneof to include a generic `Command cmd = 12` (or individual commands in the oneof).

### Control Plane Logic

#### [MODIFY] [stream_handler.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/cp/platform/cp_grpc/stream_handler.go)
- In `Connect`, before accepting the stream connection:
  - Verify that the gateway exists and its status is not `'revoked'`.
  - Extract the peer certificate, and check if its serial number is in the `crl_entries` table. If so, return `codes.Unauthenticated`.
  - Check if a connection for this `gateway_id` already exists in `s.registry`. If so, reject it with `codes.AlreadyExists`.
- Once the stream is successfully established, immediately send:
  - `CrlSyncCmd` containing all revoked certificate serial numbers.
  - `ConnectorSyncCmd` containing the authorized connectors and statuses for this gateway.

#### [MODIFY] [gateway_service.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/cp/gateway/gateway_service.go)
- Update the `RevokeGateway` method to:
  - Change the gateway status to `'revoked'` instead of `'offline'`.
  - Insert the revoked certificate into `crl_entries`.
  - Push a `RevokeGatewayCmd` to the gateway if connected, and close its stream.
- Update `RevokeGatewayCert` to:
  - Insert the certificate into `crl_entries`.
  - Push a `RevokeGatewayCertCmd` to the gateway.
- Add `DrainGateway` method to:
  - Set gateway status to `'draining'`.
  - Push `DrainGatewayCmd` to the gateway.

#### [MODIFY] [connector_service.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/cp/connector/connector_service.go)
- Update `RevokeConnector` and `RevokeConnectorCert` to:
  - Update DB fields (set status to `'revoked'`).
  - Write to `crl_entries`.
  - Find the concerned gateway and push `RevokeConnectorCmd` or `RevokeConnectorCertCmd` to it so it cuts connection with the connector.

#### [MODIFY] [identity_service.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/cp/identity/identity_service.go)
- In user session revocation, when a session is deleted in the database, find the corresponding gateway and push `RevokeSessionCmd` (containing `session_id`) to that gateway.
- In `BuildOAuthUrl`, check if the gateway is `'draining'` or `'revoked'`. If so, return an error and reject the login attempt.

### Gateway Logic

#### [MODIFY] [stream.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/gateway/grpc_client/stream.go)
- Add a reference to the `redis.Client` (or a session destroyer handler).
- In `handleMessage`, process incoming commands:
  - `RevokeSessionCmd`: Delete the session from Redis (`session:{session_id}`).
  - `RevokeConnectorCmd`/`RevokeConnectorCertCmd`/`RotateConnectorCertCmd`: Call the registry to cut the connection with that connector.
  - `CrlSyncCmd`: Persist the CRL entries list (e.g. in-memory or in bbolt db) so that incoming connector certs can be verified.
  - `ConnectorSyncCmd`: Persist the authorized connectors list.
- In `NewStreamManager`, accept the Redis client parameter.

#### [MODIFY] [handler.go](file:///run/media/johnnyash/New%20Volume/Ashrix/backend/internal/gateway/server/grpc/handler.go)
- In the connector `Connect` handler:
  - Extract the connector's client certificate.
  - Check if the serial number is in the CRL entries list, and check if the connector is in the authorized connectors list. If not, reject the connection.

## Verification Plan

### Automated Tests
- Run `go test ./...` in the backend directory to verify compile and existing tests.
- Add unit tests for gateway/connector status and connection verification rules.

### Manual Verification
- DeployCP and Gateway locally, verify that revoked gateways cannot connect, and that revoking a session deletes the redis key on the gateway.
