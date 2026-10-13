# Connector

## Role

The Connector is a **lightweight outbound-only tunnel agent** deployed inside the customer network, adjacent to the protected applications. It is the bridge between the public-facing gateway and the private applications.

The connector has **no inbound listening ports**. All connectivity is initiated outbound, which means it works behind NAT, corporate firewalls, and in private cloud environments without any inbound firewall rule changes.

---


## Dual-Plane Design

The connector maintains two concurrent connections to the gateway:

| Plane | Protocol | Purpose |
|---|---|---|
| **Management Plane** | mTLS gRPC (outbound) | Receives revocation events, rotation commands. Sends heartbeats, metrics, app health status. |
| **Data Plane** | mTLS QUIC (outbound) | Carries multiplexed application traffic streams (HTTP, TCP) from the gateway to internal applications. |

Both connections authenticate using the connector's identity certificate issued by the CP.

```
┌─────────────┐   mTLS gRPC (management)   ┌─────────────┐   HTTPS / TCP   ┌────────────┐
│   Gateway   │ ◄──────────────────────── ► │  Connector  │ ──────────────► │    App     │
│ (gRPC srv)  │                             │ (gRPC cli)  │                 │ (internal) │
│ (QUIC srv)  │ ◄──────────────────────── ► │ (QUIC cli)  │                 │            │
└─────────────┘   mTLS QUIC (data plane)   └─────────────┘                 └────────────┘
```

---

## Bootstrap Flow

A new connector has no certificate, no policy, and no knowledge of which gateway to connect to. Bootstrap resolves all three.

```
1. Operator generates bootstrap token in CP dashboard (single-use, 5-min TTL, bound to connector_id)

2. Connector binary starts with token

3. Connector generates ECDSA P-256 keypair locally

4. Connector generates CSR:
   Subject: CN=cn-prod-db-001, O=ashrix, OU=connectors
   SAN:     DNS:cn-prod-db-001

5. Connector POSTs to CP break-glass endpoint (public-CA-validated HTTPS):
   POST /v1/bootstrap
   {
     "bootstrap_token": "<token>",
     "connector_id":    "cn-prod-db-001",
     "csr":             "<base64-csr>",
     "timestamp":       <unix>
   }

6. CP validates:
   - Token exists, not consumed, not expired, bound to cn-prod-db-001
   - CSR contains valid EC P-256 public key
   - Timestamp within ±5 minutes

7. CP atomically:
   - Marks token consumed (DB unique constraint — not application logic)
   - Issues identity certificate via Intermediate CA (90-day validity)
   - Returns:
       connector_id, identity cert, trust bundle [Root CA + Intermediate CA],
       gateway_id, gateway_address, gateway_ip, tenant_id,
       app_list: [{app_id, name, subdomain, upstream, protocol, is_public}]

8. Connector persists to disk (all AES-256-GCM encrypted):
   /var/lib/ashrix-connector/
     ├── key.enc          # Private key
     ├── cert.pem         # Identity certificate
     └── trust_bundle.pb  # Trusted CA certs

9. Connector makes http request to /status with these headers: X-Ashrix-Connector-ID,X-Ashrix-Timestamp,X-Ashrix-Signature,X-Ashrix-Nonce to get the connector info

message ConnectorStatusResponse {
  string                 connector_id          = 1;
  string                 gateway_id            = 2;
  string                 gateway_url           = 3;
  string                 gateway_ip            = 4;
  string                 grpc_port             = 5;
  string                 https_port            = 6;
  string                 quic_port             = 7;
  string                 tenant_id             = 8;
  bool                   open_sock             = 9;
  repeated ConnectorApps apps                  = 10;
  string                 secondary_gateway_id  = 11;
  string                 secondary_gateway_url = 12;
  string                 secondary_gateway_ip  = 13;
  string                 secondary_grpc_port   = 14;
  string                 secondary_https_port  = 15;
  string                 secondary_quic_port   = 16;
  repeated string        crl_entries           = 17;
}


9. Connector establishes mTLS gRPC management stream to Gateway
   - Gateway verifies connector's cert against trust bundle + in-memory CRL
   - Gateway checks connector_id is authorised for this gateway

10. Connector establishes mTLS QUIC connection to Gateway
    - Same certificate, same verification
    - QUIC 0-RTT disabled (prevents replay of early data)
    - ALPN: "ashrix-quic-v1"
```

---

## App List and Local Proxy

The app list received during bootstrap (and updated via management stream) defines which internal applications this connector is authorised to proxy. It is **authoritative** — the connector only forwards traffic to applications in this list.

### App List Schema

```json
[
  {
    "app_id":            "app-jenkins-prod",
    "internal_host":     "jenkins.internal",
    "internal_port":     8080,
    "protocol":          "http",
    "health_check_path": "/health"
  },
  {
    "app_id":        "app-postgres-prod",
    "internal_host": "postgres.internal",
    "internal_port": 5432,
    "protocol":      "tcp"
  }
]
```

### Proxy Behaviour

On receiving a new QUIC stream from the gateway:

1. Extract `app_id` from stream metadata.
2. Check if `app_id` is in the local app list. If not → reject stream with error.
3. Dial `internal_host:internal_port` over the local private network.
4. Proxy bidirectionally between the QUIC stream and the internal TCP connection.


---

## Management Stream (gRPC)

### Connector → Gateway

| Message | Frequency | Content |
|---|---|---|
| Heartbeat | Every 15s | uptime, active_streams, app health status |
| AccessLog | Batched | Proxied requests: app_id, user_id, bytes, duration, result |

---

## Certificate Renewal

### Trigger
- Day 60 of a 90-day certificate (automated, proactive).
- CP-signed `RotationCommand` received via management stream.

### Flow

