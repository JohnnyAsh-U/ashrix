# Ashrix

Expose private applications without opening inbound firewall ports.

Ashrix is a lightweight, self-hosted application access gateway written in Go. It connects private applications to a public gateway through outbound QUIC tunnels secured with mutual TLS (mTLS).

No inbound tunnel connections. No centralized control plane. Just a gateway, connectors, YAML configuration, and certificates.

## How it works

```
External Client

HTTPS
v
+--------------+
|   Gateway    |
+--------------+

QUIC + mTLS (outbound tunnel)
v
+--------------+
|  Connector   |
+--------------+

TCP
v
+--------------+
| Private App  |
+--------------+
```

The connector initiates the tunnel from inside the private network. The gateway uses that connection to forward external application traffic to locally configured services.

## Features

- Outbound QUIC tunnels.
- Mutual TLS connector authentication.
- HTTP application routing.
- TCP application forwarding.
- YAML-based configuration.
- Lightweight Go binaries.
- Self-hosted deployment.
- No centralized control plane.

## Quick start

This is the full flow from empty machines to a working app.

### 1. On the gateway: initialize the CA and gateway identity

```bash
ashrix-gateway init --name gateway-01 --server-name gateway.example.com
```
This generates:

- Gateway config: `~/.ashrix/gateway.yaml`
- Root CA certificate: `~/.ashrix/pki/ca_cert.crt`
- Root CA private key: `~/.ashrix/pki/ca_key.key`
- Gateway leaf certificate: `~/.ashrix/pki/gateway.crt`
- Gateway leaf private key: `~/.ashrix/pki/gateway.key`

You do not need to hand-write the gateway YAML or generate certificates with `openssl`. `init` does it for you.

To inspect the CA you just created:
```
ashrix-gateway show-ca
```
This shows the CA subject, serial, validity, and paths, so you can verify what you are about to trust.



### 2. On the connector: initialize the connector
```bash
ashrix-connector init --name office-01 --quic-addr gateway.example.com:8443 --server-name gateway.example.com --out ~/path/to/office-01.csr
```

This generates:

- Connector config: `~/.ashrix/connector/connector.yaml`
- Connector private key: `~/.ashrix/connector/connector.key`
- Connector certificate signing request: the path supplied with `--out`

The connector does not have a signed certificate yet. It only has a key and a CSR to be signed by the gateway CA.


### 3. On the gateway: sign the connector and register it

Copy the CSR from the connector to the gateway, then run:

```bash
ashrix-gateway add-connector --csr /path/to/office-01.csr --out /path/to/office-01-signed.crt
```

This does three things:

1. Signs the CSR with the gateway CA.
2. Adds the connector to the gateway configuration file (`~/.ashrix/gateway.yaml`).
3. Creates the signed certificate bundle for that connector at the path supplied with `--out`.

The bundle contains the connector leaf certificate plus the root CA, so the connector can trust the gateway and prove its own identity.


### 4. On the connector: install the signed certificate

Copy the signed certificate bundle back to the connector, then run:
```bash
ashrix-connector install --cert /path/to/connector-signed.crt
```
This installs the root CA and the connector node certificate into the paths configured in the connector YAML. After this step, the connector has everything it needs to authenticate to the gateway with mTLS.

### 5. Add the application to the gateway config

Edit `~/.ashrix/gateway.yaml` and add the application:
```yaml
applications:
  - name: grafana
    domain: grafana.example.com
    connector: office-01
```

Rules for routing:

1. The application must reference a connector that exists in the gateway config. `applications[].connector` must equal an `id` in `connectors[]`.
2. The application name in the gateway must match the application name in the connector. Gateway `name: grafana` only routes if the connector also has `name: grafana`.

If either rule fails, there is no route. The gateway will not fall back to another connector or a default target.


### 6. Add the application to the connector config

Edit `~/.ashrix/connector/connector.yaml` and add the same application name with its local destination:

```yaml
applications:
  - name: grafana
    target: 192.168.1.10:3000
```

The gateway never defines a TCP destination. Only the connector maps an application name to a local `target`. The connector must never accept an arbitrary destination from an external request.



### 7. Start both sides

On the gateway:
```bash
ashrix-gateway start
```
On the connector:
```bash
ashrix-connector start
```

The connector dials out to the gateway over QUIC + mTLS. No inbound port needs to be opened on the connector network.


### 8. Access your application

Configure DNS:
```yaml
grafana.example.com -> YOUR_GATEWAY_PUBLIC_IP
```
Open:

```
https://grafana.example.com
```

The gateway forwards the request through the connector's established QUIC tunnel to the local Grafana service.


### 9. For HTTPS On the gateway: provide the public HTTPS certificate

`init` does *not* create your public HTTPS certificate. That certificate is what browsers validate when they open `https://grafana.example.com`.

You must put the HTTPS certificate and private key in the exact paths configured in the gateway YAML under 

`listeners.https`:
```
listeners:
  https:
    address: "0.0.0.0:443"
    certificate: ~/.ashrix/server.crt
    private_key: ~/.ashrix/server.key

```

In the example file `examples/gateway.yaml`, those paths are:

certificate: /etc/ashrix-gateway/certs/server.crt

