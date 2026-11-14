# Ashrix Architecture

## 1. Overview

Ashrix is a self-hosted application access gateway that connects private TCP services to a public gateway through outbound QUIC tunnels.

The architecture consists of three main elements:

- Gateway
- Connector
- PKI

The gateway and connector are independent processes. They communicate over an authenticated QUIC connection initiated by the connector.

Configuration is maintained locally through YAML files. There is no centralized control plane or configuration database.

## 2. Architecture

                       External Clients
                              |
                              | HTTPS
                              |
                    +-------------------+
                    |                   |
                    |   Ashrix Gateway  |
                    |                   |
                    |  - HTTP routing   |
                    |  - QUIC server    |
                    |  - mTLS auth      |
                    |  - Session state  |
                    +-------------------+
                              |
                              |
                    QUIC connection
                    initiated by
                    the connector
                              |
                    +-------------------+
                    |                   |
                    | Ashrix Connector  |
                    |                   |
                    |  - QUIC client    |
                    |  - Stream handler |
                    |  - TCP forwarding |
                    +-------------------+
                              |
                              |
                         Local TCP
                              |
                    +-------------------+
                    | Private Services  |
                    +-------------------+

## 3. Gateway

The gateway is the public-facing component.

Its responsibilities include:

### HTTP reverse proxy

- Accept incoming application requests.
- Match incoming hostnames against configured application routes.
- Identify the connector associated with the application.
- Forward the request through the connector's active QUIC session.
- Return the response to the external client.

### QUIC server

- Accept connector QUIC connections.
- Perform TLS authentication.
- Verify connector identity.
- Manage active connector sessions.
- Handle incoming application streams.
- Detect and clean up disconnected sessions.

### Configuration

The gateway loads its configuration from a local YAML file.

Configuration defines the connector identities and application routing relationships supported by the implementation.

Configuration changes are applied through the supported reload mechanism.

## 4. Connector

The connector operates inside the private network.

Its responsibilities include:

### Outbound connection

- Resolve and connect to the configured gateway.
- Establish a QUIC connection.
- Authenticate using its client certificate.
- Verify the gateway's TLS identity.
- Reconnect after connection failures.

### Stream forwarding

For each application stream:

1. Accept the incoming QUIC stream.
2. Identify the configured local application destination.
3. Establish a TCP connection to that destination.
4. Forward traffic bidirectionally.
5. Propagate cancellation and close resources when forwarding ends.

The connector must only dial destinations explicitly defined in its local configuration.

## 5. Traffic flow

An external request follows this path:

1. A client resolves the application's public hostname.
2. The client establishes an HTTPS connection to the gateway.
3. The gateway matches the hostname to an application route.
4. The gateway identifies the associated connector.
5. The gateway opens an application stream over the connector's QUIC session.
6. The connector accepts the stream.
7. The connector resolves the application name against its local configuration.
8. The connector establishes a TCP connection to the configured destination.
9. Data flows bidirectionally through the tunnel.
10. The response returns through the same path.

## 6. Connection lifecycle

The connector initiates the QUIC connection.

```
Connector starts
      |
      v
Load configuration
      |
      v
Load certificate
      |
      v
Connect to gateway
      |
      v
QUIC + mTLS handshake
      |
      v
Gateway authenticates connector
      |
      v
Session established
      |
      v
Application streams
      |
      v
Connection lost?
   /       \
 No         Yes
 |           |
 v           v
Continue   Cleanup session
             |
             v
         Reconnect
```


The gateway must not treat a disconnected connector as available for new requests.

The connector's reconnection strategy must avoid uncontrolled retry loops and resource leaks.

## 7. QUIC streams

QUIC provides independent streams within a connection.

Ashrix uses streams to carry application traffic between the gateway and connector.

Streams are independently ordered, but share connection-level resources such as congestion control and connection flow control.

Consequently, a large transfer can affect other streams sharing the same connection. Backpressure, buffering, stream limits, and cancellation are important parts of the implementation.

## 8. Configuration ownership

Configuration is deliberately split between gateway and connector.

Gateway configuration

The gateway owns:

- Public application hostnames.
- Connector association.
- Application routing.
- Connector authorization settings supported by the implementation.
- Gateway listener configuration.

### Connector configuration

The connector owns:

- Connector identity.
- Gateway address and trust settings.
- Local application names.
- Local TCP destinations.
- Connector-specific transport settings.

The gateway must not be able to use a public request to select an arbitrary local TCP destination.

The connector is responsible for enforcing its local destination mapping.

## 9. PKI and trust

The gateway and connector authenticate one another using TLS certificates.

- The connector validates the gateway's server identity.
- The gateway validates the connector's client certificate.
- Connector identity is derived from the authenticated certificate.
- Private keys remain local to their respective components.

Certificate issuance and replacement are operator-managed.

See [PKI](pki.md) and [Security](security.md).

## 10. Failure handling

The implementation should account for:

- Gateway unavailability.
- Connector process termination.
- Network interruption.
- QUIC connection closure.
- QUIC stream reset.
- Local TCP connection failure.
- Client request cancellation.
- Gateway and connector shutdown.
- Invalid configuration during reload.

A failed connector session must be cleaned up before it can be replaced by a new active session.

Existing streams must have well-defined cancellation and cleanup behavior.

## 11. Design principles

Ashrix follows these principles:

- Outbound connectivity from private networks.
- Small, independently deployable components.
- Explicit YAML configuration.
- Mutual TLS for connector authentication.
- Local control of TCP destinations.
- Bounded resource consumption.
- Minimal external dependencies.
- No centralized control plane.

## 12. Non-goals

Ashrix is not intended to be:

- A general-purpose VPN.
- A complete Zero Trust Network Access platform.
- An identity provider.
- An SSO system.
- A device posture platform.
- A centralized fleet management platform.
- A distributed configuration service.
- A general-purpose TCP or UDP proxy.

The architecture intentionally focuses on application access through outbound QUIC tunnels.


