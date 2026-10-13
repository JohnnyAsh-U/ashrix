# gRPC Control Channel

## Overview

Ashrix uses **gRPC over mTLS** for all management-plane communication:

- **Gateway ↔ Control Plane:** The primary Control Stream, carrying policy, config, commands, and audit data.
- **Connector ↔ Gateway:** The management stream, carrying heartbeats, and log forwarding.

gRPC was chosen for its typed interface (Protocol Buffers), bidirectional streaming, deadline propagation, and compatibility with Go's standard tooling. See [ADR-003](../10-engineering-decisions/adr-003-grpc-control-channel.md) for the full decision record.

---

## Protocol Buffers

All messages are defined in `proto/ashrix-proto.proto` and compiled to Go using `buf`. The `.proto` definitions are the authoritative interface contract between components.

Key message types ( verify against current `ashrix-proto.proto`):

| Message | Direction | Purpose |
|---|---|---|
| `Hello` | Gateway → CP | Announce gateway ID and current policy version at connection |
| `PolicyDelta` | CP → Gateway | Incremental policy update (from_version, to_version, mutations) |
| `PolicySnapshot` | CP → Gateway | Full policy bundle (used when delta is impossible) |
| `TrustBundle` | CP → Gateway | Updated CA certificate set |
| `CRLUpdate` | CP → Gateway | Updated Certificate Revocation List |
| `RevocationEvent` | CP → Gateway | Session or node certificate revocation |
| `RotationCommand` | CP → Gateway | Instruction to renew certificate |
| `Heartbeat` | Gateway → CP | Alive signal with operational state |
| `BundleACK` | Gateway → CP | Acknowledgement of received bundle |
| `SessionEvent` | Gateway → CP | Session created/refreshed/expired |
| `AccessLog` | Gateway → CP | Batched access log entries |

### Signed Envelope

Policy bundles, trust bundles, revocation events, and rotation commands are wrapped in a signed envelope before transmission:

```protobuf
message PolicyBundle {
  int64 last_sequence   = 1;  // bundle protocol version
  int64 issued_at = 2;  // epoch ms
  // Delta or full snapshot — gateway unpacks these one by one
  repeated PolicyRecord records = 3;

  // Signs: H(version || issued_at || H(record[0]) || H(record[1]) || ...)
  // where H(record[i]) is the hash of the *serialized* PolicyRecord bytes
  bytes signature = 4;
}

```
---

## Gateway → CP Control Stream 

### Connection Establishment

```
Gateway                                   Control Plane
    │                                          │
    │── gRPC Dial (mTLS) ────────────────────► │
    │   Gateway presents identity cert         │
    │   CP verifies cert (trust bundle + CRL)  │
    │                                          │
    │── Hello{gateway_id, policy_version} ───► │
    │                                          │
    │◄── PolicyDelta or PolicySnapshot ─────── │
    │                                          │
    │   [Bidirectional stream open]            │
```


## Stream Termination and Reconnection

| Scenario | Behaviour |
|---|---|
| CP closes stream (graceful) | Gateway receives `io.EOF`, begins reconnection with backoff |
| CP closes stream (error) | Gateway receives gRPC status error, logs, begins reconnection |
| Network interruption | Keepalive timeout fires, stream detected as dead, reconnection begins |
| Reconnection successful | Gateway sends `Hello` with current policy version, receives delta |
| Reconnection during POLICY_STALE | Gateway is in fail-closed state; continues reconnect attempts; applies bundle on success |

---

## Connector → Gateway Management Stream

The Connector also maintains a persistent gRPC/mTLS stream to the Gateway. The protocol mirrors the Gateway-to-CP stream at a smaller scope:

- Gateway relays Connector revocations or rotate cert commands to connectors.
- Connectors heartbeat to the Gateway (not directly to the CP).
- The gateway aggregates connector health and reports it to the CP in its own heartbeat.


*Next: [Connection Lifecycle →](./connection-lifecycle.md)*
