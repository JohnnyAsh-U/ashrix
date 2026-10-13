# Request Lifecycle

This document traces the complete lifecycle of a user request through the Ashrix system, from the browser's first connection to the response arriving from the internal application.

It covers two distinct flows:

1. **Session establishment** — authenticating the user and creating a gateway session.
2. **Subsequent requests** — serving an authenticated, authorised request over the existing session.

---

## Flow 1 — Session Establishment (First Access)

```
User Browser
    │
    │ GET https://app.company.com/dashboard
    ▼
Gateway (Chi HTTP router)
    │
    ├── Middleware: Check __Host-gw_sid cookie
    │     └── MISSING → generate CSRF state token
    │                   → store {app_url} in Redis
    │                   → store {state} in Redis                
    │                   → set __Host-ashrix_state cookie
    │                   → HTTP 302 redirect to:
    │                     CP_URL/authorize/providers?aid=<>&gid=<>
    ▼
Control Plane (auth.ashrix.io)
    │
    ├── Resolve IdP providers linked to app_id
    ├── Return provider selection page to user
    │
    │ User selects IdP (e.g., Okta)
    │
    ├── Create auth state in Redis → {gateway_id}
    ├── HTTP 302 redirect to Okta OIDC authorization endpoint
    ▼
Okta (Identity Provider)
    │
    ├── User authenticates (password + MFA if required)
    ├── HTTP 302 redirect to CP callback:
    │     {URL}/callback, the URL depends on the CP url and it should be configured on the IDP. /callback is the path
    ▼
Control Plane (callback)
    │
    ├── Exchange code for tokens with Okta
    ├── Verify OIDC state against Redis → recover gateway_id
    ├── Extract: user_id, email, groups, MFA level
    ├── Create session in CP Redis
    ├── Issue session ticket:
    │     ticket = 32-byte CSPRNG (base64url)
    │     Redis: SET ticket:<ticket> {user_id, email, groups, app_id} EX 60
    │     (single-use; atomic GET+DEL at redemption)
    ├── HTTP 302 redirect to:
    │     {gateway_url}/_ashrix/auth/callback?state=<ticket>
    ▼
Gateway (/_ashrix/auth/callback)
    │
    ├── Apply rolling 60-second limits: 10 requests per client IP and 100 gateway-wide
    ├── Validate __Host-ashrix_state cookie matches query state (CSRF check)
    ├── Redeem ticket via gRPC ControlStream to CP:
    │     ExchangeTicket(ticket) → {user_id, email, groups, app_id, ...}
    ├── Create gateway-local session in Redis:
    │     session:<tenant_id>:<session_id> → {user_id, email, groups, app_id, expires_at}
    ├── Set-Cookie: __Host-gw_sid=<session_id>; HttpOnly; Secure; SameSite=Lax
    ├── HTTP 302 redirect to original app URL:
    │     https://app.company.com/dashboard
    ▼
Browser receives __Host-gw_sid cookie and follows redirect
```

---

## Flow 2 — Authenticated Request (Subsequent Access)

```
User Browser
    │
    │ GET https://app.company.com/api/data
    │ Cookie: __Host-gw_sid=<session_id>
    ▼
Gateway (Chi HTTP router)
    │
    ├── Middleware: Validate __Host-gw_sid
    │     ├── Redis GET session:<tenant_id>:<session_id>
    │     ├── Session found, not expired → extract {user_id, email, groups, app_id}
    │     └── Slide TTL (extend Redis TTL by idle_timeout, capped at absolute_max)
    │
    ├── Middleware: Policy enforcement
    │     ├── Load active policy bundle for app_id (from memory)
    │     ├── Evaluate rules against {user_id, groups, source_ip, user_agent, ...}
    │     ├── ALLOW → proceed
    │     └── DENY  → HTTP 403 Forbidden
    │
    ├── Middleware: Identity injection
    │     ├── Strip all existing X-Ashrix-* headers from request
    │     └── Inject fresh headers:
    │           X-Ashrix-User-ID:    <user_id>
    │           X-Ashrix-User-Email: <email>
    │           X-Ashrix-App-ID:     <app_id>
    │           X-Ashrix-Request-ID: <trace_id>
    │
    ├── Route to Connector:
    │     ├── Look up connector(s) for app_id in ConnectorRegistry
    │     ├── Select healthy connector
    │     └── Open new QUIC stream on existing mTLS connection
    │
    │ 
    ▼
Connector
    │
    ├── Receive new QUIC stream
    ├── Verify gateway mTLS certificate is valid and not revoked (CRL check)
    ├── Check app_id exists in local app list → if not: reject stream
    ├── Strip all incoming X-Ashrix-* headers from proxied request
    ├── Dial internal_host:internal_port
    ├── Proxy bidirectionally: QUIC stream ↔ TCP to app
    ▼
Internal Application
    │
    ├── Receives request with X-Ashrix-* identity headers
    ├── Processes request
    ├── Returns response
    ▼
Response flows back: App → Connector → QUIC stream → Gateway → Browser
```

---

## Identity Header Stripping

A critical security step occurs here:

**Gateway** strips any `X-Ashrix-*` headers present in the **incoming user request** before it processes them. A user cannot inject their own `X-Ashrix-User-ID` header and have it pass to the application.

```
Incoming user request:
  X-Ashrix-User-ID: attacker-injected-value   ← STRIPPED at gateway

Gateway injects fresh headers after session + policy validation:
  X-Ashrix-User-ID: verified-from-session   ← flows over QUIC to connector

```

---

## Session Expiry and Re-authentication

| Condition | Behaviour |
|---|---|
| Idle timeout (30 min default) | Redis TTL expires; next request → `__Host-gw_sid` not found → restart auth flow |
| Absolute maximum (8 hours, configurable) | Redis TTL hard-expires regardless of activity → restart auth flow |
| Policy change denies user | Policy evaluation fails on next request → HTTP 403 |
| Certificate revocation | Gateway loads CRL → per-call connector check fails → stream terminated |

---

*Back to [Architecture Overview](./architecture-overview.md)*