```
1. Connector detects cert within renewal window

2. Generates new EC P-256 keypair

3. Creates CSR with same connector_id

4. Generates possession proof:
   nonce     = 32-byte CSPRNG
   timestamp = current_unix_time
   proof     = ECDSA_SIGN(SHA256(csr || connector_id || timestamp), current_private_key)

5. POSTs directly to CP break-glass endpoint (bypasses gateway):
   POST /v1/renew
   {
     "connector_id": "cn-prod-db-001",
     "csr":          "<base64-csr>",
     "timestamp":    <unix>,
     "signature":    "<base64-proof>"
   }

6. CP verifies:
   - connector_id registered
   - timestamp within ±5 minutes
   - proof signature valid against current certificate's public key

7. CP issues new certificate via Intermediate CA

8. Connector receives:
   - New certificate
   - Updated trust bundle (if Intermediate CA rotation in progress)
   - Updated app list (if changed)

9. Connector hot-reloads:
   - Writes new cert to disk (atomic rename, no window of inconsistency)
   - Loads new cert into TLS config
   - Reconnects gRPC management stream with new cert
   - Reconnects QUIC data plane with new cert
   - Old connections drain gracefully
```

> **Security:** The possession proof binds the CSR, node identity, and a timestamp. A captured proof from a previous renewal cannot be replayed because the timestamp window (±5 minutes) will have elapsed.

> **Rule:** Connector renewals always contact the CP break-glass endpoint directly. The gateway is never in the certificate issuance path.

---

## Expired Certificate Recovery 

If the connector certificate has expired (missed the renewal window), the connector must not attempt to connect to the gateway — the gateway will reject the expired certificate at the TLS handshake level.

```
1. Connector starts, loads certificate from disk
2. Checks NotAfter → if expired:
   a. Do NOT attempt gateway connection
   b. Enter RECOVERY state

3. Connector contacts CP break-glass HTTPS directly:
   POST /v1/renew
   {
     "connector_id": "cn-prod-db-001",
     "csr":          "<base64-csr>",        // New keypair generated for recovery
     "timestamp":    <unix>,
     "signature":    "<base64-proof>"        // Signed with OLD private key (still on disk)
   }

4. CP verifies:
   - connector_id registered
   - timestamp within ±5 minutes
   - proof signature valid against LAST KNOWN public key in CP database
     (NOT against expired certificate — key possession is what matters)

5. CP issues replacement certificate

6. Connector persists new cert, returns to BOOTSTRAPPED state
7. Connects to gateway normally
```

---

## Connector State Machine

```
INIT
 │
 ▼
BOOTSTRAPPING ─────────────────────────────┐
 │                                         │
 ▼                                         │
ACTIVE (gRPC + QUIC established)           │
 │                                         │
 ├──► RENEWING (day 60, new CSR to CP)     │
 │      │                                  │
 │      ▼                                  │
 │   ACTIVE (hot-reload new cert)          │
 │                                         │
 ├                                         |
 │                                         │
 ├──► ROTATE CERT                          │
 │      │                                  │
 │      ├── denies new access requests     │
 │      ├── in-flight sessions continue    │
 │      ├── emits stale-policy alert       │
 │      │                                  │
 │      ▼                                  │
 │   RENEW CERT (break-glass to CP)   ─────│
 │                                         
 ├──► REVOKED (revocation event received)  
 │      │                                  
 │      ▼                                  
 │   SHUTDOWN CONNECTOR
 │      
 │
 └──► EXPIRED_ON_STARTUP
        │
        ▼
     RECOVERING (break-glass to CP)
        │
        ▼
     BOOTSTRAPPING
```

---

## Failure Modes

| Scenario | Connector Behaviour |
|---|---|
| Gateway unreachable | Retry gRPC and QUIC with jittered backoff. Serve no traffic. App list remains cached. |
| Gateway reachable but cert rejected | If cert expired → recovery flow. If not expired → log anomaly, alert, retry. |
| CP break-glass unreachable | Exponential backoff to CP. If cert not yet expired, continue serving via existing gateway connection. |
| App list desync | Use locally cached app list. New streams for unknown apps are rejected. CP pushes updates via management stream when connectivity restores. |
| QUIC connection dies, gRPC stays up | Re-establish QUIC only. gRPC management stream remains for control. |
| gRPC dies, QUIC stays up | Mark QUIC as draining. Re-establish gRPC. If gRPC fails for >60s, close QUIC and retry both. |

---

## Security Properties

| Control | Implementation |
|---|---|
| **No inbound listen** | Connector initiates all connections. No listening ports exposed to the network. |
| **Payload verification** | All management payloads verified with embedded CP bundle-signing key before parsing. |
| **App list enforcement** | Only proxies to hosts/ports in the local app list. Unknown `app_id`s are rejected. |
| **Disk encryption** | All persisted files encrypted with AES-256-GCM. |
| **Break-glass TLS** | Always public-CA-validated HTTPS. Never mTLS for recovery (cert may be expired or missing). |
| **0-RTT disabled** | Prevents replay of early QUIC data. |

---

## Security Boundary Warning

An outbound connector does **not** automatically make the protected application environment secure. If an attacker compromises the connector host:

- They can reach any application in the connector's app list.
- They can intercept or modify traffic being proxied.
- They can use the connector's network position to reach other hosts accessible from that network.

**Mitigations:**

1. The connector host should be a dedicated VM or container — not a shared host.
2. Host-level firewall rules should allow the connector to reach only the specific application hosts and ports in its app list.
3. The internal applications should be bound to accept connections only from the connector host's IP or loopback address.
4. The connector binary should run as a non-root user with minimal filesystem permissions.

These are operational responsibilities of the operator.

---

*Next: [Identity and Access →](./identity-and-access.md)*
