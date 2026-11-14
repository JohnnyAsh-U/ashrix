# Configuration

Ashrix uses YAML files for configuring the gateway and connector.

Each component loads its own configuration independently. The gateway configuration defines public application routing, while the connector configuration defines the local TCP destinations available through the tunnel.

There is no centralized configuration API or database.

1. Configuration files

Typical locations:

Component| Configuration
Gateway| "/etc/ashrix/gateway.yaml"
Connector| "/etc/ashrix/connector.yaml"

These paths are deployment conventions. They may be overridden through the supported CLI or startup options.

Example repository configurations are provided in the "examples/" directory.

2. Gateway configuration

The gateway configuration defines its network listeners, connector identities, and public application routes.

Example structure:

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

The example is illustrative. Field names and supported options must match the actual implementation.

Gateway configuration responsibilities

Gateway listener

Defines the public HTTP/HTTPS listener.

QUIC listener

Defines the UDP listener used by connectors.

TLS

Defines the gateway's server certificate, private key, and trusted client CA.

Connectors

Defines connector identities the gateway is configured to recognize and authorize.

Applications

Associates public hostnames with connectors and connector-side application names.

3. Connector configuration

The connector configuration defines its identity, gateway connection settings, TLS credentials, and local TCP destinations.

Example:

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

The example is illustrative and must be checked against the actual connector configuration schema.

Connector configuration responsibilities

Connector identity

Defines the connector's configured identity.

The authenticated certificate identity must agree with the connector identity expected by the gateway.

Gateway

Defines the gateway address and expected TLS server name.

Identity

Defines the connector certificate, private key, and trust material required for the connection.

Applications

Maps application names to local TCP destinations.

4. Application routing

The gateway and connector maintain complementary mappings.

Gateway:

applications:
  - name: grafana
    domain: grafana.example.com
    connector: office-01
    target_name: grafana

Connector:

applications:
  - name: grafana
    target: 192.168.1.10:3000

The gateway selects the connector and target application name. The connector resolves that application name against its own configuration.

The connector must never accept an arbitrary destination address supplied by an external request.

5. Configuration validation

Configuration should be validated before it is used.

Validation should reject:

- Invalid YAML syntax.
- Missing required fields.
- Invalid listener addresses.
- Invalid hostnames.
- Duplicate connector identities.
- Duplicate application names where uniqueness is required.
- Invalid TCP destinations.
- Missing certificate or key paths where required.
- Invalid connector-to-application references.
- Unsupported configuration values.

Unknown-field handling should be explicit and consistent. Strict parsing is preferable because silently ignored configuration fields can produce unexpected deployments.

6. Configuration reload

Ashrix uses local YAML files as its configuration source of truth.

The operator edits the appropriate file and triggers the supported reload mechanism.

The implementation should follow these principles:

1. Read the new configuration.
2. Parse the YAML.
3. Validate the complete configuration.
4. Prepare any resources required by the new configuration.
5. Publish the new configuration only after validation succeeds.
6. Retain the previous active configuration if reload fails.

A failed reload must not leave the process operating with a partially applied configuration.

Runtime considerations

Configuration reload must define what happens to:

- Existing QUIC connections.
- Existing application streams.
- Removed connectors.
- Changed application routes.
- Changed certificate paths.
- Invalid or inaccessible files.

The exact behavior depends on the implementation. Document only the behaviors verified by tests.

7. Configuration security

Configuration files may contain paths to sensitive certificate and private-key material.

Recommended permissions:

sudo chown root:ashrix /etc/ashrix
sudo chmod 0750 /etc/ashrix

sudo chmod 0640 /etc/ashrix/gateway.yaml
sudo chmod 0640 /etc/ashrix/connector.yaml

Private keys should have more restrictive permissions and should only be readable by the component that owns them.

Do not commit production certificates, private keys, or sensitive configuration files to version control.

8. Configuration examples

The "examples/" directory should contain:

- "gateway.yaml"
- "connector.yaml"

Keep these files synchronized with the actual configuration schema and validate them automatically in tests or CI.

