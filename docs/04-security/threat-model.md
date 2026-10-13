# Threat Model

## Purpose

This document identifies the security threats that Ashrix is designed to address, the mitigations in place, and the residual risks that remain. It is not a claim that every mitigation is fully implemented and tested — each threat is annotated with its current implementation status.

A sound threat model requires distinguishing:

1. **The attacker's capabilities.**
2. **The asset being protected.**
3. **The relevant trust boundary.**
4. **The existing mitigation.**
5. **The remaining risk.**
6. **The test that verifies the mitigation.**

---

## Trust Boundaries

| Boundary | Description | Trust Level |
|---|---|---|
| **Internet / User browser** | Untrusted. Users authenticate via IdP; sessions are validated per-request. | Untrusted |
| **Gateway** | Tenant-owned infrastructure. Runs in tenant DMZ. Decrypts user TLS locally. | Tenant-trusted |
| **Connector** | Runs inside tenant private network. Adjacent to internal applications. | Tenant-trusted |
| **Control Plane** | Ashrix-operated SaaS. Never sees application payload. | Ashrix-trusted |
| **Internal applications** | Behind the connector. Should only accept connections from the connector host. | App-trusted (conditional) |

---

## Threat Matrix

### T-01 — Compromised Gateway

| Field | Detail |
|---|---|
| **Attacker capability** | Full control over the gateway process, memory, and disk. |
| **Asset at risk** | User session data, application traffic (plaintext post-TLS termination), connector connections. |
| **Trust boundary crossed** | Gateway → Connector (data plane); Gateway → CP (control stream). |
| **Mitigation** | (1) Connectors authenticate to the gateway via mTLS — a compromised gateway cannot impersonate a connector. (2) Session data encrypted at rest in Redis (optional). (3) Anomalous gateway behaviour is detectable via CP heartbeat monitoring. |
| **Residual risk** | A compromised gateway can drop, log, or modify application traffic it is already proxying. It can also reject or disconnect connectors, causing a DoS for that tenant. |
| **Status** | ⚙️ Partial — connector signature verification implemented; gateway anomaly detection is 📐 designed. |

---

### T-02 — Compromised Connector

| Field | Detail |
|---|---|
| **Attacker capability** | Full control over the connector process, host, and adjacent network. |
| **Asset at risk** | Internal applications reachable from the connector host. |
| **Trust boundary crossed** | Connector → Internal Application. |
| **Mitigation** | (1) App list enforcement — connector only proxies to registered apps. (2) Host firewall rules should restrict connector outbound to only its app list destinations. (3) App list is controlled by the CP, not the connector. (4) A compromised connector cannot impersonate the gateway (mTLS required). |
| **Residual risk** | The compromised connector can reach any host/port accessible from its host, not just the app list entries (unless host firewall is configured). The blast radius is determined by the connector host's network position. |
| **Status** | ⚙️ Partial — app list enforcement implemented; host firewall configuration is an operational responsibility. |

---

### T-03 — Stolen Session Cookie

| Field | Detail |
|---|---|
| **Attacker capability** | Cookie value obtained via XSS, physical access, or network interception. |
| **Asset at risk** | Access to the application for the session's remaining TTL. |
| **Trust boundary crossed** | User browser → Gateway. |
| **Mitigation** | (1) `__Host-` cookie prefix prevents subdomain injection. (2) `HttpOnly` flag prevents JavaScript access. (3) `Secure` flag prevents transmission over HTTP. (4) `SameSite=Lax` reduces CSRF risk. (5) Sessions are revocable: Redis `DEL session:<id>` or CP-pushed revocation event. |
| **Residual risk** | An attacker who has obtained the cookie value (via XSS exploiting a different vulnerability) can use it until it expires or is revoked. Revocation SLA is ≤5 seconds once the admin initiates it. |
| **Status** | ✅ Implemented — cookie properties; ✅ Implemented — session revocation via Redis DEL. |

---

### T-04 — Stolen Bootstrap Token

| Field | Detail |
|---|---|
| **Attacker capability** | Intercepts or steals the bootstrap token before the legitimate node uses it. |
| **Asset at risk** | Ability to enroll a malicious node as a legitimate gateway or connector and receive a real identity certificate. |
| **Trust boundary crossed** | Token delivery (out-of-band) → CP enrollment endpoint. |
| **Mitigation** | (1) Tokens are single-use — consumed atomically at first use via DB unique constraint. (2) TTL of 5 minutes from issuance. (3) Token is bound to a specific `node_id` — cannot be used for a different node. (4) Token is stored as `hash(token)` — plaintext never persisted. (5) Every token generation and use is a security audit event. (6) Optional IP binding (V2). |
| **Residual risk** | If the attacker uses the token before the legitimate node, the legitimate node cannot bootstrap (token consumed). The admin must detect and regenerate. The attacker has a valid certificate for the `node_id` — this must be revoked immediately when detected. |
| **Status** | ✅ Implemented — single-use, TTL, node binding, bcrypt storage. |

---

### T-05 — Stolen Identity Certificate (Gateway or Connector)

