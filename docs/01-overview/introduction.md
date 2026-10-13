# Introduction to Ashrix

## What Is Ashrix?

Ashrix is a **Zero Trust Network Access (ZTNA) platform** written in Go. It provides secure, identity-aware access to internal applications without requiring users to join a private network through a traditional VPN.

Instead of granting broad network access, Ashrix grants access to **specific applications** only after verifying who the user is, what device they are using, and whether current policy permits the request. Every access decision is authenticated, authorised, and encrypted — regardless of where the user or the application is located.

---

## The Problem Ashrix Solves

Traditional network access tools (VPNs) solve a connectivity problem, not an access control problem. When a VPN connects a user to a private network, it typically grants access to everything on that network — not just the application the user needs. This creates several operational and security problems:

| Problem | VPN Approach | Ashrix Approach |
|---|---|---|
| **Blast radius** | User can reach any host on the network | User can reach only permitted applications |
| **Credential theft** | Stolen VPN credentials expose the whole network | Stolen session tokens expose at most one session |
| **Remote worker onboarding** | Requires client installation, split-tunnel configuration | Browser-based; no network reconfiguration |
| **Auditability** | Network logs, difficult to map to application actions | Application-level session logs with user identity |
| **Revocation** | Revoking network access is coarse-grained | Session revocation is per-user, per-app, and real-time |

---

## What Ashrix Is Not

Ashrix is a **ZTNA access layer**. It is not:

- A firewall or network security group replacement.
- A WAF (Web Application Firewall).
- An endpoint management agent.
- A VPN replacement for generic TCP/UDP tunnelling.
- A network monitoring or SIEM product.

Ashrix assumes that the underlying network is untrusted. It does not harden the network itself — it makes application access independent of it.

---

## Where Ashrix Fits in a Security Stack

```
[ User Browser / Client ]
        │
        │  HTTPS (public TLS)
        ▼
[ Gateway — tenant-owned, policy enforcement point ]
        │
        │  mTLS gRPC (management) + mTLS QUIC (data)
        ▼
[ Connector — customer network, adjacent to apps ]
        │
        │  localhost / private network
        ▼
[ Protected Application ]
```

The Control Plane sits outside this data path. It manages identity, issues certificates, distributes policy, and tracks sessions — but it never sees application plaintext traffic.

---

## Intended Audiences for This Documentation

1. **A new engineer joining the project** — to understand the codebase, run it locally, debug it, and contribute to it safely.
2. **A technical reviewer** — to evaluate engineering judgment through architectural decisions and their documented trade-offs.

This documentation prioritises accuracy over completeness. Where a feature is designed but not yet implemented, it is labelled accordingly. Unverified behaviour is not presented as a guarantee.

---

## Relationship to Zero Trust

Ashrix adopts the Zero Trust architectural model (NIST SP 800-207). Zero Trust is not a product feature — it is a design philosophy:

> **Never trust, always verify.** No implicit trust is granted based on network location, IP address, or physical proximity.

Ashrix implements this by:

- Requiring identity authentication on every session, via the customer's existing Identity Provider (IdP).
- Enforcing authorisation policy on every request at the gateway.
- Using mutual TLS between all internal components so no component accepts connections from unauthenticated peers.
- Distributing signed, versioned policies that gateways verify cryptographically before applying.

Using OIDC, mTLS, and QUIC does not, by itself, make a system Zero Trust. These are tools. The architecture must apply them consistently and correctly across every access path. This documentation records where Ashrix does so, where it partially does so, and where gaps remain.

---

*Next: [Problem Statement →](./problem-statement.md)*
