# Identity and Access

## Separate Responsibilities

Ashrix access decisions involve three distinct responsibilities:

| Responsibility | Question | Ashrix component |
|---|---|---|
| Authentication | Who is making the request? | Control Plane identity flow and gateway session establishment |
| Authorization | Is this identity permitted to use this application? | Gateway policy evaluation |
| Enforcement | Does the request proceed to the application? | Gateway proxy and connector routing |

A valid identity or session is not, by itself, permission to use every application.


## Authorization and Enforcement

The gateway evaluates the local policy associated with the requested application and session. A deny decision must stop forwarding. For an allow decision, the gateway chooses an eligible connector and forwards the request through the data-plane tunnel.

The connector also restricts forwarding to its configured application list. This is defense in depth, not a substitute for gateway authorization: a compromised gateway or connector host can change the risk, and network-level egress restrictions remain important.

## Machine Identity

Gateway, connector, and Control Plane communication uses mutual TLS in the documented design. Machine certificates authenticate components to one another; they do not represent the end user's application authorization. Certificate issuance, trust, renewal, and revocation are covered in [mTLS](../04-security/mtls.md).

## Boundaries and Gaps

- The identity provider is an external trust dependency.
- Authorization correctness depends on policy content and on all application routes passing through enforcement.
- Session revocation and policy propagation have latency and outage behavior that require tests and operational measurements.

---

*Next: [Request Lifecycle](./request-lifecycle.md)*