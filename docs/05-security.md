

Security Model

1. Overview

Ashrix establishes authenticated outbound QUIC tunnels between a public gateway and connectors deployed inside private networks.

Mutual TLS authenticates the gateway and connector during connection establishment and protects communication in transit.

Ashrix is designed to reduce the need for inbound firewall access to private networks.

It does not replace application authentication, authorization, endpoint security, or organizational network security policies.

2. Security objectives

Ashrix aims to provide:

- Authenticated gateway-to-connector communication.
- Confidentiality and integrity for tunnel traffic.
- Explicit connector identity.
- Explicit application-to-connector routing.
- Restricted connector-side TCP destinations.
- Protection against unauthorized connector connections.
- Bounded resource consumption.
- Safe connection and stream lifecycle handling.

3. Trust boundaries

There are three principal trust boundaries.

External client to gateway

The gateway is exposed to external network traffic.

Its public HTTP/HTTPS listener must be treated as an untrusted input boundary.

The gateway should enforce appropriate request limits, timeouts, and routing validation.

Gateway to connector

The gateway and connector communicate over QUIC with mutual TLS.

The gateway authenticates the connector certificate.

The connector validates the gateway's certificate and expected server identity.

Connector to local application

The connector forwards traffic to explicitly configured local TCP services.

The connector should only dial destinations defined in its local configuration.

It must not allow external requests to select arbitrary IP addresses or ports.

4. Connector authentication

The connector presents a client certificate during the QUIC TLS handshake.

The gateway should validate:

- Certificate chain.
- Certificate validity period.
- Client authentication extended key usage.
- Expected connector identity.
- Connector authorization state.
- Certificate issuer and serial restrictions where implemented.

Connector identity must be derived from the verified TLS peer certificate.

An identity field inside an application message must never override the authenticated identity.

5. Gateway authentication

The connector validates the gateway's certificate.

Validation includes:

- Trusted certificate chain.
- Certificate validity.
- Server authentication usage.
- Expected gateway hostname or server identity.

Disabling certificate verification is not an acceptable production workaround.

6. Application routing

Application routing is configured on the gateway.

The gateway associates a public hostname with a connector and an application name.

The connector maps that application name to a local TCP destination.

This separation is a security boundary.

A public HTTP request must not be able to supply an arbitrary TCP target.

The connector must enforce its own local application mapping even if the gateway is compromised.

7. End-user authentication

Ashrix authenticates connectors, not external users.

It does not automatically provide:

- User identity.
- SSO.
- Multi-factor authentication.
- User-level authorization.
- Device posture verification.
- Application session management.

Applications exposed through Ashrix must implement their own authentication and authorization, or be protected by an appropriate external authentication layer.

An authenticated tunnel does not make an unauthenticated application safe to expose publicly.

8. Network security

The connector initiates outbound QUIC connections.

Ashrix does not require the gateway to initiate an inbound connection into the private network.

The private network must permit outbound UDP traffic to the configured gateway endpoint.

The gateway must be reachable over the configured public network path.

UDP restrictions, NAT behavior, DNS failures, and network instability can affect availability.

9. Resource exhaustion

Both gateway and connector process untrusted network traffic.

Implementations should enforce limits for:

- Concurrent QUIC connections.
- Active streams.
- Concurrent TCP dials.
- Request sizes where applicable.
- Buffered data.
- Connection establishment.
- Idle connections.
- Handshake attempts.
- Request and connection timeouts.

Resource limits must be enforced in the application, not assumed from operating-system limits alone.

Unbounded goroutines and unbounded buffering are unacceptable.

10. Configuration security

Configuration files determine public routing, trusted identities, and private TCP destinations.

Protect them from unauthorized modification.

Use restrictive file permissions and ensure only trusted operators can modify them.

A malicious configuration change could expose unintended services or disrupt application routing.

11. Private-key security

Private keys must remain local to the component that owns them.

The CA private key must be especially protected.

Never:

- Commit private keys to source control.
- Log private-key contents.
- Distribute CA private keys to connectors.
- Include private keys in public release archives.
- Expose private-key paths through public diagnostics unnecessarily.

12. Administrative access

Ashrix uses local configuration and operator-managed PKI operations.

Administrative access to the gateway and connector hosts is therefore security-sensitive.

Protect:

- Operating-system accounts.
- Configuration files.
- PKI commands.
- Certificate and key files.
- Service management permissions.
- Logs and diagnostic data.

13. Threat model

Ashrix is designed to reduce risks associated with direct inbound access to private application networks.

It does not eliminate the following risks:

Threat| Security consideration
Compromised gateway| May affect public routing and tunnel traffic
Compromised connector| May expose services accessible to that connector
Stolen connector key| May allow unauthorized connector authentication until rejected
Malicious configuration| May expose unintended local applications
Compromised application| Remains an application-level security incident
Denial of service| Requires resource limits and operational controls
Blocked outbound UDP| Can prevent tunnel establishment

A compromised connector must be treated as a compromise of the private-network host where it runs.

14. Security limitations

Ashrix is not a complete Zero Trust Network Access platform.

It does not provide centralized user identity, device posture, application-level authorization, or general network segmentation.

Its security boundary is primarily the authenticated gateway-to-connector tunnel and the connector's explicit local destination mappings.

15. Reporting vulnerabilities

Do not publish sensitive vulnerability details in public issues.

Follow the instructions in the repository's "SECURITY.md" file.

Include reproducible steps, affected versions, impact, and relevant logs without exposing private keys or confidential information.

