# mTLS and Certificate Lifecycle

## Overview

Mutual TLS (mTLS) is the foundation of trust between all Ashrix server-to-server connections. Every component that communicates with another component — gateway to CP, connector to gateway — presents an identity certificate and verifies the peer's certificate.

This document covers:

1. The PKI hierarchy that issues those certificates.
2. The identity properties embedded in each certificate.
3. The full certificate lifecycle: issuance, renewal, revocation, and recovery.
4. The trust bundle that enables cross-component verification.

---

## PKI Hierarchy

```
Root CA  (offline key)
    │
    ▼
Intermediate CA  (encrypted local key for disk backend; Vault PKI for kms backend)
    │
    ├── Gateway Identity Certificates   (90-day validity)
    ├── Connector Identity Certificates  (90-day validity)
    └── mTLS Client Certificates         (break-glass, 24h)
```

| Key | Purpose | Storage | Exportable? |
|---|---|---|---|
| Root CA private key | Signs Intermediate CA only | Encrypted file under `$HOME/.ashrix/pki`; unlock secret supplied through `PKI_UNLOCK_SECRET` | No — never stored in DB or sent to gateways |
| Intermediate CA private key | Signs component identity certificates | Encrypted local file for `disk`; Vault PKI for `kms` | No — never stored in DB or sent to gateways |
| Policy signing key | Signs policy payloads | Encrypted file under `$HOME/.ashrix/pki/cp`; secret supplied through `BUNDLE_SIGNING_SECRET` | No — public key only is distributed |
| Control Plane private key | CP mTLS identity | Encrypted file under `$HOME/.ashrix/pki/cp`; `PKI_UNLOCK_SECRET` | No |
| Gateway private key | mTLS identity for gateway | Generated and persisted by gateway registration | No |
| Connector private key | mTLS identity for connector | Generated and persisted by connector enrollment | No |

### Design Principles

- **One key, one job.** No key is reused across purposes. The policy signing key is not the CA key. The CA key is not the node identity key.
- **Limit blast radius.** A compromised gateway key allows gateway impersonation; it does not allow forging policy bundles or signing new certificates.
- **Current limitation:** the disk backend's root key is encrypted at rest but is present on the CP host when used. This is not equivalent to an offline HSM or non-exportable KMS key. Protect and back up the key file and `PKI_UNLOCK_SECRET` separately.

---

## Bundle-Signing Key Provisioning

Provision the CP bundle-signing key explicitly with `ashrix-cp setup bundle-key`. The private Ed25519 key is encrypted at rest and loaded by the CP at startup. The command prints the PEM public key and a base64-encoded PKIX DER value for gateway configuration:

```text
ASHRIX_POLICY_PUBLIC_KEY=<base64 value printed by setup bundle-key>
```

Configure that value in every gateway environment before starting it. The gateway fails startup if the variable is missing or invalid. A mismatched public key causes legitimate policy updates to fail verification.

`setup bundle-key` reuses an existing signer; it is not a key-rotation command. Rotation requires coordinated replacement of the CP private key and gateway public-key configuration. Do not delete the existing signer file to force regeneration.

The `ASHRIX_POLICY_PUBLIC_KEY` setting applies to gateway policy verification. The connector key-verification path is separate.

---

## Initial Provisioning

After database migrations and required secrets are configured, provision in this order:

1. `ashrix-cp setup pki` creates or reconciles the root and intermediate certificates and records public certificates in PostgreSQL.
2. `ashrix-cp setup cp-cert` provisions the CP mTLS identity and registers its public certificate against the intermediate CA record.
3. `ashrix-cp setup bundle-key` provisions the policy signer and prints the gateway verification key.
4. `ashrix-cp setup bootstrap --org-name ... --org-slug ... --admin-email ...` creates the initial organization and owner in a transaction. The password is prompted without echo. The account starts active with TOTP disabled.
5. Install `ASHRIX_POLICY_PUBLIC_KEY` on each gateway, then run `ashrix-cp serve`.

