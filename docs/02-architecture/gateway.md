# Gateway

## Role

The gateway is the **Policy Enforcement Point (PEP)** and **user-facing access broker**. It is the only component that:

- Accepts connections from end users.
- Terminates user TLS.
- Validates session tokens.
- Evaluates access policy.
- Routes authorised requests through Connector tunnels to internal applications.

The gateway runs in the **tenant's infrastructure** — not Ashrix's. Ashrix never sees the application payload traffic.

---

## Startup Sequence

```
1. Gateway binary starts; loads config from disk (or env)

2. Checks for existing identity certificate:
   a. If present and valid → skip to step 5
   b. If present but expired → enter RECOVERY state → break-glass to CP
   c. If absent → bootstrap with enrollment token

3. Bootstrap (first run):
   - Generate ECDSA P-256 keypair
   - Send CSR + bootstrap token to CP /v1/bootstrap
   - Receive: identity cert, trust bundle, tenant config
   - Persist to /var/lib/ashrix-gateway/ (AES-256-GCM encrypted)

4. Load policy from BBolt local database (policy.db):
   - If present and valid: apply immediately
   - If corrupt: discard, request full snapshot from CP after connecting

5. Start Chi HTTP server (serves traffic with cached policy immediately)

6. Establish outbound gRPC/mTLS Control Stream to CP (async)

7. Sync policy delta or full snapshot in background

8. Begin accepting Connector registrations over gRPC/QUIC
```

**Zero-downtime boot:** The gateway serves traffic using its cached policy before the CP connection is established. This minimises the window during which restarts cause service interruption.

---

## Authentication and Session Flow 

### The Cross-Domain Problem

The CP authenticates users at `auth.ashrix.io`. The gateway serves applications at `app-a.customer.com`. Browsers do not share cookies across domains.

**Solution:** The CP issues a short-lived, single-use **session ticket** (opaque 32-byte base64url, 60-second TTL). The gateway redeems the ticket via gRPC and creates a gateway-local, domain-scoped session.

### Step-by-Step Flow

```
1. User requests https://app.company.com

2. Gateway checks for session cookie (__Host-gw_sid):
   - Missing or invalid → generate CSRF state, store state, user request url in Redis,
     set __Host-ashrix_state cookie, redirect to:
     CP_URL/authorize/providers?aid=<>&gid=<>
     (gid=gateway_id; aid=app_id)

3. CP resolves IdP providers for the app, returns provider list to user

4. User selects IdP; CP creates state in Redis (tracking gateway_id),
   redirects user to IdP OIDC authorization endpoint

5. IdP authenticates user (MFA if required), redirects back to
   CP callback: POST {URL}/callback, the URL depends on the CP url and it should be configured on IDP. /callback is the path

6. CP exchanges authorization code for tokens with IdP
   - Verifies Redis state to recover original gateway_id
   - Creates session in CP Redis
   - Issues session ticket (32-byte opaque, 60s TTL, single-use)
   - Redirects user to: {gateway url}/_ashrix/auth/callback?state=<ticket>

7. Gateway /_ashrix/auth/callback:
   - Enforces a Redis-backed rolling 60-second limit of 10 requests per client IP and 100 gateway-wide
   - Validates __Host-ashrix_state cookie against query param (CSRF check)
   - Exchanges state/ticket with CP via gRPC ControlStream
   - Receives: user_id, email, groups, app_id, MFA level
   - Creates gateway-local session in Redis:
     session:<tenant_id>:<session_id> → {user_id, email, groups, app_id, expires_at}
   - Sets __Host-gw_sid cookie (HttpOnly, Secure, SameSite=Lax)
   - Redirects to original app URL

8. Subsequent requests:
   - Gateway validates __Host-gw_sid against Redis
   - If valid: inject identity headers, forward to Connector
   - If invalid/expired: restart auth flow
```

### Session Cookie Properties

| Attribute | Value | Rationale |
|---|---|---|
| Name | `__Host-gw_sid` | `__Host-` prefix prevents subdomain cookie injection |
| HttpOnly | Yes | Inaccessible to JavaScript; mitigates XSS session theft |
| Secure | Yes | Transmitted only over HTTPS |
| SameSite | Lax | Prevents CSRF from cross-site requests |
| Domain | None (host-locked) | Scoped to the exact gateway host |
| Max-Age | 8 hours (default) | Configurable per gateway(SessionTTL) |

---

## Policy Enforcement

The gateway evaluates access policy **locally** against its in-memory copy of the tenant's policy bundle. The CP is not consulted on a per-request basis.

### Policy Evaluation Sequence

```
Incoming authenticated request
        │
        ▼
1. Load session from Redis → extract user_id, groups, app_id
        │
        ▼
2. Look up policy rules for this app_id (from cached bundle)
        │
        ▼
3. Evaluate each matching rule:
   - User ID match?
   - Group membership match?
   - MFA level satisfied?
   - Source IP in allowed CIDR?
        │
        ▼
4. First matching ALLOW rule → proceed
   No matching ALLOW rule → 403 Forbidden
        │
        ▼
5. Inject identity headers:
   - Strip all incoming X-Ashrix-* headers before forwarding
   - Populate identity values from the verified session and resolved application
   X-Ashrix-User-ID:    <user_id>
   X-Ashrix-User-Email: <email>
   X-Ashrix-App-ID:     <app_id>
   X-Ashrix-Request-ID: <trace_id>
        │
        ▼
6. Forward to Connector over QUIC tunnel
```

### Policy-Decision vs. Policy-Enforcement Separation

The gateway contains both the Policy Decision Point (PDP) and the Policy Enforcement Point (PEP) in the same process. These are logically distinct:

- **PDP** (`internal/gateway/policy`): evaluates the policy bundle and returns allow/deny.
- **PEP** (middleware in `internal/gateway/router`): acts on the PDP decision — forwards or rejects the request.

---

## Connector Registry

The gateway maintains a registry of connected connectors (`internal/gateway/registry`). For each connected connector, the gateway tracks:

- `connector_id`
- mTLS identity certificate (serial, expiry)
- App list (which application IDs this connector proxies)
- Connection health status (last heartbeat timestamp)
- QUIC connection handle

When a request arrives for `app-jenkins`, the gateway:

1. Looks up which connector(s) expose `app-jenkins`.
2. Selects the healthy connector.
3. Opens a new QUIC stream on the existing mTLS QUIC connection.
4. Attaches stream metadata headers (session ID, user identity).
5. Proxies the request bidirectionally.

---

## Local Policy Persistence

The gateway stores its policy bundle in BBolt (`policy.db`) at `/var/lib/ashrix-gateway/policy.db`. This allows the gateway to:

- Start serving traffic immediately on restart, without waiting for the CP.
- Survive transient CP unavailability without service interruption.

**Boot sequence:**
1. Load `policy.db` into memory.
2. Verify bundle signature and check version is not stale.
3. Start HTTP server (traffic served immediately with cached policy).
4. Connect to CP asynchronously and sync delta in background.

---

## Shutdown and Cleanup

On receiving `SIGTERM`:

1. Stop accepting new HTTP connections.
2. Allow in-flight requests to complete (graceful drain, configurable timeout).
3. Close all QUIC streams to connectors gracefully (send GOAWAY).
4. Close gRPC Control Stream.
5. Flush pending audit logs to CP.
6. Exit.

Active sessions survive a gateway restart if the new instance reads the same Redis.

---

*Next: [Connector →](./connector.md)*
