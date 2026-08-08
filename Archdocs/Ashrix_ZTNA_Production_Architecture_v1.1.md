# ASHRIX ZTNA — Production Architecture

### Version 1.1 — August 2026


## 1. Executive Summary

Ashrix is a Zero Trust Network Access (ZTNA) platform that provides secure access to applications across distributed environments. The platform is architected around a **centralized Control Plane (CP)** and **decentralized, tenant-dedicated Data Planes**.

Key architectural decisions:

- **Tenant-dedicated gateways**: Each tenant operates their own gateway(s) rather than sharing a multi-tenant proxy. This provides complete traffic isolation, simplifies compliance, and removes noisy-neighbor risks.

- **Outbound-only connectivity**: All data plane components (gateways and connectors) initiate outbound connections to the CP. No inbound firewall rules are required.

- **Two-tier offering**: `Ashrix\_public` for individual users (managed by Ashrix) and Enterprise tenants for organizations (self-hosted or Ashrix-managed).

- **Multi-region support**: Enterprise tenants may deploy multiple regional gateways under a single tenant identity.


## 2. System Overview

Ashrix is a SaaS Zero Trust Network Access (ZTNA) platform with three runtime tiers. This document harmonizes the PKI/certificate lifecycle architecture with session management, policy distribution, and gateway enforcement.

### 2.1 Design Principles

| Principle | Description |
| - | - |
| **Zero Trust** | No implicit trust based on network location. Every access request is authenticated, authorized, and encrypted. |
| **Tenant Isolation** | Traffic, sessions, and configuration are isolated per tenant. Ashrix never decrypts enterprise tenant traffic. |
| **Outbound-Only** | All data plane components dial out to the CP. Works behind NAT, corporate firewalls, and in private clouds without inbound ports. |
| **Regional Affinity** | Gateways are deployed close to applications to minimize latency and backhauling. |
| **CP as Source of Truth** | The Control Plane manages identity, policies, app catalogs, and session registries. Gateways are stateful but simple. |
| **Failover by Design** | Gateways support active-passive and active-active HA within a region. |



### 2.2 High Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐  
│                           ASHRIX CONTROL PLANE                               │  
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │  
│  │   Identity  │  │   Policy    │  │  App Catalog  │  │ Session Registry   │ │  
│  │   / SSO     │  │   Engine    │  │               │  │ (per-tenant DB)    │ │  
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │  
│                              gRPC / mTLS (outbound)                         │  
└──────────────────────────────┬──────────────────────────────────────────────┘  
                               │  
         ┌─────────────────────┼─────────────────────┐  
         │                     │                     │  
    ┌────▼────┐          ┌────▼────┐          ┌────▼────┐  
    │ GW-US   │          │ GW-Asia │          │ GW-Afr  │  
    │(tenant) │          │(tenant) │          │(tenant) │  
    │+HA Pair │          │+HA Pair │          │+HA Pair │  
    └────┬────┘          └────┬────┘          └────┬────┘  
         │                    │                    │  
    ┌────▼────┐          ┌────▼────┐          ┌────▼────┐  
    │Connector│          │Connector│          │Connector│  
    │ (US)    │          │ (Asia)  │          │(Africa) │  
    └────┬────┘          └────┬────┘          └────┬────┘  
    \[US Apps\]              \[Asia Apps\]          \[Africa Apps\]
```

### Traffic Flow

1. **Gateway → CP**: Persistent gRPC/mTLS stream for registration, config sync, heartbeats, audit logs, and command reception.

2. **Connector → Gateway**: QUIC connection (outbound from connector perspective). The connector dials the gateway.

3. **User → Gateway**: HTTPS or QUIC. The user reaches the gateway directly (public IP) or via a relay.

4. **Gateway → App**: Proxied through the connector tunnel. The gateway never communicates directly with the CP for application traffic.



```
┌─────────────────────────────────────────────────────────────────────────────┐  
│                              INTERNET / USER                                │  
└─────────────────────────────────────────────────────────────────────────────┘  
 │ HTTPS (Public TLS)                          │ mTLS Break-Glass  
 ▼                                             ▼  
