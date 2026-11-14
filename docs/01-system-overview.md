# Ashrix

## Expose on-premises applications without opening inbound firewall ports.

Ashrix is a lightweight, self-hosted application access gateway built with Go. It connects private applications to a public gateway through outbound QUIC tunnels secured with mutual TLS (mTLS).

Instead of exposing internal services directly or configuring inbound port forwarding, an Ashrix connector establishes an outbound connection to the gateway. The gateway uses that existing connection to forward incoming application traffic to services inside the private network.

## Why Ashrix?

Deploying applications on-premises often creates a networking problem: making those applications accessible externally can require firewall changes, NAT configuration, port forwarding, VPN infrastructure, or additional network administration.

Ashrix offers a simpler connectivity model.

- No inbound tunnel connection into the private network.
- No inbound firewall port forwarding required for the connector.
- Outbound QUIC connectivity between gateway and connector.
- Mutual TLS authentication for connector connections.
- YAML-based configuration.
- Lightweight Go binaries.
- Self-hosted deployment with no external control plane.

The private network must still permit the connector to reach the gateway over the configured outbound network path. QUIC requires UDP connectivity.

## How it works

                 Internet
                    |
              HTTPS Request
                    |
             +--------------+
             | Ashrix       |
             | Gateway      |
             +--------------+
                    |
             Existing outbound
               QUIC tunnel
                    |
             +--------------+
             | Ashrix       |
             | Connector    |
             +--------------+
                    |
              Local TCP
                    |
             +--------------+
             | Private      |
             | Application  |
             +--------------+

The gateway accepts public application traffic and routes it through an authenticated QUIC connection to the appropriate connector. The connector forwards the traffic to a locally configured TCP service.

## Components

### Gateway

The gateway is the public-facing component.

Responsibilities include:

- Accepting incoming HTTP/HTTPS traffic.
- Managing authenticated QUIC connections.
- Associating connectors with configured applications.
- Forwarding requests through QUIC streams.
- Handling connector connection lifecycle.

### Connector

The connector runs inside the private network.

Responsibilities include:

- Establishing an outbound QUIC connection to the gateway.
- Authenticating using its mTLS certificate.
- Accepting application streams from the gateway.
- Forwarding traffic to explicitly configured local TCP destinations.
- Reconnecting after connection failures.

### PKI

Ashrix includes a small PKI workflow for establishing trust between gateway and connectors.

Certificates authenticate connectors during the QUIC TLS handshake. Private keys remain with their respective owners.

Certificate issuance, installation, replacement, and revocation are operator-managed processes.

## Security model

Ashrix protects the gateway-to-connector communication channel using mutual TLS.

It does not automatically provide end-user authentication, SSO, device posture checks, or application-level authorization. Applications remain responsible for their own user authentication and authorization, or must be protected by an appropriate external authentication layer.

Ashrix is not a general-purpose VPN and does not provide arbitrary network access.

## Installation

Download the appropriate gateway and connector binaries from the project's GitHub Releases.

See the "deployment guide" (docs/deployment.md) for installation, service configuration, and setup instructions.

## Quick start

A typical deployment involves:

1. Install the gateway on a publicly reachable Linux host.
2. Initialize the PKI and establish certificate trust.
3. Generate a connector private key and certificate request.
4. Issue and install the connector certificate.
5. Configure the gateway using "gateway.yaml".
6. Configure the connector using "connector.yaml".
7. Start the gateway and connector.
8. Access the configured application through its public hostname.

See:

- [Configuration](docs/configuration.md)
- [PKI and certificates](docs/pki.md)
- [Deployment](docs/deployment.md)

Documentation

| Document | Description |
| --- | --- |
|[Architecture](docs/architecture.md)| System components and traffic flow |
|[Configuration](docs/configuration.md)| Gateway and connector YAML
[PKI](docs/pki.md)| Certificate issuance and management
[Security](docs/security.md)| Trust boundaries and security considerations
[Deployment](docs/deployment.md)| Installation and deployment
[Operations](docs/operations.md)| Running, reloading, monitoring, and recovery

## Requirements

- Linux gateway host.
- Linux host inside the private network for the connector.
- A network path permitting outbound UDP from connector to gateway.
- Public DNS and HTTPS connectivity for applications exposed through the gateway.
- Valid certificates and trusted CA configuration.


Specific supported Linux distributions, architectures, and resource requirements should be confirmed against the current release.

## Project status

Ashrix is an actively developed open-source project.

The project prioritizes:

- Correct QUIC connection and stream lifecycle management.
- Reliable bidirectional TCP forwarding.
- Secure connector authentication.
- Predictable resource consumption.
- Failure recovery.
- Reproducible performance testing.
- Simple deployment and operation.

Check the project's releases and issue tracker for current implementation status and known limitations.

## Contributing

Contributions, bug reports, documentation improvements, and reproducible test cases are welcome.

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security issues

Please do not disclose suspected vulnerabilities in public issues. Follow the reporting instructions in "SECURITY.md" (SECURITY.md).

## License

See [LICENSE](../LICENSE).
