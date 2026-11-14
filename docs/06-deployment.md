


Deployment Guide

1. Overview

This guide describes deploying Ashrix with a public gateway and one or more connectors inside private networks.

The gateway accepts external application traffic. Connectors establish outbound QUIC connections to the gateway and forward traffic to locally configured TCP applications.

2. Deployment topology

A typical deployment looks like:

             Public Internet
                    |
                    |
             +--------------+
             | Gateway Host |
             |              |
             | HTTPS        |
             | QUIC / UDP   |
             +--------------+
                    |
              Outbound QUIC
                connection
                    |
             +--------------+
             | Private Host |
             |              |
             | Connector    |
             +--------------+
                    |
                 TCP
                    |
             +--------------+
             | Application  |
             +--------------+

The gateway must be reachable by external clients and connectors.

The connector only needs outbound connectivity to the gateway. No inbound tunnel connection is required.

3. Requirements

Gateway host

- Supported Linux distribution and architecture.
- Publicly reachable network endpoint.
- Public DNS name for exposed applications.
- Required TCP and UDP ports permitted by the host firewall and upstream network.
- Gateway certificate and private key.
- Trusted CA certificate for connector authentication.

Connector host

- Supported Linux distribution and architecture.
- Outbound network access to the gateway.
- Connector certificate and private key.
- Trusted CA certificate for gateway verification.
- Network access to the configured local applications.

Network requirements

At minimum, the deployment needs:

- External clients to reach the gateway's HTTPS listener.
- Connectors to reach the gateway's QUIC listener over UDP.
- Connectors to reach their configured local TCP destinations.

The gateway does not need an inbound connection to the private network.

Corporate firewalls may block outbound UDP. Verify this before deployment.

4. Install Ashrix

Download the gateway and connector binaries from the official GitHub Releases.

Alternatively, use the installation scripts provided by the project.

Verify release checksums before installing binaries.

Install the gateway binary on the public host and the connector binary on the private host.

The exact supported architectures and installation commands should be taken from the release documentation.

5. Directory layout

A typical deployment may use:

Gateway:

/etc/ashrix/
├── gateway.yaml
├── certs/
│   ├── gateway.crt
│   └── gateway.key
└── pki/
    └── ca.crt

/var/lib/ashrix/

Connector:

/etc/ashrix/
├── connector.yaml
├── certs/
│   ├── connector.crt
│   └── connector.key
└── pki/
    └── ca.crt

/var/lib/ashrix/

These are suggested paths. Adjust them to the actual implementation and service configuration.

Private keys should be readable only by the service account or privileged operator that needs them.

6. Initialize the PKI

Initialize the Ashrix CA using the supported PKI command.

Protect the CA private key.

Distribute only the CA certificate to the components that need to verify certificates.

See "PKI and Certificate Management" (pki.md) for the complete process.

7. Prepare the gateway

1. Install the gateway binary.
2. Install the gateway server certificate and private key.
3. Configure the trusted client CA.
4. Configure the QUIC listener.
5. Define authorized connectors.
6. Define public application routes.
7. Validate the YAML configuration.
8. Start the gateway.

8. Prepare a connector

1. Install the connector binary.
2. Generate a private key locally.
3. Generate a CSR.
4. Obtain an issued connector certificate.
5. Install the certificate and CA certificate.
6. Configure the gateway endpoint.
7. Configure local TCP destinations.
8. Validate the connector configuration.
9. Start the connector.

The connector must verify the gateway's server identity during the TLS handshake.

9. Verify connectivity

After starting both components, verify:

- The gateway starts without configuration or certificate errors.
- The connector resolves the gateway hostname.
- The connector can reach the gateway's UDP listener.
- The QUIC TLS handshake succeeds.
- The gateway authenticates the connector.
- The connector establishes an active session.
- The configured application can be reached through the gateway.

If the tunnel cannot be established, consult "Troubleshooting" (troubleshooting.md) when available.

10. DNS and HTTPS

Configure public DNS records to point application hostnames to the gateway.

The gateway must have valid HTTPS certificate coverage for the public application hostnames.

The gateway's public HTTPS certificate and its connector-authentication CA serve different trust purposes. Do not confuse application-facing TLS with gateway-to-connector mTLS.

11. systemd deployment

For production Linux deployments, run the gateway and connector as managed systemd services.

A service definition should:

- Run the correct binary and configuration path.
- Use a dedicated unprivileged service account where possible.
- Restrict access to credentials.
- Restart after unexpected process failure.
- Support graceful shutdown.
- Use appropriate filesystem and process restrictions.

Example service definitions should be added to the repository only after testing them against the actual binary flags, paths, and shutdown behavior.

12. Updating Ashrix

Before updating:

1. Read the release notes.
2. Back up configuration and PKI material.
3. Download the new version.
4. Verify checksums and signatures where available.
5. Stop or safely upgrade the relevant service.
6. Install the new binary.
7. Start the service.
8. Verify connectivity and application access.

Keep a known-good binary available for rollback.

13. Uninstallation

A complete uninstall should:

- Stop and disable the relevant service.
- Remove the binary.
- Remove service definitions.
- Remove configuration only when explicitly requested.
- Preserve PKI material and application data unless the operator deliberately chooses to delete them.

Do not silently delete private keys or CA backups during an uninstall.

14. Deployment considerations

Before exposing a production application:

- Confirm the application has its own authentication.
- Review public routing.
- Restrict access to the gateway host.
- Protect private keys.
- Verify network requirements.
- Establish monitoring and backup procedures.
- Test reconnection and failure behavior.