private_key: /etc/ashrix-gateway/certs/server.key


So for a user-directory install, place your files at:
~/.ashrix/server.crt
~/.ashrix/server.key

For a system install, place them at the paths in your `gateway.yaml`.

You can get them from any source you trust, for example Let's Encrypt, your company CA, or a self-signed cert for testing:

```
cp /path/to/your/fullchain.pem ~/.ashrix/server.crt
cp /path/to/your/privkey.pem ~/.ashrix/server.key

chmod 0640 ~/.ashrix/server.crt
chmod 0600 ~/.ashrix/server.key
```

Rules:

1. `certificate` must match `private_key`. If they do not pair, the HTTPS listener will fail to start.
2. `certificate` must be valid for the public domain the client uses, e.g. `grafana.example.com`. The gateway leaf cert in `~/.ashrix/pki/gateway.crt` is *not* used for this. It is only for gateway-to-connector mTLS.
3. If the files are missing or unreadable, HTTPS will not start. HTTP on `:80` and QUIC may still start depending on your config, but `https://<your-app>` will not work.
4. If you change the paths in `gateway.yaml`, you must put the files at the new paths. The gateway only reads what is configured, it does not search for them.




## Configuration

Gateway config — `~/.ashrix/gateway.yaml` — matches `examples/gateway.yaml`:

```yaml
gateway:
  id: gateway-01
  server_name: gateway.example.com
  certificate: /etc/ashrix/certs/connector.crt
  private_key: /etc/ashrix/certs/connector.key

listeners:
  http:
    address: "0.0.0.0:80"
  https:
    address: "0.0.0.0:443"
    certificate: /etc/ashrix-gateway/certs/server.crt
    private_key: /etc/ashrix-gateway/certs/server.key
  quic:
    address: "0.0.0.0:8443"

pki:
  ca_certificate: /etc/ashrix-gateway/pki/ca.crt
  ca_private_key: /etc/ashrix-gateway/pki/ca.key

connectors:
  - id: office-01
    certificate_serial: "04A7B2C9"

applications:
  - name: grafana
    domain: grafana.example.com
    connector: office-01

logging:
  dir: /etc/ashrix-gateway/pki
  level: info
  format: json
```


  
Connector config — `~/.ashrix/connector/connector.yaml` — matches `examples/connector.yaml`:

```yaml
connector:
  id: office-01
  certificate: /etc/ashrix/certs/connector.crt
  private_key: /etc/ashrix/certs/connector.key
  ca: /etc/ashrix/certs/ca.crt

gateway:
  address: gateway.example.com:443
  server_name: gateway.example.com

applications:
  - name: grafana
    target: 192.168.1.10:3000
  - name: internal-api
    target: 192.168.1.20:8080

logging:
  dir: /etc/ashrix-gateway/pki
  level: info
  format: json
```


### Application routing

Gateway:
```yaml
applications:
  - name: grafana
    domain: grafana.example.com
    connector: office-01
```

```yaml
Connector:
applications:
  - name: grafana
    target: 192.168.1.10:3000
```
|Gateway app	|Gateway connector	|Connector exists?	|Connector has same app name?	|Result|
|---|---|---|---|---|
`grafana` / `grafana.example.com` | `office-01`	|	Yes	| Yes, `grafana -> 192.168.1.10:3000`|	Routed
`grafana` / `grafana.example.com` | `office-99`	|	No |	N/A	 |Rejected, no such connector |
`internal-api` / `api.example.com` | `office-01`	|	Yes	|Yes, `internal-api -> 192.168.1.20:8080`	| Routed
`wiki` / `wiki.example.com` | `office-01`	|	Yes	|No, connector has no `wiki`	|Rejected, no such app on connector

See [Configuration](docs/03-configuration.md) for configuration and validation rules.



## Installation

Download the appropriate Linux binaries from https://github.com/ashrixlabs/ashrix/releases. Verify the release checksums before installation.


## Requirements

- Linux gateway host.
- Linux connector host.
- Publicly reachable gateway.
- Outbound UDP connectivity from connector to gateway.
- Valid gateway and connector certificates.
- DNS pointing application hostnames to the gateway.

## Documentation
Document	Description
[System overview](docs/01-system-overview.md)	Product scope and components
[Architecture](docs/02-architecture.md)	Gateway, connector, and tunnel architecture
[Configuration](docs/03-configuration.md)	Gateway and connector YAML
[PKI](docs/04-pki.md)	Certificate management
[Security](docs/05-security.md)	Security model and limitations
[Benchmarking](docs/06-benchmark.md)	Performance testing methodology

## Development

Build:
```
go build ./...
```

Test:
```
go test ./...
```

Race detection:
```
go test -race ./...
```
Run these commands with the Go version declared in `go.mod`


## Security

Ashrix authenticates connectors using mTLS. It does not provide end-user authentication, SSO, or application-level authorization. Your applications remain responsible for their own access control.

For the security model and reporting guidance, see [Security](docs/05-security.md).


## Project status

Ashrix is an open-source infrastructure project under active development. The focus is reliable tunneling, secure application forwarding, predictable resource consumption, and straightforward deployment.

## License

See [LICENSE](LICENSE).