Provisioning commands reuse matching artifacts and fail on incomplete or conflicting state rather than replacing keys or certificates. Normal server startup is load-only: missing material is an error, not a request to mint a new identity.

On the owner's first password login, the dashboard presents TOTP enrollment. The short-lived password-verified challenge is restricted to enrollment endpoints; successful code verification enables TOTP and issues the normal session. No CLI-generated TOTP setup token is used.

## Key Use

The bundle-signing public key is used to verify:
- Policy bundles (delta and snapshot)
- Trust bundles
- Revocation events
- Rotation commands
- Suspend / resume commands

**Security implication:** A compromised gateway cannot forge payloads signed by the CP private key. Public-key configuration does not expose the private signing key.

---

## Certificate Validity Periods

| Certificate | Validity | Rotation Trigger |
|---|---|---|
| Root CA | 20 years in current generator | No automated rotation; replacement requires a planned trust migration. |
| Intermediate CA | 1 year | Month 9–10 of validity. Human approval required. |
| Gateway / Connector | 90 days | Day 60. Fully automated. |
| mTLS break-glass session | 24 hours | Per re-authentication / re-handshake. Automated. |

### Why 90-Day Node Certificates?

- Short enough to limit the exposure window if a private key is stolen.
- Long enough to absorb brief offline periods (a connector host that is offline for a month will still have a valid cert when it comes back).
- Aligned with Let's Encrypt's default, which has established tooling and operational familiarity.
- Rotation at day 60 (2/3 of validity) provides a 30-day buffer for rotation to complete before expiry.

---

## Certificate Renewal Flow ✅ Designed

### Trigger

- **Day 60:** Gateway or connector detects its certificate is within the renewal window (30 days before expiry).
- **CP-signed `RotationCommand`:** CP instructs a node to renew immediately (e.g., before a scheduled CA rotation).

### Possession Proof

Before the CP issues a replacement certificate, the renewing node must prove it controls the private key of the currently issued certificate:

```
possession_proof = ECDSA_SIGN(
    SHA256(csr_pem || node_id || timestamp),
    current_private_key
)
```

The CP verifies the signature against the public key in the currently issued certificate. Knowing a `node_id` is not sufficient — the requester must control the key.

### Connector Renewal Flow

```
1. Connector detects cert within renewal window
2. Generates new EC P-256 keypair
3. Creates CSR with same connector_id
4. Creates possession proof (signed with CURRENT private key)
5. Contacts CP directly via break-glass HTTPS (bypasses gateway)
   POST /v1/renew {connector_id, csr, timestamp, signature}
6. CP verifies: connector_id registered, timestamp ±5min, proof valid
7. CP issues new cert via Vault Intermediate CA
8. Connector receives: new cert, updated trust bundle (if CA rotation)
9. Connector hot-reloads:
   - Writes new cert to disk (atomic rename)
   - Loads new cert into live TLS config
   - Reconnects gRPC + QUIC with new cert
   - Old connections drain gracefully
```

### Gateway Renewal Flow

Mirrors connector renewal, except the gateway contacts the break-glass endpoint and then reconnects its Control Stream to the CP with the new certificate.

---

## Certificate Revocation ✅ Designed

### CP Is the Sole Revocation Authority

Only the CP can revoke certificates. No other component has the authority or the keys to issue or modify revocation records.

### Gateway Certificate Revocation

```
1. CP revokes gateway cert, updates CRL
2. CP pushes RevocationEvent to gateway via ControlStream (priority message)
3. Gateway verifies CP signature on RevocationEvent
4. Gateway terminates mTLS session with CP
5. Gateway stops accepting Connector connections
6. Gateway initiates recovery flow directly with CP break-glass
```

### Connector Certificate Revocation

```
1. CP revokes connector cert, updates CRL
2. CP pushes updated CRL to gateway via ControlStream (priority message)
3. Gateway loads CRL into memory
4. Gateway's per-call interceptor detects revoked serial on next Connector request
5. Gateway terminates active Connector session immediately
6. Gateway relays CP-signed RevocationEvent to Connector
7. Connector verifies CP signature using embedded public key
8. Connector enters REVOKED state:
   - Closes all QUIC streams
   - Terminates gRPC management stream
   - Stops all traffic proxying
9. Connector attempts re-enrollment via break-glass if permitted by policy
```

