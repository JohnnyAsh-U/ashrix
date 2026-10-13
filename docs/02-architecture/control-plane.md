# Control Plane

## Role and Responsibility

The Control Plane (CP) is the **management plane** and **identity plane** of Ashrix.

The CP is the **sole authority** for:

- Issuing and revoking gateway and connector identity certificates.
- Signing all distributed policy bundles, trust bundles, and revocation events.
- Authenticating users against tenant Identity Providers.
- Tracking active sessions across all tenant gateways.
- Distributing configuration and commands to registered gateways.

The CP is **never** in the hot path for application traffic. It does not participate in per-request authorisation; that is done locally by the gateway against its cached, CP-signed policy.


## Gateway Registration Flow

A gateway must register with the CP before it can serve traffic. Registration uses an enrollment token (bootstrap token) issued by a CP admin.

```
1. Admin generates bootstrap token in CP dashboard
   → token = 6 chars
   → stored as hash(token) in DB, bound to gateway_id
   → token displayed ONCE to admin, plaintext never persisted

2. Gateway binary starts with token

3. Gateway generates ECDSA P-256 keypair locally

4. Gateway POST /v1/bootstrap over public-CA-validated HTTPS:
   {
     "bootstrap_token": "<token>",
     "gateway_id":      "gw-us-acme-001",
     "csr":             "<base64-csr>",
     "timestamp":       <unix>
   }

5. CP validates:
   - Token exists, unused, not expired (<5 min), bound to gateway_id
   - CSR contains valid EC P-256 public key
   - Timestamp within ±5 minutes

6. CP atomically:
   - Marks token as consumed (DB-level unique constraint)
   - Issues identity certificate via Intermediate CA (90-day validity)
   - Returns: identity cert, trust bundle, gateway address

7. Gateway persists to disk (AES-256-GCM encrypted):
   /var/lib/ashrix-gateway/
     ├── key.enc           # Private key
     ├── cert.pem          # Identity certificate
     ├── trust_bundle.pb   # Trusted CA certs
     └── config.pb         # Tenant configuration

8. Gateway establishes persistent gRPC/mTLS Control Stream to CP
```

> **Security note:** The bootstrap token is the highest-value credential in the system. Its delivery is out-of-band (copy-paste).

---

## Control Stream (Bidirectional gRPC/mTLS)

Once registered, the gateway maintains a persistent, bidirectional gRPC stream to the CP. This is the channel through which all management communication flows after bootstrap.

### CP → Gateway (pushed)

| Message | Purpose |
|---|---|
| `PolicyDelta` / `PolicySnapshot` | Incremental or full policy bundle update |
| `TrustBundle` | Updated CA certificate set (e.g., during Intermediate CA rotation) |
| `CRLUpdate` | Updated Certificate Revocation List |
| `RevocationEvent` | Session or node certificate revocation command |
| `RotationCommand` | Instruction to initiate certificate renewal |


### Gateway → CP (pushed)

| Message | Purpose |
|---|---|
| `Heartbeat` | Gateway alive signal; includes policy version, cert expiry, state, metrics |
| `CommandACK/PolicyACK` | Acknowledgement of received policy delta or command |
| `AccessLog` | Batched structured access log entries |

---

## Policy Distribution 

### Policy as a Versioned Key-Value Store

Every tenant's policy is stored as a collection of independently versioned access rules. The collection as a whole has a monotonic global version number.

### Delta Computation

When a gateway connects (or reconnects), it announces its current policy version in the `Hello` message:

```protobuf
message HelloMessage {
  string gateway_id     = 1;  // stable ID — never changes
  string tenant_id      = 2;
  string binary_version = 3;  // Ashrix binary version
  int64  policy_version = 4;  // policy bundle version currently applied
  string quic_port      = 5;
  string https_port     = 6;
  string grpc_port      = 7;
  int64  uptime         = 8;
}
```

The CP queries mutations since that version and sends only what has changed:


### Signed Payload Wrapper

All bundles (delta or full snapshot) are wrapped in a CP-signed envelope before transmission. Gateways verify:

1. ECDSA signature — verified against the embedded bundle-signing public key.
2. `version > current_version` — prevents downgrade attacks.


The gateway only parses the bundle content after all four checks pass.

---

## Session Registry

The CP maintains a `user_sessions` table tracking active sessions across all tenant gateways. This enables:

- **Targeted revocation**: the CP knows which gateways hold a given user's sessions.
- **Admin dashboards**: admins can see and kill active sessions.

---

## CP Unavailability Behaviour

When the CP becomes unreachable:

| Condition | Gateway Behaviour |
|---|---|
| CP unreachable, policy fresh | Serve traffic using cached policy and local Redis sessions while retrying the CP control stream. |
| CP control stream reconnects | Reset the offline grace-period timer. |
| CP unreachable, MAX_OFFLINE_PERIOD exceeded | Gateway shuts down and closes its listeners. |
| CP restarts | Up to N gateways reconnect with **jittered exponential backoff** (base 1s, max 60s) to avoid thundering herd. |

> **Availability vs. security trade-off:** Allowing cached-policy access during CP outages improves availability but means a revocation issued during the outage will not take effect until the CP reconnects and pushes an update. The maximum acceptable staleness before shut down is defined by `MAX_OFFLINE_PERIOD`.

---

## Multi-Tenancy

All CP data is strictly namespaced by `tenant_id`:

- Database tables include `tenant_id` as a mandatory column with index.
- Redis keys are prefixed: `session:<tenant_id>:<session_id>`.
- The CP filters all policy mutations, session queries, and gateway commands by `tenant_id` before sending to a specific gateway.

---

*Next: [Gateway →](./gateway.md)*
