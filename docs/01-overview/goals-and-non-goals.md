# Goals and Non-Goals

## Goals

The following goals define the scope of Ashrix v1.x.

### G-1 — Application-Level Access, Not Network-Level Access

Ashrix grants users access to named, registered applications — not to the network the applications live on. A user authorised for `app-jenkins` cannot automatically reach `app-postgres` simply because both are in the same private subnet.

### G-2 — Identity-Aware Access Control

Every session is tied to a verified user identity from the tenant's Identity Provider (IdP). The gateway enforces access policies that reference the user's identity, group membership, MFA status, and device posture at session creation and optionally on each request.

### G-3 — Outbound-Only Connectivity

All data-plane components (gateways and connectors) initiate **outbound** connections to their upstream. Connectors open connections to gateways. Gateways open connections to the Control Plane. No component needs inbound firewall rules opened on the customer's network.

### G-4 — Tenant Traffic Isolation

Ashrix never decrypts enterprise tenant application traffic. The gateway runs in the tenant's infrastructure and terminates TLS locally. The Control Plane only handles authentication metadata, policy, and session tracking — never the payload of proxied requests.

### G-5 — Real-Time Revocation

Session revocation must take effect within a bounded and observable time window. Revocation events are priority messages on the CP-to-gateway channel. The SLA target is **5 seconds** from CP decision to gateway enforcement.

### G-6 — Cryptographic Trust Chain

All inter-component trust is established through a PKI hierarchy anchored at an offline Root CA. Gateways and connectors authenticate with identity certificates issued by an Ashrix-managed Intermediate CA. All policy bundles, revocation events, and rotation commands are signed by the CP and verified by receivers using the bundle public key set in the env.

### G-7 — Operational Resilience

The gateway must continue serving authorised traffic during transient Control Plane unavailability, using locally cached, cryptographically verified policy. If the policy cache becomes too stale (configurable, default `max_valid_until`), the gateway transitions to a fail-closed state that denies new sessions while preserving in-flight ones.

### G-8 — Multi-Tenant SaaS Platform

The Control Plane serves multiple tenants from a single deployment. Each tenant's policy, sessions, certificates, and audit logs are strictly namespaced by `tenant_id`. Data-plane components (gateways and connectors) are single-tenant by design.

---

## Non-Goals

The following are explicitly outside the scope of Ashrix v1.x.

### NG-1 — Full Network Security

Ashrix does not replace firewalls, network ACLs, or host-based security controls. A user with direct network access to a protected application, who bypasses the connector entirely, is outside Ashrix's enforcement perimeter. Ashrix assumes that the internal application is configured to accept connections only from the connector's host.

### NG-2 — Endpoint Security Agent (MVP)

Ashrix does not install an agent on the user's device. Device posture in MVP is passive — derived from HTTP headers (User-Agent, source IP, geo-location). Full endpoint posture (disk encryption status, AV, patch level) is a post-MVP feature.

### NG-3 — Content Inspection

Ashrix is a transparent proxy for authorised traffic. It does not inspect, scan, or filter application payload content. A WAF or DLP tool is a separate, complementary control.

### NG-4 — Universal TCP/UDP Tunnelling

Ashrix does not provide general-purpose tunnelling for arbitrary TCP/UDP traffic. It is designed for application-level access where each application is explicitly registered, named, and governed by policy.

### NG-5 — IdP User Lifecycle Management

Ashrix does not own the user directory. It authenticates users against the tenant's IdP and trusts the IdP's assertions. User provisioning, group management, and directory synchronisation are the IdP's responsibility. Ashrix handles **deprovisioning** via SCIM(to be implemented) for session revocation only.

---

## Design Trade-offs Explicitly Made

| Decision | Trade-off Accepted |
|---|---|
| Separate control plane from data plane | Higher architectural complexity; better availability and traffic isolation |
| Opaque session tokens in Redis | Cannot validate tokens without Redis; enables real-time revocation |
| Policy signed by CP and verified locally | CP is offline-tolerant; policy freshness requires bounded TTL enforcement |
| Connector uses outbound QUIC | No inbound ports; introduces dependency on QUIC library and UDP routing |


---

*Next: [Use Cases →](./use-cases.md)*
