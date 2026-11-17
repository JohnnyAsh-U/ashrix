# PKI and Certificate Management

## 1. Overview

Ashrix uses mutual TLS (mTLS) to authenticate the communication between the gateway and connectors.

The gateway presents a server certificate. Each connector presents a client certificate.

Both components validate the certificate presented by the other side during the TLS handshake.

Ashrix includes a small PKI workflow to support certificate generation and issuance.

Certificate lifecycle operations are managed by the operator. Ashrix does not automatically renew or rotate certificates.

## 2. Certificate roles

|Certificate| Purpose |
|---|---|
CA certificate| Establishes certificate trust
Gateway certificate| Authenticates the gateway to connectors
Connector certificate| Authenticates a connector to the gateway


The corresponding private keys must remain protected.


## 3. Trust relationships

```
             Trusted CA
                 |
          +------+------+
          |             |
          v             v
    Gateway Cert   Connector Cert
          |             |
          v             v
    Connector      Gateway
    verifies       verifies
    gateway        connector
```

The connector verifies the gateway's server identity.

The gateway verifies the connector's client identity.

The gateway must not trust a connector merely because it supplies a connector ID in an application message. Identity must come from the authenticated TLS certificate.

## 4. CA initialization

+Initialize the CA and gateway server identity using the public name connectors will use to reach the gateway.

```shell
ashrix-gateway init --name gateway-01 --server-name gateway.example.com
```

The gateway certificate includes this DNS name (or IP address) as its server identity. The connector's `gateway.server_name` must match it.

The CA private key must be protected and should not be distributed to connectors.

Recommended principles:

- Generate the CA private key in a protected environment.
- Restrict access to the private key.
- Back up the CA key securely.
- Distribute only the CA certificate to components that need to verify certificates.
- Never place the CA private key in the connector configuration or connector installation.



## 5. Connector certificate generation

Generate the connector CSR using the supported Ashrix PKI command.
```bash
ashrix-connector init --name office-01 --quic-addr gateway.example.com:8444 --server-name gateway.example.com --out office-01.csr
``` 

The private key must never be transmitted to the gateway.

The connector generates a certificate signing request (CSR) containing the required connector identity information.


Conceptually:

```
Connector
   |
   | Generate private key
   |
   v
Private key stays local
   |
   | Generate CSR
   |
   v
Operator transfers CSR
   |
   v
Gateway / PKI
   |
   | Validate, sign and add connector
   |
   v
Issued connector certificate
```


## 6. Certificate issuance

The operator submits the connector CSR to the PKI workflow.

```bash
ashrix-gateway add-connector \
  --csr office-01.csr \
  --out office-01-signed.crt
```

Before signing, the issuing process should validate:

- CSR signature.
- Requested connector identity.
- Permitted subject and SAN values.
- Public-key algorithm and parameters.
- Requested certificate usage.


The issuing authority should construct or validate the connector identity rather than blindly signing arbitrary CSR identity fields.

Connector certificates must contain the appropriate client authentication extended key usage. Connector certificates are limited to 90 days; gateway server certificates have a separate server-authentication profile.

The gateway must verify the certificate chain, validity, intended usage, and connector identity.


## 7. Certification Installation

The operator submits the cert file generated from the previous command to the connector for installation.
```bash
ashrix-connector install
  --cert cert.crt
```

This installs the certificate and root ca in the path configured in the connector.yaml (`certificate`, `ca`)





## 8. Certificate replacement

Certificate replacement is a manual operator-managed process, using the same process mentioned above


## 9. Certificate expiration

Certificates have finite validity periods.

The gateway must reject expired certificates during authentication.

Operators are responsible for tracking certificate expiration and replacing certificates before they expire.

Ashrix does not automatically renew certificates or distribute replacements.


See "Security" (security.md) for the broader trust model.


