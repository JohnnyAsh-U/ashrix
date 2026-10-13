# QUIC Transport

## Why QUIC for the Connector-to-Gateway Tunnel

Ashrix uses QUIC for the **data-plane tunnel** between the connector and the gateway. This carries multiplexed application traffic (HTTP, TCP) from the gateway to internal applications.

The key properties QUIC provides that made it the right choice:

| Property | Benefit for Ashrix |
|---|---|
| **Built-in TLS 1.3** | No separate TLS negotiation layer; authentication and encryption are integral |
| **Stream multiplexing** | Multiple concurrent application sessions share one connection without head-of-line blocking |
| **Connection IDs** | The connection survives IP address changes (e.g., connector host getting a new IP after NAT rebinding) |
| **UDP-based** | Works through many corporate firewalls on port 443 alongside HTTPS |
| **0-RTT support** | Available but **disabled** in Ashrix (see Security section) |

See [ADR-002 — QUIC Transport](../10-engineering-decisions/adr-002-quic-transport.md) for the full decision record comparing QUIC with TLS-over-TCP alternatives.

---

## Connection Establishment ✅ Implemented

The Connector establishes the QUIC connection **outbound** to the Gateway. The Gateway listens on port 443 (UDP).

### Handshake

```
Connector                                  Gateway
    │                                          │
    │── QUIC Initial (ClientHello TLS 1.3) ──►│
    │                                          │  Gateway presents identity cert
    │                                          │  (issued by Ashrix Intermediate CA)
    │◄── QUIC Handshake (ServerHello) ─────────│
    │                                          │
    │── Connector presents identity cert ─────►│
    │   (mTLS — Gateway verifies against       │
    │    trust bundle + in-memory CRL)          │
    │                                          │
    │◄── Handshake Complete ───────────────────│
    │                                          │
    │   ALPN: "ashrix-quic-v1"                  │
    │   Connection established (mTLS QUIC)     │
```

### Key Parameters

| Parameter | Value | Rationale |
|---|---|---|
| ALPN | `ashrix-quic-v1` | Identifies the Ashrix application protocol version |
| 0-RTT | **Disabled** | Prevents replay attacks on early data |
| Connection migration | **Enabled** | Allows connector to survive NAT rebinding without reconnecting |

---

## Stream Multiplexing

A single mTLS QUIC connection from a connector to a gateway carries multiple independent application sessions as **bidirectional streams**.

```
QUIC Connection (mTLS)
  │
  ├── Stream 0: HTTP CONNECT tunnel → app-jenkins-prod (User A)
  ├── Stream 1: HTTP CONNECT tunnel → app-jenkins-prod (User B)
  ├── Stream 2: Raw TCP tunnel      → app-postgres-prod (User C)
  ├── Stream 3: HTTP CONNECT tunnel → app-dashboard (User A, second tab)
  └── ...
```

### Why This Matters

- **No head-of-line blocking:** A slow response from `app-postgres` does not delay packets for `app-jenkins` streams. Each QUIC stream is independently flow-controlled.
- **Efficient resource use:** One TLS handshake, one connection, many sessions.
- **Failure scoping:** If a single stream fails, it does not terminate the QUIC connection or affect other streams.

### Stream Lifecycle

```
Gateway opens new stream for inbound user request:
  1. Gateway assigns new stream ID
  2. Gateway sends stream metadata:
       :authority        = app-jenkins-prod.internal
       x-ashrix-session-id = <opaque>
       x-ashrix-user-id    = u-123
       x-ashrix-groups     = eng,oncall
  3. Connector verifies app_id exists in local app list
  4. Connector dials internal_host:internal_port
  5. Bidirectional proxy begins

Stream termination (normal):
  1. User closes browser connection → Gateway sends STREAM_FIN
  2. Connector receives FIN → closes TCP connection to internal app
  3. Stream closes gracefully

Stream termination (error):
  1. App connection error → Connector sends RESET_STREAM
  2. Gateway receives reset → propagates error to user (HTTP 502/503)
```

---

## Connection Age Jitter

A fixed max connection age of 10 minutes causes all connectors that started simultaneously (e.g., after a gateway restart) to reconnect at the same moment — a **thundering herd** of concurrent TLS handshakes.

Ashrix spreads this load using jitter:

```
max_connection_age = base_age + random_jitter
base_age           = 10 minutes
random_jitter      = random(0, 2 minutes)
effective_range    = 10–12 minutes per connection
```

Each connector draws its jitter value independently at connection establishment. The result is that reconnection load is spread across a 2-minute window regardless of how many connectors started simultaneously.

---

## Connection vs. Stream Failures

| Failure Type | Scope | Recovery |
|---|---|---|
| **QUIC stream reset** | Single application session | Gateway returns 502 to user; QUIC connection survives |
| **QUIC connection closed** | All streams on that connection | Connector reconnects with jittered backoff; gateway marks connector offline until reconnected |
| **QUIC connection lost (network)** | All streams on that connection | QUIC detects via keepalive timeout; connector reconnects |
| **TLS handshake failure** | No connection established | Connector logs error, backs off, retries |

---

## Security Properties

| Property | Implementation |
|---|---|
| Encryption | TLS 1.3 (integral to QUIC). |
| Peer authentication | mTLS — both connector and gateway present identity certificates. |
| Certificate verification | Gateway verifies connector cert against trust bundle + in-memory CRL. Connector verifies gateway cert against trust bundle. |
| Replay prevention | 0-RTT disabled. TLS 1.3 session tickets include anti-replay mechanisms. |

---

## Operational Considerations

### UDP Filtering

Some enterprise environments block UDP, including port 443. QUIC requires UDP. If a connector cannot establish a QUIC connection due to UDP filtering:

- The connector's data-plane connection fails.
- The management gRPC stream (TCP-based) may still succeed.
- Application traffic cannot be proxied until QUIC connectivity is restored.

---

*Next: [gRPC Control Channel →](./grpc-control-channel.md)*
