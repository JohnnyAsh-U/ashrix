# Architecture Overview

## System Context

Ashrix separates its responsibilities into two distinct planes that operate independently:

| Plane | Components | Responsibility |
|---|---|---|
| **Control Plane** | `cmd/cp` | Identity, PKI, policy authoring, session registry, gateway C2 |
| **Data Plane** | `cmd/gateway`, `cmd/connector` | Policy enforcement, traffic proxying, application access |

This separation is a deliberate architectural choice. It means:

1. Application traffic never passes through the Control Plane.
2. The Control Plane can be unavailable without immediately terminating active user sessions.
3. The gateway can verify all CP-issued artefacts without calling back to the CP on every request.

---

## High-Level Component Diagram

```
┌──────────────────────────────────────────────────────────────┐
│                     ASHRIX CONTROL PLANE                     │
│                                                              │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────────┐ │
│  │  Identity / │  │Policy Engine │  │   Session Registry  │ │
│  │  SSO (IdP)  │  │ + App Catalog│  │   (per-tenant DB)   │ │
│  └─────────────┘  └──────────────┘  └─────────────────────┘ │
│                                                              │
│         ◄── gRPC / mTLS (outbound from Gateway) ──►         │
└───────────────────────────┬──────────────────────────────────┘
                            │ Control Stream (persistent)
            ┌───────────────┼──────────────────┐
            │               │                  │
      ┌─────▼──────┐  ┌─────▼──────┐   ┌──────▼─────┐
      │  GW-US     │  │  GW-Asia   │   │  GW-Africa │
      │ (gateway)  │  │ (gateway)  │   │ (gateway)  │
      │            │  │            │   │            |
      └─────┬──────┘  └─────┬──────┘   └──────┬─────┘
            │ QUIC/mTLS     │                  │
      ┌─────▼──────┐  ┌─────▼──────┐   ┌──────▼─────┐
      │ Connector  │  │ Connector  │   │ Connector  │
      │   (US)     │  │  (Asia)    │   │ (Africa)   │
      └─────┬──────┘  └─────┬──────┘   └──────┬─────┘
      [US Apps]        [Asia Apps]       [Africa Apps]
```

### Traffic Flow (Numbered)

1. **Gateway → CP**: Persistent gRPC/mTLS stream (outbound). Carries registration, config sync, policy deltas, heartbeats, audit logs, and command receipt.
2. **Connector → Gateway**: Outbound QUIC/mTLS connection for data-plane traffic. Outbound gRPC/mTLS connection for management-plane traffic.
3. **User → Gateway**: HTTPS (browser) on port 443. Terminated at the gateway; the user never communicates with the CP directly during a session.
4. **Gateway → App**: Proxied through the connector tunnel. The gateway selects the correct connector and opens a QUIC stream to the target application.

> **Key invariant:** The CP is never in the hot path for application requests. Once the gateway has a valid session and a current policy bundle, it can serve traffic independently.

---

## Component Responsibilities

### Control Plane

The CP is the **single source of truth** for:

- Identity: it connects to tenant IdPs via OIDC to authenticate users.
- PKI: it issues and manages all gateway and connector identity certificates.
- Policy: it authors, stores, versions, and signs access rules.
- Session registry: it tracks active user sessions across all gateways for revocation targeting.
- Gateway command-and-control: it pushes configuration, policy deltas, CRL updates, and revocation commands to registered gateways.

The CP does **not**:

- Handle application payload traffic.
- Issue TLS certificates for user-facing HTTPS (that is the gateway's responsibility using a standard public CA).
- Make per-request authorisation decisions (policy is distributed to and evaluated at the gateway).

### Gateway

The gateway is a **stateful reverse proxy** and **Policy Enforcement Point (PEP)**. It:

- Accepts user connections over HTTPS (`chi` HTTP router, `internal/gateway/router`).
- Validates session cookies against local Redis session store (`internal/gateway/session`).
- Enforces access policy from the locally cached, CP-signed policy bundle (`internal/gateway/policy`).
- Maintains a persistent outbound gRPC/mTLS connection to the CP (`internal/gateway/grpc_client`).
- Accepts inbound Connector registrations and routes application traffic over QUIC tunnels.
- Logs all access events (OTEL Instrumentation on the gateway).

**Important distinction:** The gateway contains both a Policy Decision Point (PDP) — it evaluates access rules — and a Policy Enforcement Point (PEP) — it enforces the decision by allowing or refusing the proxy. These responsibilities are logically separated within the codebase but physically co-located in the same process.

### Connector

The connector is a **lightweight outbound tunnel agent**. It:

- Maintains no inbound listening ports.
- Establishes an outbound mTLS gRPC stream to the gateway for management messages (`internal/connector/management`).
- Establishes an outbound mTLS QUIC connection to the gateway for data-plane traffic (`internal/connector/transport`, `internal/connector/tunnel`).
- Maintains a local app list defining which internal applications it is authorised to proxy.
- Forwards QUIC streams from the gateway to the appropriate local application.
- Verifies all management payloads from the gateway using the embedded CP bundle-signing public key.

---

## Trust Architecture

The entire Ashrix trust model is built on three properties:

1. **The CP is the sole trust authority.** No component can issue, revoke, or modify trust without the CP's participation.
2. **The Gateway distributes, but never creates.** A compromised gateway cannot forge bundles or commands because it does not possess the CP's bundle-signing private key.

---

## Repository Structure

```
backend/
├── cmd/
│   ├── cp/          # Control Plane entry point
│   ├── gateway/     # Gateway entry point
│   └── connector/   # Connector entry point
│   
│
├── internal/
│   ├── cp/          # CP domain packages (auth, policy, pki_ca, gateway, connector, ...)
│   ├── gateway/     # Gateway packages (router, session, policy, grpc_client, crypto, ...)
│   └── connector/   # Connector packages (management, transport, tunnel, storage, ...)
│
├── pkg/             # Shared packages (crypto, pki, logger, frame, flow, filehelper)
├── proto/           # Protobuf definitions (ashrix-proto.proto + generated Go code)
└── docs/            # This documentation
```

---

## Key Architectural Decisions

| Decision | Rationale | Reference |
|---|---|---|
| Separate control plane from data plane | Availability isolation; gateway serves traffic even when CP is offline | [ADR-001](../10-engineering-decisions/adr-001-control-plane-separation.md) |
| QUIC for connector-to-gateway tunnel | Multiplexed streams, built-in TLS | [ADR-002](../10-engineering-decisions/adr-002-quic-transport.md) |
| gRPC for control-plane communication | Typed interfaces, bidirectional streaming, Protobuf compatibility | [ADR-003](../10-engineering-decisions/adr-003-grpc-control-channel.md) |
| Application-level access over VPN | Per-app enforcement, fine-grained revocation, no network-wide exposure | [ADR-004](../10-engineering-decisions/adr-004-application-level-access.md) |

---

*Next: [Control Plane →](./control-plane.md)*
