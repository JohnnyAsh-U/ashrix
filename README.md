# Ashrix

Expose private applications without opening inbound firewall ports.

Ashrix is a lightweight, self-hosted application access gateway written in Go. It connects private applications to a public gateway through outbound QUIC tunnels secured with mutual TLS (mTLS).

No inbound tunnel connections. No centralized control plane. Just a gateway, connectors, YAML configuration, and certificates.

How it works

       External Client
              |
              | HTTPS
              v
       +--------------+
       |    Gateway   |
       +--------------+
              |
         QUIC + mTLS
       (outbound tunnel)
              |
       +--------------+
       |  Connector   |
       +--------------+
              |
           TCP
              |
       +--------------+
       | Private App  |
       +--------------+

The connector initiates the tunnel from inside the private network. The gateway uses that connection to forward external application traffic to locally configured services.

Features

- Outbound QUIC tunnels.
- Mutual TLS connector authentication.
- HTTP application routing.
- TCP application forwarding.
- YAML-based configuration.
- Lightweight Go binaries.
- Self-hosted deployment.
- No centralized control plane.

Quick start

This example assumes you have already installed the gateway and connector binaries and provisioned their certificates.

1. Configure the gateway

Create "/etc/ashrix/gateway.yaml":

gateway:
  listen: ":443"

quic:
  listen: ":8443"

tls:
  certificate: /etc/ashrix/certs/gateway.crt
  private_key: /etc/ashrix/certs/gateway.key
  client_ca: /etc/ashrix/pki/ca.crt

connectors:
  - id: office-01
    enabled: true

applications:
  - name: grafana
    domain: grafana.example.com
    connector: office-01
    target_name: grafana

2. Configure the connector

Create "/etc/ashrix/connector.yaml":

connector:
  id: office-01

gateway:
  address: gateway.example.com:8443
  server_name: gateway.example.com

identity:
  certificate: /etc/ashrix/certs/connector.crt
  private_key: /etc/ashrix/certs/connector.key
  ca_certificate: /etc/ashrix/pki/ca.crt

applications:
  - name: grafana
    target: 192.168.1.10:3000

3. Start the gateway

ashrix-gateway -config /etc/ashrix/gateway.yaml

4. Start the connector

ashrix-connector -config /etc/ashrix/connector.yaml

5. Access your application

Configure DNS:

grafana.example.com -> YOUR_GATEWAY_PUBLIC_IP

Open:

https://grafana.example.com

The gateway forwards the request through the connector's established QUIC tunnel to the local Grafana service.

Important: The YAML fields and CLI flags above are illustrative. Use the exact schema and commands supported by the current release.

Installation

Download the appropriate Linux binaries from "GitHub Releases" (https://github.com/ashrixlabs/ashrix/releases).

Verify the release checksums before installation.

See "Deployment" (docs/deployment.md) for complete installation instructions.

Requirements

- Linux gateway host.
- Linux connector host.
- Publicly reachable gateway.
- Outbound UDP connectivity from connector to gateway.
- Valid gateway and connector certificates.
- DNS pointing application hostnames to the gateway.

Documentation

Document| Description
"Architecture" (docs/architecture.md)| How Ashrix works
"Configuration" (docs/configuration.md)| Gateway and connector YAML
"PKI" (docs/pki.md)| Certificate management
"Security" (docs/security.md)| Security model and limitations
"Deployment" (docs/deployment.md)| Installation and deployment
"Operations" (docs/operations.md)| Operating and maintaining Ashrix
"Benchmarking" (docs/benchmarking.md)| Performance testing methodology

Development

Build:

go build ./...

Test:

go test ./...

Race detection:

go test -race ./...

See "Development" (docs/development.md) for contributor instructions.

Security

Ashrix authenticates connectors using mTLS. It does not provide end-user authentication, SSO, or application-level authorization.

Your applications remain responsible for their own access control.

For security reports, see "SECURITY.md" (SECURITY.md).

Project status

Ashrix is an open-source infrastructure project under active development.

The focus is reliable tunneling, secure application forwarding, predictable resource consumption, and straightforward deployment.

License

See "LICENSE" (LICENSE).