| Field | Detail |
|---|---|
| **Attacker capability** | Extracts the private key from the gateway or connector host disk. |
| **Asset at risk** | Ability to impersonate a legitimate gateway or connector. |
| **Trust boundary crossed** | mTLS authentication between components. |
| **Mitigation** | (1) Private keys are stored AES-256-GCM encrypted on disk. (2) Keys are loaded into a `crypto.Signer` abstraction after decryption — raw key bytes are discarded. (3) 90-day certificate validity limits the window of usefulness of a stolen key. (4) Certificate revocation via CRL, propagated with ≤5 second SLA. (5) Future upgrade path to Vault Agent / Cloud KMS hardware-backed keys. |
| **Residual risk** | An attacker with root on the host can obtain the AES key and decrypt the private key. Without hardware-backed key storage, software-only disk encryption is the last line of defence. |
| **Status** | ✅ Implemented — AES-256-GCM encrypted keys. |

---

### T-06 — Policy Downgrade Attack

| Field | Detail |
|---|---|
| **Attacker capability** | Network attacker who can intercept or replay gRPC messages; or compromised CP operator. |
| **Asset at risk** | Authorization policy — granting access that should have been revoked. |
| **Trust boundary crossed** | CP → Gateway (control stream). |
| **Mitigation** | (1) Policy bundles have a monotonic version. Gateway rejects any bundle with `version <= current_version`. (2) ECDSA signature on every bundle — a tampered bundle will fail signature verification. (3) Version check is performed both before and after signature verification. |
| **Residual risk** | If the CP signing key is compromised, an attacker could issue a valid downgrade bundle. |
| **Status** | ✅ Implemented — monotonic version check + signature verification. |

---

### T-07 — Bundle Withholding (Stale Policy Attack)

| Field | Detail |
|---|---|
| **Attacker capability** | Network attacker who blocks policy updates (e.g., isolating the gateway from the CP). |
| **Asset at risk** | Access continues to be granted based on a policy that the CP has since changed to deny. |
| **Trust boundary crossed** | CP → Gateway (control stream). |
| **Mitigation** | Every gateway has a MAX_OFFLINE_PERIOD. If this period is exceeded, the gateway shuts down and denies all new access requests. |
| **Residual risk** | . |
| **Status** | ✅ Implemented |

---

### T-08 — Gateway Resource Exhaustion (DoS)

| Field | Detail |
|---|---|
| **Attacker capability** | High-volume connection or request flooding. |
| **Asset at risk** | Gateway availability — all users in the tenant lose access. |
| **Trust boundary crossed** | Internet → Gateway. |
| **Mitigation** | (1) Rate limiting on `/_ashrix/auth/callback` (10 req/min per IP, 100 req/min global). (2) Max concurrent gRPC streams per gateway_id. |
| **Residual risk** | Layer-3/4 volumetric DDoS is outside Ashrix's scope; an upstream DDoS mitigation service is required. |
| **Status** | Implemented. |

---

### T-09 — Man-in-the-Middle (User → Gateway)

| Field | Detail |
|---|---|
| **Attacker capability** | Network position between user and gateway. |
| **Asset at risk** | User session data, application traffic in transit. |
| **Mitigation** | (1) HTTPS with public CA certificate (2) TLS 1.3 minimum. |
| **Status** | ✅ Implemented. |

---

### T-10 — Man-in-the-Middle (Gateway → Connector)

| Field | Detail |
|---|---|
| **Attacker capability** | Network position between gateway and connector. |
| **Asset at risk** | Application traffic, management commands. |
| **Mitigation** | (1) mTLS — both sides present and verify identity certificates. (2) Per-call CRL check. (3) All management payloads are CP-signed and verified independently. |
| **Status** | ✅ Implemented — mTLS; ✅ Implemented — per-call CRL. |

---

### T-11 — Break-Glass Endpoint DoS

| Field | Detail |
|---|---|
| **Attacker capability** | Knows valid `node_id` values (enumerable from public DNS or leaked config). |
| **Asset at risk** | CP availability for legitimate certificate renewals and recoveries. |
| **Mitigation** | (1) Max 5 recovery attempts per `node_id` per hour. (2) Exponential backoff on consecutive failures (1s, 2s, 4s, 8s, 16s). (3) IP-level rate limit (20 req/min per source IP). (4) 2 failed attempts in 10 minutes triggers security alert. |
| **Status** | 📐 Designed — rate limiting controls specified; implementation requires verification. |

---

### T-12 — Forged CP Command (via Compromised Gateway)

| Field | Detail |
|---|---|
| **Attacker capability** | Full control over the gateway process. |
| **Asset at risk** | Connector behaviour — attacker sends forged revocation or rotate command. |
| **Mitigation** | - |
| **Residual risk** | -  |
| **Status** | Pending. |

---

## Explicitly Out-of-Scope Threats

| Threat | Why Out of Scope |
|---|---|
| Vulnerability in IdP (Okta, Azure AD) | Ashrix trusts the IdP as an external root of trust. IdP security is the tenant's and IdP's responsibility. |
| Physical access to connector host | Ashrix cannot prevent physical attack on tenant hardware. Host OS security and disk encryption at the OS level are separate controls. |
| Vulnerability in QUIC or gRPC libraries | Ashrix depends on upstream Go libraries (`quic-go`, `grpc-go`). Library security is monitored via dependency updates and CVE scanning. |
| Application vulnerability | Ashrix enforces access at the application entry point. SQL injection, XSS, and other app-level vulnerabilities within the protected application are the application developer's responsibility. |

---

*Next: [Authentication →](./authentication.md)*