### CRL Distribution Priority

CRL updates and revocation events are **priority messages** that bypass the normal stream queue. The maximum revocation propagation SLA:

```
CP issues revocation
    → CRL pushed as priority message
    → Gateway loads CRL within 2 seconds of receipt
    → Per-call check on next Connector request
    → Total: ≤ 5 seconds from CP decision to enforcement
```

SLA compliance is monitored:
- CP records `revocation_issued_at` timestamp.
- Gateway records `crl_loaded_at` in next heartbeat.
- CP calculates propagation latency and alerts if it exceeds 5 seconds.

---

## Expired Certificate Recovery ✅ Designed

If a node's certificate has expired (it missed the renewal window), the node must not attempt to connect to the gateway. The gateway **explicitly rejects expired certificates** — not even as a special case for recovery.

```
1. Node starts, loads certificate from disk
2. Checks NotAfter → if expired:
   a. Do NOT attempt gateway connection (would be rejected)
   b. Enter RECOVERY state

3. Node contacts CP directly via break-glass HTTPS:
   POST /v1/recover {node_id, csr (new keypair), timestamp, signature (signed with OLD key)}

4. CP verifies:
   - node_id is registered
   - timestamp within ±5 minutes
   - proof signature valid against LAST KNOWN public key in CP database
     (NOT the expired certificate — key possession is what matters)

5. CP issues replacement certificate
6. Node persists new cert, connects normally
```

### Break-Glass Endpoint Rate Limiting

| Control | Value |
|---|---|
| Max recovery attempts per `node_id` per hour | 5 |
| Backoff on consecutive failures | Exponential: 1s, 2s, 4s, 8s, 16s |
| Alert threshold | 2 failed attempts for a single `node_id` in 10 minutes |
| IP-level rate limit | 20 requests/min per source IP |

---

## Trust Bundle Rotation

When the Intermediate CA is rotated, the trust bundle contains **both old and new intermediates simultaneously** during the overlap window:

```
Trust Bundle v5:
  - Root CA cert
  - Intermediate CA v1 cert (retiring)
  - Intermediate CA v2 cert (active)
```

**Rotation sequence:**

```
1. v2 generated and signed by Root CA (offline ceremony)
2. CP pushes bundle [Root, v1, v2] to all nodes
3. All nodes verify CP signature and apply
4. Overlap window (60 days): all node certs reissued under v2
5. CP confirms every node has acknowledged v2 (version ledger)
6. v1 removed from bundle, new bundle pushed
7. v1 expires
```

**Version ledger:** The CP tracks which nodes have confirmed each bundle version. Removal of v1 is gated on 100% acknowledgment from all registered nodes.

---

## What Happens When a Certificate Is Revoked During an Active Connection?

This is a question that requires an explicit policy, not a technical default.

Current behaviour:
- **Connector → Gateway (mTLS QUIC):** The gateway's per-call CRL check detects the revoked serial on the next gRPC call from the connector. The connector's QUIC connection is terminated.
- **Gateway → CP (mTLS gRPC):** If the gateway's certificate is revoked, the CP pushes a `RevocationEvent`. The gateway terminates the stream and enters recovery.
- **User sessions:** Session revocation does not depend on TLS certificates — sessions are managed via Redis and revoked by Redis key deletion. A revoked connector cert does not automatically revoke user sessions, but it does prevent new application traffic from flowing through that connector.

> **Gap to address:** What happens to in-flight QUIC streams when a connector's certificate is revoked? The current architecture terminates the gRPC call that triggers the CRL check, but the QUIC connection (and its active streams) may survive briefly until the gateway closes them. The exact stream drain/close sequence requires verification.

---

*Next: [Policy Integrity →](./policy-integrity.md)*
