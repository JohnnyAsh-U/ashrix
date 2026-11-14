# Configuration

Ashrix uses YAML files for configuring the gateway and connector.

Each component loads its own configuration independently. The gateway configuration defines public application routing, while the connector configuration defines the local TCP destinations available through the tunnel.

There is no centralized configuration API or database.

## 1. Configuration files

Typical locations:

| Component | Configuration |
|---|---|
|Gateway| /homedirectory/.ashrix/gateway.yaml|
|Connector| /homedirectory/.ashrix/connector/connector.yaml|


Example repository configurations are provided in the `examples/` directory.

## 2. Gateway configuration

The gateway configuration defines its identity, listeners, PKI trust material, connector identities, and public application routes.

Example structure — matches `examples/gateway.yaml`:

```yaml
gateway:
  id: gateway-01
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
The example is illustrative. Field names and supported options must match the actual implementation.

### Gateway configuration responsibilities

- `gateway`: Defines the gateway identity (`id`) and its certificate / private key.
- `listeners.http`: Defines the public plain HTTP listener. No TLS.
- `listeners.https`: Defines the public HTTPS listener, with its own certificate and private key. This is separate from `gateway.certificate`.
- `listeners.quic`: Defines the  QUIC listener used by connectors. In the example this is `:8443`.
- `pki`: Defines the CA certificate and CA private key used to trust / verify connectors. `ca_certificate` is the trust root. It must be a CA certificate (`IsCA=true`), not a leaf certificate.
- `connectors`: Defines connector identities the gateway is configured to recognize and authorize. Each entry is identified by `id` and bound to an expected `certificate_serial`.
- `applications`: Associates a public hostname (`domain`) with a connector (`connector`) and an application name (`name`). The gateway does *not* define a TCP destination here. It only defines the name to ask the connector for.
- `logging`: Defines log output directory, level, and format.

## 3. Connector configuration

The connector configuration defines its identity, gateway connection settings, TLS credentials, and local TCP destinations.

Example structure — matches `examples/connector.yaml`:

```yaml
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

logging:
  dir: /etc/ashrix-gateway/pki
  level: info
  format: json

```


- `connector.id`: Defines the connector's configured identity. The authenticated certificate identity must agree with the connector identity expected by the gateway.
- `connector.certificate` / `connector.private_key`: Defines the connector certificate and private key presented to the gateway.
- `connector.ca`: Defines the CA certificate used to verify the gateway.
- `gateway.address`: Defines the gateway address the connector dials. In the example this is `gateway.example.com:443`.
- `applications`: Maps application names to local TCP destinations (`target`). This is the only place a real destination exists.
- `metrics`: Defines the local observability listener. It is not exposed through the tunnel.
- `logging`: Defines log output directory, level, and format.

## 4. Application routing

The gateway and connector maintain complementary mappings. Neither side alone is enough to route a request.


Gateway:

```yaml
applications:
    - name: grafana
    domain: grafana.example.com
    connector: office-01
```

Connector:
```yaml
applications:
    - name: grafana
    target: 192.168.1.10:3000
```

Routing flow:

1. Client requests `grafana.example.com`.
2. Gateway looks up `applications` by `domain`.
3. Gateway selects the `connector` named in that application.
4. Gateway asks that connector for the application by `name`.
5. Connector looks up `name` in its own `applications` and dials `target`.
6. Connector must never accept an arbitrary destination address supplied by an external request. Only names present in its own config may be resolved.


### Routing requirements — both must hold

For the gateway to route a request to a connector, *both* of these must be true:

1. *The application must reference a connector that exists in the gateway config.*
      - `applications[].connector` must equal the `id` of an entry in `connectors[]`.
      - If it references a connector that is not listed, the application is unroutable and must be rejected at validation / return `404 No App` at request time.
      - Example: `connector: office-01` only works because `connectors` contains `- id: office-01`.

2. *The application name in the gateway must match the application name in the connector.*
      - Gateway `applications[].name` must exactly match connector `applications[].name`.
      - The match is exact and case-sensitive.
      - Example: gateway `name: grafana` only routes if the connector also has `name: grafana`. If the connector only has `internal-api`, the gateway cannot route `grafana` to it, even if the connector is connected and authorized.

If either condition fails, there is no route. The gateway must not fall back to another connector, another application, or a default target.


### Worked example
| Gateway app |	Gateway connector | Connector exists? | Connector has same app name? | Result |
|---|---|---|---|---|
|`grafana` / `grafana.example.com` | `office-01`	| Yes | Yes, `grafana -> 192.168.1.10:3000` |	Routed |
|`grafana` / `grafana.example.com` | `office-99`|	No	| N/A	 |Rejected, no such connector|
|`internal-api` / `api.example.com` | `office-01` |	Yes	| Yes, `internal-api -> 192.168.1.20:8080`	| Routed
`wiki` / `wiki.example.com` | `office-01`	|	Yes	| No, connector has no `wiki` |	Rejected, no such app on connector



## 5. Configuration validation

Configuration should be validated before it is used. Validation should reject:

- Invalid YAML syntax.
- Missing required fields.
- Invalid listener addresses.
- Invalid hostnames.
- Duplicate connector identities.
- Duplicate application names where uniqueness is required.
- Duplicate application domains in the gateway.
- Invalid TCP destinations.
- Missing certificate or key paths where required.
- Invalid connector-to-application references — an application whose `connector` does not exist in `connectors[]`.
- Invalid application name references — a gateway application whose `name` has no matching application on the referenced connector, where that can be checked at deployment time.
- Unsupported configuration values.
- A CA certificate path that points to a leaf certificate (`IsCA=false`).

Unknown-field handling should be explicit and consistent. Strict parsing is preferable because silently ignored configuration fields can produce unexpected deployments.


## 6. Configuration examples

The `examples/` directory should contain:

- `gateway.yaml`
- `connector.yaml`

Keep these files synchronized with the actual configuration schema and validate them automatically in tests or CI.