┌─────────────────┐      mTLS gRPC ControlStream      ┌─────────────────┐  
│     Gateway     │ ◄────────────────────────────────►│  Control Plane  │  
│  (Per-Tenant   	│                                   │   (SaaS Core)   │  
│                 │                                   │  • Identity       │  
│  ┌───────────┐  │      				       │  • Policy Authz   │  
│  │  Chi HTTP │  │ 					       │  
│  │  Router   │  │                                   │  • Session Ticket │  
│  └───────────┘  │                                   │  • Revocation     │  
└─────────────────┘                                   └─────────────────┘  
 │  
 │ Control Stream (via mTLS Tunnel) & QUIC Tunnel for App Proxy  
 ▼  
┌─────────────────┐  
│    Connector    │  
│  (App-Side Agent)│  
└─────────────────┘
```

| Tier | Protocols | Role |
| - | - | - |
| **Control Plane (CP)** | gRPC (mTLS), HTTPS (OIDC), HTTPS (break-glass) | Source of truth for identity, PKI, policy authoring, revocation |
| **Gateway** | HTTPS (public browser), gRPC (mTLS to CP), gRPC (mTLS to Connector) | Policy enforcement point, session termination, traffic broker |
| **Connector** | gRPC (mTLS to Gateway), QUIC Connection for App Proxy(mTLS), HTTPS (break-glass to CP) | Application-side tunnel agent, workload access endpoint |









## 2.3 Control Plane

The CP is the **management plane** and **identity plane**. It is a SaaS service operated by Ashrix.

### 2.3.1 Responsibilities

| Component | Responsibility |
| - | - |
| **Identity & SSO** | User authentication via SAML 2.0, OIDC, or Ashrix-managed identity. Issues short-lived short lived tokens for gateway to create sessions. |
| **Tenant Management** | Tenant provisioning, billing, feature entitlements, and lifecycle. |
| **App Catalog** | Registry of applications per tenant, including app metadata, access policies, and gateway routing targets. |
| **Policy Engine** | Evaluates access policies (user group, device posture, MFA, time-based, location-based). |
| **Session Registry** | Central database tracking active user sessions across all tenant gateways. Source of truth for revocation. |
| **Gateway Command & Control** | Pushes configuration, receives heartbeats, and sends commands (e.g., session revocation) to gateways via gRPC. |
| **Audit & Analytics** | Collects authentication events, policy decisions, and gateway health metrics. |


### 2.3.2 Session Registry Schema

The CP maintains a `user\_sessions` table to track active sessions across all tenant gateways.

**Why the CP keeps this registry:**

- The CP must answer "What are Alice's active sessions?" without querying live gateways.

- Enables targeted revocation: the CP knows exactly which gateways hold Alice's sessions.

- Supports admin dashboards and compliance reporting.

**Sync protocol:**

- Gateway reports `SessionCreated`, `SessionRefreshed`, and `SessionExpired` events to the CP via the persistent gRPC stream.

- CP tolerates drift: if a gateway reports a session that the CP does not know about, the CP ingests it. If the CP tries to revoke a session the gateway has already expired, the gateway returns `NOT\_FOUND` and the CP marks it as expired.

## 2.4 Data Plane: Gateway

The gateway is a **stateful proxy** that terminates user connections and forwards them to applications through connector tunnels.

### 2.4.1 Responsibilities

| Component | Responsibility |
| - | - |
| **User Connection Termination** | Accepts HTTPS/QUIC connections from users. Validates Sessions signed by the CP. |
| **Session Store** | Local Redis (or in-memory) for active session state, connection mappings, and rate limiting. |
| **Connector Registry** | Tracks which connectors in its region are online and which apps they expose. |
| **Traffic Proxy** | Routes authenticated user requests to the correct connector tunnel and onward to the target application. |
| **gRPC Client** | Maintains a persistent outbound gRPC/mTLS connection to the CP for commands and config. |
| **Audit Logging** | Streams access logs to the CP (user, app, timestamp, action, result). |


### 2.4.2 Gateway Startup Flow

```
1. Gateway binary starts with enrollment token.  
2. Gateway resolves CP endpoint and initiates gRPC/mTLS handshake.  
3. Gateway authenticates using enrollment token.  
4. CP validates token, registers gateway, and pushes tenant config.  
5. Gateway opens local listener for user connections.  
6. Gateway begins accepting connector registrations over QUIC.
```

### 2.4.3 High Availability

Each enterprise tenant may deploy gateways in an **active-passive** or **active-active** configuration within a region.

**Active-Passive:**

- Two gateway instances share a virtual IP or DNS record.

- Connectors are configured with both gateway addresses.

- If the active fails, connectors reconnect to the passive.

- The CP updates the session registry to reflect the new active node.


**Active-Active:**

- Multiple gateway instances behind a load balancer.

- Shared Redis or session replication ensures session continuity.

- The CP tracks the cluster ID rather than individual nodes for revocation.


## 2.5 Data Plane: Connector

The connector is a lightweight agent deployed inside the tenant network, close to the applications.

### 2.5.1 Responsibilities

- **Outbound QUIC**: Opens a persistent QUIC connection to its assigned regional gateway.

- **App Exposure**: Registers local applications (IP, port, protocol) with the gateway.

- **Tunnel Proxy**: Forwards traffic from the gateway to the local application.

- **Heartbeat**: Maintains connection health and reconnects on failure.



## 2. Trust Model & PKI Hierarchy

### 2.1 Core Principle

> **Control Plane creates trust. Gateway distributes trust. Connector verifies trust.**

### 2.2 PKI Hierarchy

```
Root CA (Offline — Cloud KMS / HSM)  
    │  
    ▼  
Intermediate CA (Online — HashiCorp Vault PKI)  
    │  
    ├── Gateway Identity Certificates (mTLS)  
    ├── Connector Identity Certificates (mTLS)  
    └── mTLS Client Certificates (break-glass)
```

| Key | Purpose | Storage |
| - | - | - |
| **Root CA Private Key** | Sign Intermediate CA only | Cloud KMS / HSM. Never exported. |
| **Intermediate CA Key** | Sign node identity certs | Vault PKI engine. Never leaves Vault. |
| **Policy Signing Key** | Sign all policy/trust/revocation bundles, rotation commands. | Vault / KMS. Loaded into CP memory at boot.  |
| **Gateway Private Key** | mTLS identity | AES-256-GCM encrypted on disk. Decrypted to `crypto.Signer` at boot. |
| **Connector Private Key** | mTLS identity | Same as Gateway. |


### 2.3 Embedded Signing Public Key

Every Gateway and Connector binary embeds the Bundle Signing public key at compile time via `go:embed`.

**Verified at runtime:**

- Policy bundles (from CP )

- Trust bundles

- Revocation events

- Rotation commands

> **Security:** A compromised Gateway cannot forge CP commands. 



## 3. Component Identity & Bootstrap

### 3.1 Stable Identifiers

| Field | Format | Stability |
| - | - | - |
| `gateway\_id` | `gw-\<region\>-\<tenant\>-\<seq\>` | Never changes across cert rotations |
| `connector\_id` | `cn-\<app\>-\<tenant\>-\<seq\>` | Never changes across cert rotations |


Certificate rotations replace the certificate wrapper, not the keypair or the stable ID.

### 3.2 Bootstrap Flow

**Gateway:**

1. Generate ECDSA P-256 keypair locally

2. Generate CSR containing `gateway\_id`

3. POST Bootstrap Token + CSR to CP over **public-CA-validated HTTPS**

4. CP validates token (single-use, 5-minute TTL, tied to `gateway\_id`)

5. CP issues: Gateway ID, Name, URL, Identity Cert + Trust Bundle + Config

6. Gateway persists encrypted private key, cert, trust bundle, config to disk

7. Gateway establishes persistent mTLS gRPC `ControlStream` to CP

**Connector:**

1. Generate EC P-256 keypair locally

2. Generate CSR containing `connector\_id`

3. POST Bootstrap Token + CSR to CP over public-CA-validated HTTPS

4. CP validates token

5. CP issues: Connector ID, Identity Cert + Trust Bundle

6. Connector send a request to get Status from CP, and gets Response of Gateway ID, Gateway URL, GatewayIP, TenantID and Apps(name, subdomain, upstream, protocol, ispublic)

7. Connector persists encrypted private key, cert, trust bundle, gateway address

8. Connector establishes persistent mTLS gRPC stream to Gateway

> **Security:** Bootstrap tokens are single-use, short-lived, and scoped to a specific node ID. Invalidated atomically after first use.

### 3.3 Bootstrap Token Security Requirements **\[NEW\]**

> **Security Note:** The bootstrap token is the highest-value attack target in the entire system. Every other security control assumes the connector is legitimate. Bootstrap is where legitimacy is established. Treat bootstrap token delivery with the same operational gravity as a private key.

Bootstrap tokens must satisfy all of the following:

| Requirement | Detail |
| - | - |
| Single-use | Enforced at the database level with a unique constraint — not application logic. The token is marked consumed atomically at the moment of first use. |
| Short TTL | Maximum 15 minutes from issuance. CP rejects tokens beyond this window regardless of use status. (ZTNA default: 5 minutes.) |
| Node-bound | Token is cryptographically tied to a specific `node\_id`. A token issued for `cn-prod-db-001` cannot bootstrap `cn-prod-db-002`. |
| IP-bound (V2) | Token is optionally bound to an expected IP range or AS number. CP rejects bootstrap requests originating outside the declared range. |
| Audit event | Every bootstrap token generation is a security event: logged with the generating user's identity, the target `node\_id`, the declared IP range, and the expiry time. Every bootstrap token use (success or failure) is also logged and alertable. |





## 4. Control Plane Communication

### 4.1 Connection Topology

**Gateway → CP (Outbound Initiated)**

- Gateways are can be behind customer NAT/firewalls, but must have a public IP address, for redirection from the CP during authentication. CP cannot dial inbound.

- Outbound 443/TCP is universally allowed.

- CP sits behind an L4 (TCP) load balancer supporting HTTP/2.

**Connector → Gateway (Outbound Initiated)**

- Same principle. Connector is inside the private network.

- Gateway address is learned at bootstrap and persisted to disk.

### 4.2 Bidirectional gRPC Stream

**Inside the stream:**

- CP pushes: Policy deltas/snapshots, trust bundles, CRL updates, rotation/revocation commands, config updates.

- Gateway pushes: Heartbeats, bundle ACKs, access logs, metrics.

### 4.3 gRPC Keepalive & Timeouts

**Gateway (client):** Time: 20seconds; Timeout:5seconds; PermitWithoutStream: false

**CP (server enforcement):** MinTime: 10 seconds; PermitWithoutStream: false

**Max connection age:** 10 minutes. Forces periodic re-handshake to pick up cert rotations and CRL updates.

### 4.4 Reconnect Resilience

BaseDelay : 1 Second; MaxDelay: 60 Seconds

**Why:** Prevents 100 gateways from thundering-herding a restarting CP.

### 4.5 Per-Call Revocation Check

The Gateway loads the CRL into memory from CP. On **every** gRPC call from a Connector, the Gateway checks the presented certificate's serial number against the in-memory CRL. This ensures emergency revocations take effect immediately on active long-lived connections without waiting for reconnection.

### 4.6 Connection Age Jitter

A fixed max connection age of 10 minutes causes all connectors that started simultaneously (for example, after a gateway restart) to reconnect at the same moment. On a system with many connectors, this creates a thundering herd of simultaneous TLS handshakes. TLS handshakes are CPU-intensive (EC key operations). A burst of concurrent handshakes degrades gateway response time for all active traffic.

Max connection age is randomised at connection establishment:

```
max\_connection\_age = base\_age + random\_jitter  
base\_age           = 10 minutes  
random\_jitter      = random(0, 2 minutes)  
effective\_range    = 10–12 minutes per connection
```

Each connector draws its jitter value independently at the moment the connection is established. The result is that reconnection load is spread across a 2-minute window rather than concentrated at a single point, regardless of how many connectors started at the same time.


## 5. User Authentication & Session Management (HTTP Flow)

### 5.1 The Cross-Domain Problem

- CP authenticates users at `auth.ashrix.io`/api/v1/authorize/providers?app\_id&gateway\_id

- Gateway serves applications at `app-a.customer.com`

- Browsers do not share cookies across domains

**Solution:** CP issues a short-lived, single-use **session ticket**. The Gateway redeems it and establishes its own domain-scoped session.

### 5.2 Authentication Flow

1. User Tries to access App =\> [https://app.company.com](https://app.company.com/)

2. Gateway: No Session Cookie ? Creates state in the Redis(app url) and add to cookie then redirects to “[https ://auth.ashrix.io/authorize?app\_id](https://auth.ashrix.io/authorize?app_id)&gateway\_id”

3. CP Resolve IDP Providers related to the App and return the providers to User; 

4. User Select the IDP; CP Creates a state in the redis(gateway\_id) to track the auth process and authentication via  OIDC

4. User is redirected to CP Callback; CP exchanges token with IDP; and verifies the state in the redis to get the original gateway the request came from; Create Sessions in redis and redirect to gateway with a state in the url query

5. User is redirected to Gateway Callback([https://gateway.customer.com/\_auth/callback](https://gateway.customer.com/_auth/callback)) and  Exchanges the state with CP (via gRPC mTLS) to get the user info, and also verifies the initial state in the cookie to retrieve the original app ID; 

6. Create a session in the gateway redis and set cookies; and redirect to the original url(app url)


### 5.3 Ticket Security Constraints

| Property | Value | Rationale |
| - | - | - |
| **Format** | Opaque 32-byte base64url | Non-guessable, no embedded data |
| **TTL** | 60 seconds | Limits exposure window |
| **Usage** | Single-use, atomic GET+DEL | Prevents replay attacks |
| **Transmission** | Query parameter only | Immediately redeemed and stripped from URL |
| **Binding** | Tied to `redirect\_uri` | Prevents ticket theft to another gateway |


**Rate limiting:** `/\_auth/callback` is rate-limited per IP (10 req/min). Excess returns 429.

### 5.4 Session Cookie Properties

> **Security:** `\_\_Host-` prefix prevents subdomain cookie injection. No `Domain` attribute means cookie is host-locked to \*`.customer.com of gateway.`

### 5.5 Why Not JWT in the Cookie?

JWT sessions are stateless and cannot be revoked instantly. In ZTNA, revocation must be real-time (user termination, device compromise, policy change).

**Rule:** Use opaque tokens in Redis for browser sessions. 


## 6. Policy Distribution

### 6.1 Policy as a Versioned Key-Value Store

A policy bundle is a collection of independently versioned rules. The bundle itself has a monotonic global version.

### 6.2 Database Changelog (CP)

```
CREATE TABLE policy\_mutations (  
    version         BIGSERIAL PRIMARY KEY,  
    tenant\_id       VARCHAR(64) NOT NULL,  
    rule\_id         VARCHAR(255) NOT NULL,  
    op              VARCHAR(10) NOT NULL CHECK (op IN ('UPSERT', 'DELETE')),  
    rule\_snapshot   JSONB,  
    mutated\_at      TIMESTAMPTZ DEFAULT NOW()  
);  
  
CREATE INDEX idx\_mutations\_tenant\_version ON policy\_mutations(tenant\_id, version);
```

**Every policy write is atomic:**

```
BEGIN;  
UPDATE policies SET ... WHERE rule\_id = 'rule-002' AND tenant\_id = 't-123';  
INSERT INTO policy\_mutations (tenant\_id, rule\_id, op, rule\_snapshot)   
VALUES ('t-123', 'rule-002', 'UPSERT', '\{...\}');  
COMMIT;
```

### 6.3 Delta Computation

Gateway connects with `Hello \{ gateway\_id, current\_policy\_version: 847 \}`.

CP queries:

```
SELECT version, rule\_id, op, rule\_snapshot  
FROM policy\_mutations  
WHERE tenant\_id = ? AND version \> ?  
ORDER BY version;
```

CP collapses redundant mutations in-memory (if a rule was updated twice, only the final state is sent).

```
message PolicyDelta \{  
  uint64 from\_version = 1;  
  uint64 to\_version = 2;  
  
  message Mutation \{  
    enum Op \{ UPSERT = 0; DELETE = 1; \}  
    Op op = 1;  
    string rule\_id = 2;  
    AccessRule rule = 3;  
  \}  
  repeated Mutation mutations = 3;  
\}
```

### 6.4 Signed Payload Wrapper

All bundles (delta or full) are wrapped in a CP-signed envelope:

**Verification:**

1. Gateway/Connector checks `expires\_at`

2. Verifies ECDSA signature using embedded BundleSigning public key

3. Checks `version \> current\_version` (prevents downgrade attacks)

4. Checks `max\_valid\_until` deadline (prevents bundle withholding)

5. Only then parses payload content

### 6.5 Full Snapshot Format

When delta is impossible (new gateway, stale gateway, corruption):

### 6.6 Gateway Local Persistence

Gateway stores policies in **BBolt**  for zero-config durability:

```
/var/lib/ashrix-gateway/  
 ├── policy.db       \# BBolt
```



**Boot sequence:**

1. Load `policy.db` into memory 

2. Start Chi HTTP server immediately (serves traffic with cached policy)

3. Connect gRPC to CP asynchronously

4. Sync delta or snapshot in background

**Stale policy handling:**

- Not Yet Decided


## 7. Data Plane: Browser → Gateway → Connector

### 7.1 Gateway → Connector Proxy

The Gateway proxies authorized requests to the Connector over the existing mTLS QUIC stream.

**Identity injection headers (added by Gateway):**

```
X-Ashrix-User-ID: u-123  
X-Ashrix-Groups: eng,oncall  
X-Ashrix-Session-ID: \<opaque\>
```

**Connector verifies:**

1. Gateway's mTLS certificate is signed by Ashrix Intermediate CA

2. Certificate's `gateway\_id` matches expected value

3. Request carries valid identity headers

### 7.2 User Identity Architecture & Internal App Trust **\[NEW\]**

> **Scope:** This section covers how end-user identity is established, transported through the Ashrix tunnel, and delivered to internal applications in a trustworthy form. This is distinct from node identity (covered above), which governs how gateways and connectors authenticate to each other.

#### The Identity Problem

When a user accesses `app.customer.ashrix.io`, the gateway must:

1. Establish who the user is (authentication)

2. Determine what they are allowed to access (authorisation)

3. Transport that identity through the tunnel to the internal app

4. Ensure the internal app can trust the identity it receives

Without a defined identity architecture, the internal app either blindly trusts all incoming requests (no security) or must implement its own authentication stack (defeating the purpose of Ashrix).

#### Authentication Flow

```
User browser  
    │  
    │  HTTPS to app.customer.ashrix.io  
    ▼  
Gateway  
    │  
    ├── Is this request authenticated?  
    │     NO  → redirect to IdP (OIDC / SAML)  
    │     YES → validate session token  
    │  
    ├── Does policy allow this user to access this app?  
    │     NO  → 403 Forbidden  
    │     YES → proceed  
    │  
    ├── Inject identity headers (see below)  
    │  
    └── Forward request over tunnel to Connector → App
```

Authentication is handled by the gateway via OIDC or SAML integration with the tenant's identity provider (Okta, Azure AD, Google Workspace, etc.). The connector and internal app are never involved in user authentication.

#### Identity Headers

After authentication and authorisation, the gateway injects the following headers before forwarding the request over the tunnel:

| Header | Content | Example |
| - | - | - |
| `X-Ashrix-User-ID` | Stable unique user identifier from IdP | `usr\_a1b2c3d4` |
| `X-Ashrix-User-Email` | User's email address | `john@company.com` |
| `X-Ashrix-App-ID` | The Ashrix app identifier | `app-internal-dashboard` |
| `X-Ashrix-Request-ID` | Unique per-request trace ID | `req\_x9y8z7` |


#### Header Stripping

The connector strips all `X-Ashrix-\*` headers from requests before they reach the internal app, then re-injects only the verified set. This prevents a user from injecting their own `X-Ashrix-User-ID` header in the original request and having it pass through to the app.

```
Incoming request (from user):  
  X-Ashrix-User-ID: attacker-injected-value   ← STRIPPED by connector  
  
Re-injected by connector after JWT verification:  
  X-Ashrix-User-ID: verified-value-from-jwt   ← TRUSTED
```

#### Preventing Direct App Access

If a user is on the private network and can reach the internal app directly (port 8080), they bypass Ashrix entirely. The internal app must be configured to:

1. Only accept connections from the connector's loopback address (`127.0.0.1` or the connector host IP)

The connector's deployment guide must include firewall or binding configuration to enforce this. Ashrix's security guarantee only holds if the internal app is not reachable by any path other than through the connector.


## 8. Certificate Lifecycle

### 8.1 Validity Periods

| Certificate | Validity | Rotation Trigger |
| - | - | - |
| Root CA | 10–20 years | Compromise or algorithm deprecation only. Formal ceremony. |
| Intermediate CA | 1 year | Month 9–10. Human approval required. |
| Gateway / Connector | 90 days | Day 60. Fully automated. |
| mTLS Session | 24 hours | Every re-authentication / re-handshake. Automated. |


### 8.2 Possession Proof

Every renewal/recovery request must prove private key ownership:

```
possession\_proof = ECDSA\_SIGN(  
    SHA256(csr\_pem || node\_id || timestamp),  
    current\_private\_key  
)
```

CP verifies the signature against the public key in the currently issued certificate before issuing a replacement.

### 8.3 Renewal Flow

**Gateway:**

1. Day 60: Gateway detects expiry window

2. Generates new EC keypair

3. Creates CSR + possession proof

4. Contacts CP via break-glass https endpoint

5. CP verifies possession proof → issues new cert 

6. Gateway hot-reloads cert (no restart)

7. Reconnects mTLS with new cert

**Connector:**

1. Day 60: Connector detects expiry window

2. Generates new keypair, CSR, possession proof

3. Contacts CP **directly** via break-glass HTTPS endpoint (bypasses Gateway)

4. CP verifies → issues new cert

5. Connector hot-reloads and reconnects to Gateway

> **Rule:** Connector renewals always go directly to CP. The Gateway is never in the certificate issuance path.

### 8.4 Revocation

**CP is the sole revocation authority.**

**Gateway Revocation:**

1. CP revokes cert, updates CRL

2. CP pushes revocation event to Gateway via `ControlStream`

3. Gateway verifies CP signature

4. Gateway terminates mTLS session with CP

5. Gateway stops accepting Connector connections

6. Gateway initiates recovery flow directly with CP

**Connector Revocation:**

1. CP revokes cert, updates CRL

2. CP pushes CRL to Gateway via `ControlStream`

3. Gateway loads CRL into memory

4. Gateway's per-call interceptor detects revoked serial on next Connector request

5. Gateway terminates active Connector session immediately

6. Gateway relays CP-signed revocation event to Connector

7. Connector verifies BundleSign signature, enters REVOKED state

> **\[UPDATED\]** CRL and revocation events are **priority messages** on the CP→Gateway gRPC stream. They bypass the normal stream queue. The Gateway must process and load a new CRL within 2 seconds of receipt. Maximum revocation propagation SLA: **5 seconds** from CP decision to gateway enforcement. SLA compliance is monitored and alerted via `revocation\_issued\_at` (CP) and `crl\_loaded\_at` (Gateway heartbeat) timestamps.

### 8.5 Expired Certificate Recovery

> **Security Rule:** The Gateway never accepts expired certificates — not even for recovery.

1. Node detects expired cert on startup

2. Node does **NOT** attempt Gateway connection

3. Node contacts CP directly via break-glass HTTPS

4. Node sends: `node\_id` + new CSR + possession proof

5. CP validates key possession independently of cert expiry

6. CP issues replacement certificate

7. Node persists and connects normally

#### Break-Glass Endpoint Rate Limiting **\[NEW\]**

The break-glass endpoint is publicly reachable (the connector must reach it from behind NAT without going through the gateway). This makes it a target for denial-of-service attacks: an attacker who knows valid `node\_id` values can flood the endpoint with recovery requests. Each request fails (no valid possession proof) but forces CP to perform cryptographic verification, creating CPU exhaustion at scale.

Rate limiting controls applied at the break-glass endpoint:

| Control | Value |
| - | - |
| Max recovery attempts per `node\_id` per hour | 5 |
| Backoff on consecutive failures | Exponential: 1s, 2s, 4s, 8s, 16s |
| Alert threshold | 2 failed recovery attempts for any single `node\_id` within 10 minutes |
| IP-level rate limit | 20 requests per minute per source IP |


> **Alert Behaviour:** More than 2 failed recovery attempts for a single `node\_id` in a 10-minute window triggers a security alert to the tenant admin and the Ashrix operations team. This pattern indicates either a misconfigured node or an active enumeration attack.

### 8.6 CRL Distribution Priority **\[NEW\]**

The CP→Gateway gRPC stream carries multiple message types: heartbeats, log acknowledgements, bundle updates, and revocation events. Under load, message delivery is subject to stream backlog. A CRL push delayed behind routine heartbeats creates a window where revoked credentials remain active.

CRL and revocation events are **priority messages** on the CP→Gateway stream:

- Priority messages bypass the normal stream queue

- Gateway must process and load a new CRL within 2 seconds of receipt

- Maximum revocation propagation SLA: **5 seconds** from CP decision to gateway enforcement

- SLA compliance is monitored and alerted

> **SLA Monitoring:** The CP records a `revocation\_issued\_at` timestamp. The gateway records a `crl\_loaded\_at` timestamp and reports it in the next heartbeat. The CP calculates propagation latency and alerts if it exceeds 5 seconds.

### 8.7 Trust Bundle Rotation

During Intermediate CA rotation, the trust bundle contains both old and new intermediates simultaneously:

```
Trust Bundle v5:  
  - Root CA cert  
  - Intermediate CA v1 cert (retiring)  
  - Intermediate CA v2 cert (active)
```

**Sequence:**

1. v2 generated and signed by Root CA (offline ceremony)

2. CP pushes bundle `\[Root, v1, v2\]` to all nodes

3. All nodes verify CP signature and apply

4. Overlap window (60 days): all node certs reissued under v2

5. CP confirms every node has acknowledged v2

6. v1 removed from bundle, new bundle pushed

7. v1 expires

**Version ledger:** CP tracks which nodes have confirmed each bundle version. v1 removal is gated on 100% acknowledgment.


## 9. Security Controls

### 9.1 Rate Limiting

| Endpoint | Limit | Action |
| - | - | - |
| `/\_auth/callback` | 10 req/min per IP | 429 Too Many Requests |
| `/\_auth/callback` | 100 req/min globally | 429 (DDoS protection) |
| General API | 1000 req/min per IP | 429 |
| gRPC ControlStream | 1 concurrent stream per `gateway\_id` | Reject duplicate |
| Break-glass recovery | 5 req/hour per `node\_id` | 429 + alert |
| Break-glass recovery | 20 req/min per source IP | 429 |


### 9.2 CSRF Protection

The `state` parameter in the OIDC redirect is a 256-bit random value stored in a short-lived cookie (`\_\_Host-ashrix\_state`) before redirect to CP. On callback, Gateway validates that the returned `state` matches the cookie value.

### 9.3 Session Fixation Prevention

When a ticket is redeemed, the Gateway always generates a **new** `session\_id`. The ticket value is never used as or mapped to the session identifier.

### 9.4 Replay Protection

- **Tickets:** Redis `GETDEL` (atomic fetch and delete) or Lua script ensuring single-use

- **Bundles:** Monotonic version check (`version \> current`) prevents replay of old bundles

- **Possession proofs:** Timestamp must be within ±5 minutes of server time

### 9.5 Downgrade Attack Prevention

Nodes reject any bundle or delta with `version \<= current\_version`. This is checked **before** signature verification to avoid CPU waste, and **after** to ensure integrity.

### 9.6 Audit Logging

All events are emitted as structured JSON logs (or to an audit bus):

| Event | Fields |
| - | - |
| `auth.success` | user\_id, gateway\_id, app\_id, ip, user\_agent, mfa\_level, timestamp |
| `auth.failure` | user\_id, gateway\_id, reason, ip, timestamp |
| `session.created` | session\_id, user\_id, gateway\_id, app\_id, ttl |
| `session.revoked` | session\_id, reason (manual, policy, expiry), revoked\_by |
| `policy.applied` | gateway\_id, version, delta\_or\_snapshot, checksum |
| `cert.issued` | node\_id, node\_type, serial, expiry, issued\_by |
| `cert.revoked` | node\_id, serial, reason, revoked\_by |
| `bundle.rejected` | node\_id, version, reason (signature, downgrade, expiry) |
| `node.suspended` | node\_id, gateway\_id, issued\_by, timestamp |
| `node.resumed` | node\_id, gateway\_id, issued\_by, timestamp |
| `breakglass.attempt` | node\_id, ip, success, timestamp |



## 10. Node Lifecycle State Machine **\[NEW\]**

Gateway and Connector both follow the same state machine.

```
INIT  
  │  
  ▼  
BOOTSTRAPPING  (first contact with CP, token exchange, cert issuance)  
  │  
  ▼  
ACTIVE  (mTLS established, serving traffic)  
  │  
  ├──► EXPIRING  (cert within 30-day warning window)  
  │          │  
  │          ▼  
  │      RENEWING  (CSR + possession proof sent to CP)  
  │          │  
  │          ▼  
  │       ACTIVE  (hot-reloaded new cert)  
  │  
  ├──► SUSPENDED  (CP-signed suspend command received)  \[NEW\]  
  │          │  
  │          ├── drops all active streams  
  │          ├── rejects all new connections  
  │          ├── heartbeat continues (state: suspended)  
  │          │  
  │          ▼  
  │       ACTIVE  (CP-signed resume command received and verified)  
  │  
  ├──► POLICY\_STALE  (max\_valid\_until elapsed, no newer bundle arrived)  \[NEW\]  
  │          │  
  │          ├── denies all new access requests (fail-closed)  
  │          ├── in-flight sessions continue  
  │          ├── emits stale-policy alert in heartbeat  
  │          │  
  │          ▼  
  │       ACTIVE  (valid newer bundle received and applied)  
  │  
  └──► REVOKED  (revocation event received and verified)  
             │  
             ▼  
      RE-ENROLLMENT  (new CSR + possession proof sent to CP)  
             │  
             ▼  
           ACTIVE
```

### SUSPENDED State **\[NEW\]**

SUSPENDED enables temporary administrative hold on a connector without triggering full revocation. Use cases include:

- Security investigation of a node or the user responsible for it (revocation would alert the subject; suspension is quieter)

- Scheduled maintenance window where the node should not serve traffic

- Compliance hold pending audit completion

| Property | Behaviour |
| - | - |
| Trigger | CP-signed SUSPEND command, relayed through gateway |
| Verification | Connector verifies CP signature before entering SUSPENDED |
| Traffic | All new stream accepts are rejected. Active streams are drained and closed. |
| Heartbeat | Continues. Heartbeat payload includes `state: suspended`. |
| Resume | CP-signed RESUME command, relayed through gateway. Connector verifies signature before returning to ACTIVE. |
| Gateway cannot | Issue SUSPEND or RESUME commands. A compromised gateway cannot suspend or resume connectors. |
| Audit | Every SUSPEND and RESUME is a logged security event with the CP operator identity that issued the command. |


> **Expired on Startup:** If a node starts with an expired certificate it enters BOOTSTRAPPING — not ACTIVE. It contacts CP directly using the break-glass endpoint before attempting any gateway connection.


## 11. Resilience & Production Readiness

### 11.1 Failure Modes

| Failure | Behavior |
| - | - |
| **CP unreachable** | Gateway serves traffic using disk-cached policy + Redis sessions. Retries CP every 30s with jitter. |
| **CP unreachable + policy stale (\>24h)** | Degraded mode: existing sessions continue, new sessions blocked. |
| **Redis down** | Gateway returns 503 for new requests. Existing TCP streams to Connector continue (stateless proxy). |
| **Gateway restart** | Loads policy from disk into memory before accepting HTTP requests. Zero-downtime boot. |
| **CP restart** | 100 gateways reconnect with jittered backoff. No thundering herd. |
| **Policy corruption** | Hash mismatch detected on disk load. Gateway discards local policy, requests full snapshot from CP. |
| **POLICY\_STALE triggered** | Node denies new access requests. In-flight sessions continue. Alert emitted. Retries bundle fetch. |
| **SUSPENDED state** | Node rejects new connections, drains active streams, continues heartbeat. Resumes on CP-signed RESUME. |


### 11.2 Health Checks

**Gateway:**

- `/healthz` — Liveness (HTTP server running)

**CP:**

- `/healthz` — Liveness



### 11.3 Observability

| Metric | Source |
| - | - |
| `ashrix\_auth\_requests\_total` | Gateway (labeled by result: success, failure, redirect) |
| `ashrix\_active\_sessions` | Gateway (Redis key count) |
| `ashrix\_policy\_version` | Gateway (current enforced version) |
| `ashrix\_cp\_stream\_connected` | Gateway (0 or 1, labeled by gateway\_id) |
| `ashrix\_cert\_expiry\_days` | Gateway, Connector (days until cert expiry) |
| `ashrix\_bundle\_apply\_duration\_ms` | Gateway (time to apply delta/snapshot) |
| `ashrix\_revocation\_batch\_size` | CP (number of sessions per batch) |
| `ashrix\_connector\_crl\_hits` | Gateway (revocations caught by per-call check) |
| `ashrix\_policy\_stale\_state` | Gateway, Connector (0 or 1) |
| `ashrix\_node\_suspended\_state` | Gateway, Connector (0 or 1) |
| `ashrix\_breakglass\_attempts\_total` | CP (labeled by node\_id, result) |


### 11.4 Circuit Breakers

- **CP gRPC connection:** If 5 consecutive connection failures, enter "island mode." Serve from cache. Retry every 30s (not exponential — fixed interval in island mode to reduce load).

- **Redis:** If Redis unreachable for \>5s, fail-open on existing sessions (they are in Gateway memory), fail-closed on new sessions.


## 12. Multi-Tenancy & SaaS Isolation

### 12.1 Tenant Isolation

| Layer | Isolation Mechanism |
| - | - |
| **Policy** | `tenant\_id` column in all policy tables. CP filters mutations by tenant before pushing to Gateway. |
| **Session** | Redis keys prefixed: `session:\<tenant\_id\>:\<session\_id\>`. Gateway enforces tenant match between session and requested app. |
| **Certificate** | `tenant\_id` encoded in certificate Subject or SAN. CP validates tenant scope during bootstrap. |
| **Gateway** | May be dedicated per-tenant (enterprise) or shared per-region (SaaS). Shared Gateways enforce tenant boundary in policy engine. |


### 12.2 Gateway Modes

| Mode | Use Case |
| - | - |
| **Dedicated** | One Gateway per tenant. Simpler isolation. Higher cost. |
| **Shared** | One Gateway serves multiple tenants. Tenant identified by SNI / Host header. Policy engine filters by `tenant\_id`. |



## 13. Threat Model & Mitigations

| Threat | Mitigation |
| - | - |
| **Compromised Gateway** | Cannot forge CP commands (Connector verifies CP signature independently). Cannot decrypt Connector traffic (symmetric session keys). Can only drop or log traffic — detected by anomalous metrics. |
| **Compromised Connector** | Cannot impersonate Gateway (mTLS client cert required). Can only exfiltrate its own app traffic. |
| **Stolen session cookie** | Cookie is `HttpOnly` + `Secure` + `\_\_Host-` prefixed. Attacker needs same-origin access or XSS (mitigated by `HttpOnly`). Sessions are revocable via Redis DEL. |
| **Ticket brute force** | 256-bit space (infeasible). Rate limiting on callback endpoint. Short TTL (60s). |
| **Ticket replay** | Single-use atomic redemption. Redis `GETDEL` or Lua script. |
| **Policy downgrade** | Version monotonicity check. Reject `version \<= current`. |
| **Man-in-the-middle (user→Gateway)** | HTTPS with valid public CA certificate. HSTS header enforced. |
| **Man-in-the-middle (Gateway→Connector)** | mTLS with Ashrix Intermediate CA. Per-call CRL check. |
| **CP impersonation** | Embedded CP public key at compile time. All bundles signed and verified. |
| **Expired cert acceptance** | Gateway explicitly rejects expired certs. Break-glass goes directly to CP. |
| **Private key exfiltration** | Keys encrypted at rest. `crypto.Signer` abstraction prevents raw key export. Upgrade path to Vault Agent / Cloud KMS. |
| **Bundle withholding** | `max\_valid\_until` deadline triggers POLICY\_STALE fail-closed state. |
| **Break-glass DoS** | Per-node and per-IP rate limiting with exponential backoff and alerting. |
| **Thundering herd** | Connection age jitter spreads reconnection load across a 2-minute window. |



## 14. Design Principles **\[NEW\]**

| Principle | Rationale |
| - | - |
| One key, one job | No key serves more than one purpose. Limits blast radius on compromise. |
| CP is the sole trust authority | No component can issue, modify, revoke, suspend, or resume trust except the CP. |
| Gateway distributes, never creates | A compromised gateway cannot forge bundles or commands — all are CP-signed. |
| Connector verifies independently | Connectors verify CP signatures directly using the embedded public key. The gateway is not in the verification chain. |
| No expired certs on the wire | Expired certificate recovery bypasses the gateway. The gateway never accepts expired certs under any condition. |
| Possession proof on renewal | Cert renewal always requires proof of private key ownership. Knowing a `node\_id` is not enough. |
| Rotate before expiry | All certs rotate at 2/3 of their validity period — never at expiry. Overlap windows absorb offline nodes. |
| Hot reload everywhere | Cert rotation never causes a process restart. All components support atomic cert swap. |
| Bootstrap token is highest-value target | Token delivery is out-of-band. Every generation and use is a security event. Tokens are node-bound, IP-bound where possible, and consumed atomically. |
| Fail-closed on stale policy | A node that does not receive a bundle update by `max\_valid\_until` denies new access rather than continuing on stale policy. |
| Revocation is a priority message | CRL and revocation events jump the stream queue. Maximum propagation SLA is 5 seconds from CP decision to gateway enforcement. |
| Suspend before revoke | SUSPENDED state enables temporary administrative hold without triggering the irreversible revocation path. |
| Jitter all timers | Connection age, retry intervals, and renewal triggers are jittered to prevent thundering herd at scale. |
| User identity is gateway-injected and connector-verified | The internal app never performs user authentication. Identity is established at the gateway, signed with the CP key, and verified by the connector before the request reaches the app. |



## 15. Connector Architecture

### 15.1 Connector Role

The Connector is the application-side tunnel agent. It runs inside the customer network, adjacent to the protected applications. It has two runtime responsibilities:

| Plane | Protocol | Purpose |
| - | - | - |
| **Management Plane** | mTLS gRPC stream to Gateway | Receives policy bundles, trust bundles, revocation events, rotation commands, config updates. Sends heartbeats, logs, metrics. |
| **Data Plane** | mTLS QUIC to Gateway | Carries multiplexed application traffic (HTTP, TCP) between Gateway and internal applications. |




Both connections are authenticated with the Connector's identity certificate issued by the CP.

```
┌─────────────────┐      mTLS gRPC      ┌─────────────────┐      HTTPS / TCP      ┌─────────────┐  
│    Gateway      │ ◄────────────────► │    Connector    │ ◄──────────────────► │   App       │  
│  (gRPC server)  │   Management      │  (gRPC client)  │    Local network     │ (internal)  │  
│  (QUIC server)  │ ◄────────────────► │  (QUIC client)  │                      │             │  
└─────────────────┘   Data / App        └─────────────────┘                      └─────────────┘
```

### 15.2 Connector Bootstrap

A new Connector has no certificate, no policy, and no knowledge of which applications it serves.

**Prerequisites:**

- Connector binary installed on host

- Bootstrap token generated by CP admin (single-use, 5-minute TTL, bound to `connector\_id`)

- CP break-glass endpoint address embedded in binary

**Flow:**

```
1. Operator starts Connector with token generated on CP  
  
2. Connector generates EC P-256 keypair locally  
  
3. Connector generates CSR:  
 Subject: CN=cn-prod-db-001, O=ashrix, OU=connectors  
 SAN: DNS:cn-prod-db-001  
  
4. Connector POSTs to CP break-glass endpoint over public-CA-validated HTTPS:  
 \{  
 "bootstrap\_token": "\<token\>",  
 "connector\_id": "cn-prod-db-001",  
 "csr": "\<base64-csr\>",  
 "timestamp": 1753389600  
 \}  
  
5. CP validates:  
 - Bootstrap token exists, not used, not expired, bound to cn-prod-db-001  
 - CSR public key is valid EC P-256  
 - timestamp within ±5 minutes of server time  
  
6. CP issues via Intermediate CA:  
 - Connector Identity Certificate (90-day validity)  
 - Trust Bundle \[Root CA, Intermediate CA\]  
 - App List: \[\{app\_id, internal\_host, internal\_port, protocol\}\]  
 - Gateway address: gw-\<region\>-\<tenant\>.ashrix.io:443  
  
7. CP invalidates bootstrap token atomically  
  
8. Connector persists to disk (all AES-256-GCM encrypted):  
   /var/lib/ashrix-connector/  
     ├── key.enc          \# Private key  
     ├── cert.pem         \# Identity certificate  
     ├── trust\_bundle.pb  \# Trusted CA certs  
     └── gateway.addr     \# Gateway endpoint address  
  
9. Connector establishes mTLS gRPC stream to Gateway  
    - Presents connector identity certificate  
    - Gateway verifies cert against trust bundle + CRL  
    - Gateway checks connector\_id is authorized for this Gateway  
  
10. Connector establishes mTLS QUIC connection to Gateway  
    - Same certificate, same verification  
    - QUIC 0-RTT disabled (prevents replay of early data)  
    - ALPN: "ashrix-app-v1"
```



### 15.3 App List & Local Proxy

The **App List** defines which applications this Connector exposes. It is authoritative — the Connector only proxies traffic to apps in this list.

```
\[  
  \{  
    "app\_id": "app-jenkins-prod",  
    "internal\_host": "jenkins.internal",  
    "internal\_port": 8080,  
    "protocol": "http",  
    "health\_check\_path": "/health"  
  \},  
  \{  
    "app\_id": "app-postgres-prod",  
    "internal\_host": "postgres.internal",  
    "internal\_port": 5432,  
    "protocol": "tcp"  
  \}  
\]
```

**Connector behavior:**

- On receiving app traffic from Gateway over QUIC, the Connector checks that the `app\_id` in the stream metadata is present in its local app list

- If present: dial `internal\_host:internal\_port`, proxy bidirectionally

**App list updates:**

- User needs to Restart the connector for it to get the updated app listeners



### 15.4 Management Stream (gRPC)

**Connector → Gateway:**

- Heartbeat (every 20s)

- Access logs (batched)

- Metrics

- App health status

**Gateway → Connector:**

- Policy bundle updates (CP-signed, relayed)

- Trust bundle updates (CP-signed, relayed)

- Revocation events (CP-signed, relayed)

- Rotation commands (CP-signed, relayed)

- Suspend / resume commands (CP-signed, relayed) **\[NEW\]**

**Verification:** Every management payload from Gateway carries a CP signature. The Connector verifies it using the embedded CP public key **before** parsing content. A compromised Gateway cannot forge management commands.


### 15.5 Data Plane (QUIC)

```
QUIC Connection (mTLS)  
  └── Bidirectional Streams  
        ├── Stream 0: HTTP CONNECT tunnel to app-jenkins-prod  
        ├── Stream 1: HTTP CONNECT tunnel to app-jenkins-prod    
        ├── Stream 2: Raw TCP tunnel to app-postgres-prod  
        └── ...
```

**Stream metadata (sent by Gateway at stream creation):**

```
:authority = app-jenkins-prod.internal  
x-ashrix-session-id = \<opaque\>  
x-ashrix-user-id = u-123  
x-ashrix-groups = eng,oncall
```

**Connector verifies on each new stream:**

1. `app\_id` in metadata exists in local app list

2. Gateway's mTLS certificate is valid and not revoked (CRL check)

3. (Optional) Stream metadata signature from Gateway if implemented



**QUIC-specific security:**

- **0-RTT disabled** on both client and server. Prevents replay attacks.

- **Connection migration** enabled for NAT rebinding resilience.

- **Max stream ID** limits prevent resource exhaustion.

- **Idle timeout:** 5 minutes. Gateway or Connector may close idle connections.

### 15.6 Certificate Renewal

**Trigger:** Day 60 of 90-day certificate, or CP rotation command received via management stream.

**Flow:**

```
1. Connector detects cert within renewal window  
2. Generates new EC P-256 keypair  
3. Creates CSR with same connector\_id  
4. Generates possession proof:  
   nonce = 32-byte CSPRNG  
   timestamp = current\_unix\_time  
   proof = ECDSA\_SIGN(SHA256(csr || connector\_id || timestamp), current\_private\_key)  
  
5. Connector contacts CP directly via break-glass HTTPS endpoint  
   POST /v1/renew  
   \{  
     "connector\_id": "cn-prod-db-001",  
     "csr": "\<base64-csr\>",  
     "timestamp": 1753389600,  
     "signature": "\<base64-signature\>"  
   \}  
  
6. CP verifies:  
   - connector\_id is registered  
   - timestamp within ±5 minutes  
   - proof signature valid against current certificate's public key  
  
7. CP issues new certificate via Vault Intermediate CA  
  
8. Connector receives:  
   - New certificate  
   - Updated trust bundle (if Intermediate CA rotation in progress)  
   - Updated app list (if changed)  
  
9. Connector hot-reloads:  
   - Writes new cert to disk (atomic rename)  
   - Loads new cert into TLS config  
   - Reconnects gRPC management stream with new cert  
   - Reconnects QUIC data plane with new cert  
   - Old connections drain gracefully
```

> **Security:** The possession proof binds the CSR, node identity, and a timestamp. A captured proof from a previous renewal cannot be replayed (timestamp window).

### 15.7 Expired Certificate Recovery

**Rule:** The Connector never presents an expired certificate to the Gateway. If the certificate is expired on startup, the Connector bypasses the Gateway entirely.


**Flow:**

```
1. Connector starts, loads certificate from disk  
2. Checks NotAfter. If expired:  
 a. Do NOT attempt Gateway connection  
 b. Enter RECOVERY state  
  
3. Connector contacts CP directly via break-glass HTTPS  
 POST /v1/recover  
 \{  
 "connector\_id": "cn-prod-db-001",  
 "csr": "\<base64-csr\>",       // New keypair generated for recovery  
 "timestamp": 1753389600,  
 "signature": "\<base64-signature\>"  // Signed with OLD private key (still on disk)  
 \}  
  
4. CP verifies:  
 - connector\_id registered  
 - timestamp within ±5 minutes  
 - proof signature valid against LAST KNOWN public key in CP database  
 (NOT against the expired certificate — the certificate may have expired,  
 but the private key proving possession is what matters)  
  
5. CP issues replacement certificate  
  
6. Connector persists new cert, enters BOOTSTRAPPED state  
7. Connects to Gateway with valid certificate
```


> **Critical:** The Gateway rejects expired certificates at the TLS handshake level. The Connector must know not to waste time attempting a Gateway connection with an expired cert. The break-glass endpoint is the only path.


### 15.8 Revocation Handling

**Connector receives revocation event (CP-signed) via Gateway management stream:**

```
1. Connector verifies CP signature on revocation event  
2. Checks if revoked serial matches its own certificate serial  
3. If match:  
   a. Immediately close all QUIC streams  
   b. Terminate gRPC management stream  
   c. Enter REVOKED state  
   d. Stop all traffic proxying  
   e. Attempt re-enrollment via break-glass if permitted by policy
```

**If Gateway is compromised and sends forged revocation:**

- Signature verification fails against embedded CP public key

- Connector ignores the event, logs anomaly, continues operation

### 15.9 Connector State Machine



```
INIT  
 │  
 ▼  
BOOTSTRAPPING ───────────────────────────────┐  
 │                                            │  
 ▼                                            │  
ACTIVE (gRPC + QUIC established)                │  
 │                                            │  
 ├──► RENEWING (day 60, new CSR to CP)        │  
 │      │                                     │  
 │      ▼                                     │  
 │   ACTIVE (hot-reload new cert)             │  
 │                                            │  
 ├──► SUSPENDED (CP-signed suspend command)   │  
 │      │                                     │  
 │      ├── drops all active streams          │  
 │      ├── rejects all new connections       │  
 │      ├── heartbeat continues               │  
 │      │                                     │  
 │      ▼                                     │  
 │   ACTIVE (CP-signed resume command)        │  
 │                                            │  
 ├──► POLICY\_STALE (max\_valid\_until elapsed)  │  
 │      │                                     │  
 │      ├── denies new access requests        │  
 │      ├── in-flight sessions continue       │  
 │      ├── emits stale-policy alert          │  
 │      │                                     │  
 │      ▼                                     │  
 │   ACTIVE (valid newer bundle received)     │  
 │                                            │  
 ├──► REVOKED (revocation event received)   │  
 │      │                                     │  
 │      ▼                                     │  
 │   RE-ENROLLING ────────────────────────────┘  
 │      (break-glass to CP)  
 │  
 └──► EXPIRED\_ON\_STARTUP  
 │  
 ▼  
 RECOVERING (break-glass to CP)  
 │  
 ▼  
 BOOTSTRAPPING
```

### 15.10 Security Controls

| Control | Implementation |
| - | - |
| **Payload verification** | All management payloads verified with embedded BundleSigning public key before parsing. |
| **App list enforcement** | Connector only proxies to hosts/ports in its local app list. Unknown app\_ids are rejected. |
| **No inbound listen** | Connector initiates outbound connections only. No listening ports exposed to the network. |
| **Key isolation** | Private key in `crypto.Signer`, never exported as raw bytes after load. |
| **Disk encryption** | All persisted files encrypted with AES-256-GCM  |
| **Break-glass TLS** | Always public-CA-validated HTTPS. Never mTLS (cert may be expired/missing). |



### 15.11 Failure Modes

| Scenario | Behavior |
| - | - |
| **Gateway unreachable** | Retry gRPC and QUIC with jittered backoff. Serve no traffic. App list remains cached. |
| **Gateway reachable but cert rejected** | Check if cert expired → recovery flow. If not expired → log anomaly, retry. |
| **CP break-glass unreachable** | Exponential backoff to CP. If cert not yet expired, continue serving via existing Gateway connection. |
| **App list desync** | Connector uses local cached app list. New streams for unknown apps are rejected. CP pushes updates via management stream when connectivity restores. |
| **QUIC connection dies, gRPC stays up** | Re-establish QUIC only. gRPC management stream remains for control. |
| **gRPC dies, QUIC stays up** | Mark QUIC as draining. Re-establish gRPC. If gRPC fails for \>60s, close QUIC and retry both. |






## 16. Identity Provider Integration

### 16.1 Multi-IdP Architecture

Every tenant (organization) may configure **multiple** identity providers. Applications are linked to IdPs via a **many-to-many** relationship. A user authenticating to `app-jenkins` may be redirected to Okta; a user accessing `app-gitlab` may be redirected to Azure AD.

```
Tenant: acme-corp  
├── IdP-1: Okta (oidc)  
│     └── Apps: app-jenkins, app-confluence  
├── IdP-2: Azure AD (oidc)  
│     └── Apps: app-gitlab, app-sharepoint  
└── IdP-3: Google Workspace (oidc)  
      └── Apps: app-drive, app-sheets
```

**IdP Configuration (per tenant):** | Field | Description | |---|---| | `idp\_id` | UUID | | `tenant\_id` | FK to tenant | | `provider\_type` | `oidc` (MVP); `saml` (post-MVP) | | `discovery\_url` | OIDC `.well-known/openid-configuration` | | `client\_id` | OAuth client ID | | `client\_secret\_encrypted` | AES-256-GCM encrypted in CP database | | `group\_mappings` | JSON: `\{"idp\_group": "ashrix\_group"\}` | | `scopes` | Default: `openid profile email` | | `mfa\_claim` | Claim name indicating MFA status (e.g., `amr`, `acr`) | | `enabled` | Boolean |

### 16.3 JIT Provisioning (MVP)

CP Doesn’t have any records of user or groups. Only authenticates using IDP provider:

### 16.4 SCIM Integration (Deprovisioning & Revocation)

SCIM is **not** used for provisioning in MVP. It is used exclusively for **deprovisioning** — translating IdP user lifecycle events into Ashrix session revocations.

**SCIM Inbound Endpoint (CP exposes to IdP):**

```
POST /scim/v2/Users          → Create user (no-op in MVP, return 201)  
PUT  /scim/v2/Users/\{id\}    → Update user (sync groups if changed)  
DELETE /scim/v2/Users/\{id\}  → Deactivate user → trigger revocation cascade
```

**Revocation Cascade on User Deactivation:**

```
1. IdP sends DELETE /scim/v2/Users/u-123 (or PATCH status=inactive)  
2. CP queries active sessions for user\_id = u-123  
3. CP generates revocation batch:  
 \{batch\_id: "rev-uuid", session\_ids: \["sid-1", "sid-2", ...\]\}  
4. CP pushes revocation batch to all Gateways via gRPC ControlStream  
5. Gateways delete sessions from Redis immediately  
6. CP logs: user.deactivated, sessions.revoked
```



**SCIM Security:**

- IP allowlisting optional (if IdP egress IPs are known)

- Request signing verification if IdP supports it


## 17. Session Lifecycle & Security Controls

### 17.1 Session Design Principles

As a ZTNA security architecture, sessions must balance **user experience** with **Zero Trust posture**. The following controls are recommended:

| Control | Recommendation | Rationale |
| - | - | - |
| **Absolute maximum lifetime** | 8 hours | Prevents indefinite sessions even with continuous activity. Forces re-authentication daily. |
| **Idle timeout** | 30 minutes | If the user walks away, the session dies. Reduces physical-access attack window. |
| **Sliding window** | Yes, with absolute cap | Every request extends Redis TTL up to the 8-hour absolute limit. User is not interrupted during active work. |
| **Re-authentication triggers** | Policy change, posture degradation, MFA step-up | Zero Trust requires continuous validation, not one-time authentication. |
| **Concurrent session limit** | 5 per user per app | Prevents session sprawl and credential sharing. |


### 17.2 Logout Flow

**User-Initiated Logout:**

```
1. User clicks "Logout" in application  
2. App redirects to: POST https://gateway.app-a.com/\_auth/logout  
3. Gateway:  
   a. Reads \_\_Host-gw\_sid cookie  
   b. Deletes session from Redis: DEL session:\<sid\>  
   c. Clears cookie: Set-Cookie \_\_Host-gw\_sid=""; Max-Age=0  
   d. Redirects to CP logout endpoint: https://auth.ashrix.io/logout?post\_logout\_redirect\_uri=...  
4. CP:  
   a. Terminates CP-side OIDC session  
   b. Redirects to IdP logout endpoint (if OIDC RP-Initiated Logout supported)  
   c. Redirects user back to application landing page
```

**Security note:** Clearing the Gateway cookie is local to that Gateway domain. If the user has sessions on multiple Gateways (app-a, app-b), they must log out from each or CP must push a cross-gateway revocation. For MVP, document that logout is **per-gateway** and CP may optionally push revocation to all user sessions on tenant logout.

### 17.3 Force Kill (Admin / CP-Initiated)

```
1. Admin clicks "Kill Session" in CP dashboard (or SCIM deprovisioning triggers)  
2. CP generates revocation batch containing session\_id  
3. CP pushes to Gateway via gRPC ControlStream  
4. Gateway: DEL session:\<sid\> from Redis  
5. Gateway: If session is actively being used, next request hits 401  
6. CP logs: session.force\_revoked, actor: admin\_id, reason: "admin\_action"
```

**Alternative for immediate kill on active requests:** Gateway maintains an in-memory `sync.Map` of revoked session IDs (5-minute TTL). The HTTP middleware checks this map **before** Redis lookup. This catches in-flight requests without waiting for Redis propagation delay.

## 18. Device Posture (MVP)

### 18.1 Scope

MVP posture is **passive** — derived from the HTTP request, not from an endpoint agent.

| Signal | Source | Enforcement |
| - | - | - |
| **User-Agent** | HTTP header | OS/browser detection, block outdated browsers |
| **Source IP** | TCP connection / X-Forwarded-For | Geo-location, IP reputation, CIDR allow/block lists |
| **Geo-location** | IP geolocation DB | Country/city allow/block lists |


### 18.2 Posture Drift Handling

**MVP Behavior (Recommended):**

- Posture is captured at session creation time

- On each request, Gateway compares current posture to session posture

- If IP changes (e.g., user switches from WiFi to mobile): **update posture and log** (do not kill session in MVP — too disruptive)

- If posture change crosses a policy boundary (e.g., moved from "allowed country" to "blocked country"): **kill session immediately**

**Post-MVP:** Agent-based posture (disk encryption, AV status, OS patch level) collected via lightweight agent or MDM integration.


## 19. Bootstrap Token Lifecycle

### 19.1 Generation

**Web UI (CP Dashboard):**

```
Admin navigates to: Tenant → Connectors → Add Connector  
1. Enters connector\_id: cn-prod-db-001  
2. Selects apps to link: \[app-jenkins, app-postgres\]  
3. Clicks "Generate Token"  
5. CP generates: token = "ashrix\_bt\_" + 32-byte CSPRNG, base64url  
6. CP stores in DB: hash(token) = bcrypt(token, cost=12)  
7. CP displays token ONCE to admin (plaintext never stored)  
8. Admin copies token and provides it to Connector host
```

### 19.2 Validation at Bootstrap

**Security considerations:**

- **Timing attack resistance:** bcrypt comparison is constant-time. Database lookup by `node\_id` then iterate + bcrypt verify.

- **Rate limiting:** Max 5 failed bootstrap attempts per `node\_id` per hour. Excess returns 429 and logs anomaly.

- **Audit:** Every bootstrap attempt (success or failure) is logged with IP, timestamp, and node\_id.

### 19.3 Regenerate Token

Admin can regenerate another token in the Web UI:

A former token fails validation immediately, even if not expired and not fully used.

### 19.4 Cleanup

Expired tokens are purged by a daily background job:



## 20. Deployment Topology

## 20.1 Multi-Region Deployment

Enterprise tenants with global subsidiaries may deploy **multiple regional gateways** under a single tenant identity.

### Example: GlobalCorp

```
Tenant: GlobalCorp  
├── Region: Americas  
│   ├── Gateway: gw-globalcorp-us (AWS us-east-1)  
│   ├── Connector: conn-us (Brazil office)  
│   └── Apps: SAP-US, SharePoint-US  
├── Region: Africa  
│   ├── Gateway: gw-globalcorp-africa (Azure South Africa)  
│   ├── Connector: conn-africa (Nigeria office)  
│   └── Apps: ERP-Africa, FileServer-Africa  
└── Region: Asia  
 ├── Gateway: gw-globalcorp-asia (GCP Singapore)  
 ├── Connector: conn-asia (Singapore office)  
 └── Apps: CRM-Asia
```


**Key insight**: The CP is the routing layer. Gateways do not share session state. Each gateway independently validates CP-signed tokens.

User Access Flow (Multi-Region) : To be defined

## 20.2 Tenant Model

Ashrix is a **multi-tenant platform** at the CP level, but **single-tenant at the data plane level**.

### 20.2.1 Tenant Types

| Attribute | `Ashrix\_public` | Enterprise (Self-Hosted) | Enterprise (Ashrix-Managed) |
| - | :-: | - | - |
| **Target User** | Individuals, developers, hobbyists | Organizations with IT teams | Organizations without IT ops capacity |
| **Gateway Ownership** | Ashrix | Tenant | Ashrix (deployed in tenant cloud account) |
| **Gateway Location** | Ashrix infrastructure | Tenant infrastructure (on-prem, VPC, etc.) | Tenant's cloud account (AWS/Azure/GCP) |
| **App Types** | Public apps only | Public + private network apps | Public + private network apps |
| **Authentication** | Ashrix-managed or social SSO | Tenant IdP (SAML/OIDC) | Tenant IdP (SAML/OIDC) |
| **Policies** | Basic (public/private toggle) | Granular (group, device, MFA, time) | Granular |
| **HA** | Best effort | Tenant-configured active-passive/active-active | Ashrix-configured |
| **SLA** | None | Negotiated | Negotiated |
| **Support** | Community | Dedicated | Dedicated |



### 20.2.2 Tenant Isolation Guarantees

- **Traffic isolation**: Tenant A's traffic never traverses Tenant B's gateway.

- **Cryptographic isolation**: Each tenant gateway validates Local Sessions with tenant-specific claims.

- **Config isolation**: Tenant policies, app catalogs, and session registries are namespaced by `tenant\_id` in the CP database.

- **Audit isolation**: Audit logs are tagged by `tenant\_id` and accessible only to that tenant's admins.




## 20.3. Deployment Options

### 20.3.1 Self-Hosted Gateway

**Ideal for**: Enterprises with strict security, air-gapped networks, or mature platform teams.


**Tenant responsibilities:**

- Provision VM/container (Linux, 4+ vCPU, 8+ GB RAM).

- Open outbound HTTPS (443) and gRPC (8443) to CP.

- Run Redis locally (or use in-memory for small deployments).

- Monitor disk, CPU, and memory.

- Apply OS security patches.

**Ashrix responsibilities:**

- Provide signed binary and Docker image.

- Provide auto-update mechanism.

- Push config and commands via gRPC.

- Monitor gateway health via heartbeats.


### 20.3.2 Ashrix-Managed Gateway

**Ideal for**: Enterprises that want isolation without operational burden.

**Deployment:**

1. Tenant provides Ashrix with cloud account credentials (IAM role in AWS, Service Principal in Azure).

2. Ashrix deploys a CloudFormation/Terraform stack into the tenant's account.

3. The stack provisions a gateway VM, Redis, and security groups.

4. Ashrix has admin access for updates and monitoring; tenant retains root ownership of the account.

**Billing:**

- Tenant pays their cloud provider directly for compute.

- Ashrix bills a management fee per gateway.



## 20.4. Ashrix\_public (Free Tier)

### 20.4.1 Purpose

`Ashrix\_public` is the **acquisition funnel**. It allows individual users to expose local applications to the internet using Ashrix-managed infrastructure, similar to Cloudflare Tunnel or ngrok.

### 20.4.2 Limitations

| Limit | Value | Rationale |
| - | - | - |
| **Apps** | Public apps only | No access to private network resources. |
| **Connectors** | 1 per user | Simplifies abuse detection. |
| **Rate limit** | 100 req/min per tunnel | Prevents botnet abuse. |
| **Bandwidth** | 1 Mbps sustained | Sufficient for development, not production. |
| **Session TTL** | 30 minutes idle timeout | Keeps Redis memory low. |
| **Session persistence** | In-memory only | Data loss on gateway restart is acceptable. |
| **Custom domains** | Not allowed | Only `\*.ashrix.io` subdomains. |
| **Support** | Community only | No SLA. |



### 20.4.3 Abuse Mitigation

Because `Ashrix\_public` provides free internet egress, it is a target for abuse:

| Control | Implementation |
| - | - |
| **Domain reputation** | Block tunnels to known-malicious destinations (phishing, C2, malware). |
| **Content scanning** | Basic HTTP header inspection for suspicious patterns. |
| **Rate limiting** | Per-IP and per-account throttling. |
| **Account verification** | Email verification + optional phone verification for new accounts. |
| **Automated suspension** | ML-based anomaly detection triggers account review. |
| **Report abuse** | Public abuse reporting form with 24-hour response SLA. |


### 20.4.4 Migration to Enterprise

Users must be able to migrate from `Ashrix\_public` to an Enterprise tenant:

1. User invites their company admin from the public dashboard.

2. Admin creates an Enterprise tenant.

3. User's public apps can be **transferred** or **re-enrolled** under the Enterprise tenant.

4. Post-migration, the user's apps are subject to Enterprise policies and the Enterprise gateway.


## 20.5. Enterprise Tier

### 20.5.1 Features

| Feature | Description |
| - | - |
| **Unlimited connectors** | Deploy connectors across all subsidiaries and cloud regions. |
| **Private apps** | Access internal apps (RFC 1918 addresses, on-prem, VPC). |
| **Custom IdP** | SAML 2.0 and OIDC integration with corporate identity providers. |
| **Granular policies** | Per-app, per-group, device posture, MFA, time-of-day, and geo-location policies. |
| **Multi-region gateways** | Deploy gateways in Americas, EMEA, and APAC under one tenant. |
| **High Availability** | Active-passive or active-active gateway clusters. |
| **Audit retention** | 1-year default, extendable to 7 years. |
| **Dedicated support** | Email and Slack support with 4-hour response SLA. |






## 20.6 Network Requirements

| Component | Outbound | Inbound | Notes |
| - | - | - | - |
| **CP** | None (except IdP callbacks, SCIM) | 443/TCP (gRPC + HTTP) | Behind L4 LB |
| **Gateway** | CP DB, Redis, Vault | 443/TCP (user HTTPS, Connector gRPC/QUIC) | Public Ips/ DMZ |
| **Connector** | Gateway:443, CP:443 (break-glass) | None | Outbound only |
| **Browser** | Gateway:443 | None | Standard HTTPS |



**Root CA HSM loss:** Requires formal ceremony with offline key shards. Documented in separate operational runbook (not architecture doc). HSM key is never in software; loss means re-enrollment of all nodes.




## 21. Security Audit Checklist

This section maps the architecture against common security audit frameworks (SOC 2, ISO 27001, NIST 800-207 Zero Trust) to demonstrate coverage.


### 21.1 Trust Boundaries

| Boundary | Trust Level | Notes |
| - | - | - |
| **User Browser** | Untrusted | Validates HTTPS/TLS. Presents JWT. |
| **Internet** | Untrusted | All traffic is TLS 1.3 or QUIC (encrypted). |
| **Gateway** | Tenant-trusted | Runs in tenant infrastructure. Decrypts traffic for local apps. |
| **Connector** | Tenant-trusted | Runs inside tenant network. Has access to internal apps. |
| **CP** | Ashrix-trusted | Operated by Ashrix. Never sees application plaintext. |


### 21.2 Ashrix Cannot See Tenant Traffic

Because the gateway runs in the tenant's environment:

- Ashrix operates the CP, which only handles auth metadata and policy decisions.

- The gateway terminates user TLS and re-encrypts (or forwards) to the connector.

- Ashrix has no access to the gateway's memory, Redis, or disk unless the tenant chooses **Ashrix-managed** deployment.


### 21.3 gRPC/mTLS Security

- All gateway-to-CP communication uses mutual TLS.

- Gateways authenticate with **enrollment tokens** (short-lived, single-use) during initial registration.

- After registration, the CP issues a **long-lived client certificate** to the gateway for ongoing mTLS.

- Certificate rotation is handled automatically by the gateway's auto-updater.



### 21.4 Authentication & Authorization

| Requirement | Implementation | Evidence |
| - | - | - |
| Strong identity verification | OIDC with external IdP (Okta, Azure AD, Google) | Section 17 |
| MFA enforcement | Policy-level `require\_mfa` flag; IdP claim verification | Section 18.2 |
| Least privilege access | Per-app AccessRules with group/CIDR/posture matching | Section 6 |
| Session binding | Cookie `\_\_Host-` prefix, HttpOnly, Secure, SameSite=Lax | Section 5.4 |
| Session timeout | Absolute 8h + idle 30min + revocable via Redis | Section 18 |
| Concurrent session limits | 5 per user per app, enforced in Redis | Section 18.6 |


### 21.5 Cryptography

| Requirement | Implementation | Evidence |
| - | - | - |
| Key hierarchy | Root CA (HSM) → Intermediate CA (Vault) → Node certs | Section 2.2 |
| Key separation | One key, one job. CP signing key ≠ CA key ≠ node key | Section 2.2 |
| Key storage | Root: HSM. Intermediate: Vault. Nodes: AES-256-GCM encrypted disk | Section 2.2, 15 |
| TLS everywhere | mTLS for all server-to-server. Public CA TLS for user-facing | Throughout |
| Certificate rotation | Automated at 2/3 validity. Possession proof required | Section 8 |



### 21.6 Zero Trust Principles (NIST 800-207)

| Principle | Implementation |
| - | - |
| **Assume breach** | Every request authenticated + authorized. No implicit trust inside network. |
| **Verify explicitly** | mTLS + session cookie + policy engine on every request. |
| **Least privilege** | Per-app AccessRules. No network-level access grants. |
| **Continuous validation** | Session posture drift checks. CRL per-call. Policy delta sync. |


### 21.7 Audit & Logging

| Requirement | Implementation |
| - | - |
| Immutable audit trail | All auth, policy, cert, session events logged as structured JSON |
| Admin action attribution | Every policy change, revocation, token generation attributed to admin user |
| Failed access attempts | Rate-limited, logged with IP, user agent, timestamp |
| Anomaly detection | Gateway reports anomalous patterns to CP (unusual geo, rapid session cycling) |

### 22.5 Data Protection

| Requirement | Implementation |
| - | - |
| Encryption at rest | Private keys: AES-256-GCM. Policy DB: SQLCipher. CP DB: Transparent DB encryption |
| Encryption in transit | TLS 1.3 (user), mTLS (internal). No unencrypted channels. |
| Key rotation | CP signing key: planned, rare, requires binary rebuild. Node keys: every 60 days. |
| Secure deletion | Memory wipe of plaintext keys after load into `crypto.Signer` |


### 22.6 Availability & Resilience

| Requirement | Implementation |
| - | - |
| No single point of failure | 3+ CP replicas, multi-AZ Redis |
| Graceful degradation | Gateway serves from local cache if CP unreachable. Fail-closed if policy stale \>24h. |
| Recovery procedures | Expired cert break-glass. Bootstrap token re-issuance. Documented runbooks. |




## 23. Operational Considerations

### 23.1 Gateway Auto-Updates

With hundreds of tenant gateways, manual patching is impossible. The gateway must self-update:

```
1. Gateway polls CP every 15 minutes for version manifest.  
2. If new version available, gateway downloads signed binary.  
3. Gateway verifies signature using embedded public key.  
4. Gateway performs rolling restart:  
   a. Start new binary on alternate port.  
   b. Health check new binary.  
   c. If healthy, switch traffic, stop old binary.  
   d. If unhealthy, rollback to old binary, alert CP.  
5. Gateway reports version to CP.
```

### 23.2 Disaster Recovery

| Scenario | Recovery |
| - | - |
| **CP outage** | Gateways continue operating with cached config and local sessions. New logins fail (no token validation), but existing sessions persist until TTL. |
| **Gateway outage** | Users reconnect to the passive gateway (HA pair). If no HA, users cannot access apps in that region until the gateway recovers. |
| **Connector outage** | Gateway marks connector as offline. Apps behind that connector are unreachable until the connector reconnects. |
| **Submarine cable cut** | Multi-region tenants route through alternate gateways. Single-region tenants experience outage until connectivity restores. |




### 23.3 Monitoring & Alerting

| Metric | Source | Alert Threshold |
| - | - | - |
| Gateway heartbeat | CP | \> 60 seconds since last heartbeat |
| Gateway CPU | Gateway | \> 80% for 5 minutes |
| Gateway memory | Gateway | \> 85% for 5 minutes |
| Connector reconnect rate | Gateway | \> 10 reconnects/minute |
| Session creation rate | Gateway | \> 1000/minute (DDoS indicator) |
| gRPC stream errors | Gateway/CP | \> 5 errors/minute |






## 24. Future Considerations

| Feature | Description | Priority |
| - | - | - |
| **Global Anycast** | Offer Ashrix-managed anycast IPs for Enterprise tenants who want global ingress without managing DNS geo-routing. | Medium |
| **Edge Relay** | Provide Ashrix-operated relay nodes for users who cannot reach a self-hosted gateway directly (e.g., both user and gateway are behind symmetric NAT). | Medium |
| **Device Posture** | Integrate with endpoint detection and response (EDR) tools to enforce device health before granting access. | High |
| **SSH/RDP Native** | Native protocol support for SSH (certificate-based) and RDP (guacamole-style or native relay). | High |
| **API Access** | Machine-to-machine authentication using client credentials flow for service accounts. | Medium |
| **Federation** | Cross-tenant trust for B2B collaboration (Vendor A's employees access Vendor B's apps via their own IdP). | Low |





## Appendix A: Glossary

| Term | Definition |
| - | - |
| **CP** | Control Plane. The Ashrix-operated SaaS that manages identity, policy, and configuration. |
| **Gateway** | The tenant-owned proxy that terminates user connections and forwards to connectors. |
| **Connector** | The lightweight agent deployed inside the tenant network that opens outbound tunnels to the gateway. |
| **ZTNA** | Zero Trust Network Access. A security model that assumes no trust based on network location. |
| **QUIC** | A transport protocol built on UDP, used for connector-to-gateway tunnels. |
| **gRPC/mTLS** | gRPC over mutual TLS. The protocol for gateway-to-CP communication. |
| **Anycast** | A network addressing method where the same IP is advertised from multiple locations, routing users to the nearest. |



*Ashrix ZTNA — Production Architecture. Confidential.* *v1.1*

