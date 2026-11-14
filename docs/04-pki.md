



PKI and Certificate Management

1. Overview

Ashrix uses mutual TLS (mTLS) to authenticate the communication between the gateway and connectors.

The gateway presents a server certificate. Each connector presents a client certificate.

Both components validate the certificate presented by the other side during the TLS handshake.

Ashrix includes a small PKI workflow to support certificate generation and issuance.

Certificate lifecycle operations are managed by the operator. Ashrix does not automatically renew or rotate certificates.

2. Certificate roles

Certificate| Purpose
CA certificate| Establishes certificate trust
Gateway certificate| Authenticates the gateway to connectors
Connector certificate| Authenticates a connector to the gateway

The corresponding private keys must remain protected.

3. Trust relationships

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

The connector verifies the gateway's server identity.

The gateway verifies the connector's client identity.

The gateway must not trust a connector merely because it supplies a connector ID in an application message. Identity must come from the authenticated TLS certificate.

4. CA initialization

Initialize the CA using the supported Ashrix PKI command.

The CA private key must be protected and should not be distributed to connectors.

Recommended principles:

- Generate the CA private key in a protected environment.
- Restrict access to the private key.
- Back up the CA key securely.
- Distribute only the CA certificate to components that need to verify certificates.
- Never place the CA private key in the connector configuration or connector installation.

The exact command syntax and supported key algorithms should be documented from the actual CLI implementation.

5. Connector certificate generation

Connector private keys should be generated locally.

The private key must never be transmitted to the gateway.

The connector generates a certificate signing request (CSR) containing the required connector identity information.

Conceptually:

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
   | Validate and sign
   |
   v
Issued connector certificate

6. Certificate issuance

The operator submits the connector CSR to the PKI workflow.

Before signing, the issuing process should validate:

- CSR signature.
- Requested connector identity.
- Permitted subject and SAN values.
- Public-key algorithm and parameters.
- Requested certificate usage.

The issuing authority should construct or validate the connector identity rather than blindly signing arbitrary CSR identity fields.

Connector certificates must contain the appropriate client authentication extended key usage.

The gateway must verify the certificate chain, validity, intended usage, and connector identity.

7. Certificate installation

After issuance:

1. Transfer the issued certificate to the connector.
2. Transfer or install the trusted CA certificate.
3. Keep the connector private key unchanged.
4. Set restrictive file permissions.
5. Validate that the certificate matches the private key.
6. Start or reconnect the connector using the installed credentials.

The connector must not start with an invalid certificate/key pair.

8. Gateway certificate

The gateway requires a server certificate and its corresponding private key.

The connector must trust the CA that issued the gateway certificate.

The connector must also validate the expected gateway server name.

Certificate chain validation alone is not sufficient if the expected server identity is not checked.

9. Certificate replacement

Certificate replacement is a manual operator-managed process.

A safe replacement procedure should:

1. Generate a replacement key and CSR where appropriate.
2. Issue the replacement certificate.
3. Install the replacement credentials.
4. Validate the new certificate and key.
5. Update the gateway's accepted connector identity or authorization state as required.
6. Reconnect the connector.
7. Verify that the new identity is accepted.
8. Retire the previous certificate according to the configured revocation process.

Do not invalidate the existing certificate before the replacement has been installed and verified unless an emergency compromise requires immediate revocation.

The exact activation and existing-session behavior must match the implementation.

10. Certificate expiration

Certificates have finite validity periods.

The gateway must reject expired certificates during authentication.

Operators are responsible for tracking certificate expiration and replacing certificates before they expire.

Ashrix does not automatically renew certificates or distribute replacements.

11. Revocation

Revocation is an operator-managed operation.

The gateway must enforce its configured connector authorization state when accepting new connections.

If a certificate is revoked or replaced, understand whether an already-established QUIC connection remains active. Rejecting a certificate on future handshakes does not automatically terminate an existing authenticated session.

If immediate disconnection is required, the corresponding active session must also be terminated.

12. Private-key protection

- Restrict private-key file permissions.
- Do not place private keys in source control.
- Do not include private keys in logs or diagnostic output.
- Avoid copying private keys between hosts.
- Protect CA backups.
- Use separate identities for different connectors.

13. Operational checklist

Before deploying a connector, verify:

- The connector has its own private key.
- The connector certificate is valid.
- The certificate contains the expected identity.
- The connector trusts the gateway CA.
- The gateway trusts the connector issuing CA.
- The certificate is valid for its intended TLS role.
- File permissions prevent unauthorized access.

See "Security" (security.md) for the broader trust model.


