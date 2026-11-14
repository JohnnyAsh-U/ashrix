


Operations Guide

1. Overview

This guide covers operating an Ashrix deployment after installation.

It includes service management, configuration reloads, certificate maintenance, monitoring, backups, and recovery.

2. Service management

The gateway and connector run as independent processes.

In a systemd deployment, use the service manager to control them.

Typical commands:

sudo systemctl start ashrix-gateway
sudo systemctl stop ashrix-gateway
sudo systemctl restart ashrix-gateway
sudo systemctl status ashrix-gateway

For a connector:

sudo systemctl start ashrix-connector
sudo systemctl stop ashrix-connector
sudo systemctl restart ashrix-connector
sudo systemctl status ashrix-connector

These service names are examples and must match the installed unit files.

3. Configuration changes

Ashrix uses YAML files as its configuration source of truth.

To change configuration:

1. Back up the current configuration.
2. Edit the relevant YAML file.
3. Validate the configuration using the supported validation command, if available.
4. Trigger the supported reload mechanism.
5. Inspect logs and runtime behavior.
6. Confirm that the expected configuration is active.

Do not assume that editing a YAML file automatically changes the running process.

Configuration reload

Reload behavior must be predictable.

A successful reload should activate a complete validated configuration.

A failed reload should retain the previous active configuration.

Operators should understand how reload affects:

- Existing QUIC connections.
- Active application streams.
- Changed application routes.
- Removed connector definitions.
- Certificate paths.
- Listener settings.

Use the actual reload mechanism implemented by Ashrix. Do not assume a signal or CLI command unless it is supported by the installed version.

4. Connector connectivity

A connector should maintain an outbound QUIC connection to its configured gateway.

When connectivity is interrupted:

- The active session becomes unavailable.
- The gateway must stop routing new application requests through that session.
- The connector should clean up the failed connection.
- The connector should attempt reconnection according to its retry policy.

Operators should investigate:

- Gateway availability.
- DNS resolution.
- Outbound UDP restrictions.
- Firewall policies.
- Certificate validity.
- TLS verification errors.
- Network instability.

5. Logs

Logs are important for understanding connection lifecycle and forwarding failures.

Useful events include:

- Gateway startup and shutdown.
- Connector startup and shutdown.
- QUIC connection establishment.
- Connector authentication success or failure.
- Connection loss and reconnection.
- Application routing failures.
- Local TCP connection errors.
- Configuration reload success or failure.
- Certificate validation errors.

Avoid logging private keys, certificate private material, sensitive request content, or credentials.

Use structured logging where possible.

For systemd deployments:

journalctl -u ashrix-gateway -f
journalctl -u ashrix-connector -f

6. Metrics and observability

Monitor the gateway and connector for:

- Active QUIC connections.
- Active application streams.
- Request count and errors.
- Request latency.
- Bytes transmitted and received.
- CPU utilization.
- Memory consumption.
- Goroutine count.
- File descriptor usage.
- Connection establishment and reconnection failures.

Metric labels should remain bounded.

Avoid using connector IDs, client IPs, arbitrary hostnames, or request paths as unrestricted metric labels.

Document the actual metrics endpoint and names supported by each release.

7. Certificate maintenance

Certificates require periodic operator attention.

Operators are responsible for:

- Tracking certificate expiration.
- Protecting private keys.
- Replacing certificates before expiration.
- Updating trust material when necessary.
- Removing compromised identities from the gateway's accepted authorization state.
- Terminating affected active sessions when immediate revocation is required.

Ashrix does not automatically renew or rotate certificates.

See "PKI" (pki.md).

8. Backups

Back up important deployment state.

Gateway

- Gateway YAML configuration.
- CA certificate and protected CA private key.
- Gateway certificate and private key.
- Connector authorization state, if persisted separately.
- Any other persistent identity or PKI state.

Connector

- Connector YAML configuration.
- Connector certificate.
- Connector private key.
- Required CA certificate.
- Any persistent state used by the implementation.

Keep backups protected and encrypted where appropriate.

The CA private key deserves particular protection because compromise may affect the trust of the entire deployment.

9. Recovery

Recovery procedures should be tested, not merely documented.

Gateway recovery

1. Restore the gateway binary.
2. Restore configuration.
3. Restore required certificates and identity state.
4. Verify file ownership and permissions.
5. Start the gateway.
6. Verify that connectors can authenticate.
7. Verify public application routing.

Connector recovery

1. Restore the connector binary.
2. Restore configuration and credentials.
3. Verify certificate and private-key consistency.
4. Start the connector.
5. Verify gateway authentication.
6. Verify application forwarding.

If a private key is compromised, do not simply restore it from backup and continue operating. Replace the affected identity and revoke the compromised certificate according to the PKI procedure.

10. Graceful shutdown

Before planned maintenance:

1. Stop accepting new application traffic where supported.
2. Allow active streams to finish or terminate them according to the configured shutdown policy.
3. Close the QUIC connection cleanly.
4. Stop the process.
5. Perform the maintenance operation.
6. Start the service and verify recovery.

The actual shutdown guarantees depend on the implementation and should be validated with integration tests.

11. Resource monitoring

Monitor resource consumption over time.

Pay attention to:

- Unexpected memory growth.
- Increasing goroutine counts.
- File descriptor exhaustion.
- Excessive active streams.
- Repeated reconnect loops.
- Growing request latency.
- Increased error rates.
- CPU saturation.

If resource usage increases continuously under stable traffic, investigate potential resource leaks or unbounded buffering.

12. Operational checklist

Regularly verify:

- [ ] Gateway is running.
- [ ] Expected connectors are connected.
- [ ] Public applications are reachable.
- [ ] Certificates remain valid.
- [ ] Configuration is backed up.
- [ ] Private keys have restrictive permissions.
- [ ] Logs do not contain repeated unexpected errors.
- [ ] Resource usage is within tested operating limits.
- [ ] Backups can be restored.
- [ ] Updates have been tested before production deployment.

13. Operational principle

Ashrix should remain simple to operate.

Prefer explicit configuration, predictable process behavior, clear logs, bounded resource usage, and tested recovery procedures over introducing a centralized management system.



