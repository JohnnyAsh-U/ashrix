# Policy Integrity

## Overview

Ashrix distributes access policy from the Control Plane to gateways and connectors. Because the CP is not in the hot path for access decisions, gateways evaluate policy locally. This means the integrity of the distributed policy is critical to the security of the entire system.

This document explains:

1. How policies are signed and how receivers verify them.
2. How version monotonicity prevents downgrade attacks.
3. How freshness deadlines prevent bundle withholding attacks.
4. The distinction between what a digital signature provides and what it does not.

---

## What a Digital Signature Provides (and What It Does Not)

A digital signature on a policy bundle establishes two things:

1. **Authenticity:** The bundle was signed by a key holder — in Ashrix's case, the CP's bundle-signing private key.
2. **Integrity:** The bundle has not been modified since it was signed.

A digital signature does **not** prevent:

- **Replay attacks:** An older, legitimately signed bundle can be re-sent to a receiver. If the receiver does not check the version, it would accept an outdated policy.
- **Bundle withholding:** An attacker who controls the network can simply not deliver new bundles. Signatures on bundles that never arrive provide no protection.

Ashrix addresses both of these threats with additional mechanisms beyond the signature itself.

---

## Signed Bundle Structure ✅ Implemented

All policy bundles (delta and full snapshot), trust bundles, revocation events, and rotation commands are wrapped in a CP-signed envelope:

```protobuf
message SignedBundle {
  bytes  payload          = 1;  // Serialised PolicyDelta or PolicySnapshot
  bytes  signature        = 2;  // Ed25519 signature over the canonical bundle signing payload
  uint64 version          = 3;  // Monotonic version number
  int64  expires_at       = 4;  // Unix timestamp — bundle validity window
  int64  max_valid_until  = 5;  // Unix timestamp — fail-closed deadline
}
```

### Verification Sequence

Receivers apply the following checks **in order** before parsing `payload`:

```
1. Check expires_at > now
   → Bundle is still within its declared validity window

2. Verify Ed25519 signature using the configured `ASHRIX_POLICY_PUBLIC_KEY`
   → Bundle is authentic and unmodified

3. Check version > current_version
   → Prevents downgrade: a replayed older bundle is rejected

4. Check max_valid_until (deadline tracking)
   → If no bundle with version > current has arrived by this deadline,
     trigger POLICY_STALE state

5. Parse and apply payload content
```

Steps 1 and 3 are checked **before** step 2 (signature verification) to avoid wasting CPU on bundles that are trivially invalid. However, the monotonic version check must also be re-applied **after** signature verification to prevent an attacker from crafting a message that passes step 3 but has a forged signature.

---

## Downgrade Attack Prevention ✅ Implemented

A monotonic version constraint ensures that a gateway cannot be forced to operate on an older policy version:

```
gateway.currentPolicyVersion = 847

Incoming bundle with version = 840:
  → version (840) <= current (847)
  → REJECT (logged as "bundle.rejected", reason="downgrade")

Incoming bundle with version = 848:
  → version (848) > current (847)
  → Proceed with signature verification and apply
```

This check is applied on every bundle received, regardless of whether it arrives via a delta or full snapshot path.

---

## Bundle Withholding and POLICY_STALE ✅ Designed, ⚙️ Partial

A signed bundle that is never delivered provides no protection. `max_valid_until` is the mechanism that detects this condition.

Each bundle includes a `max_valid_until` timestamp. If the gateway has not received and applied a bundle with a **greater version** by the time `max_valid_until` is reached, it transitions to `POLICY_STALE`:

```
Normal operation:
  bundle(version=847, max_valid_until=T+24h) applied
  Before T+24h: bundle(version=848) received → apply, reset deadline
  → Continues in ACTIVE state

Withholding attack (or CP outage):
  bundle(version=847, max_valid_until=T+24h) applied
  T+24h passes with no newer bundle:
  → POLICY_STALE state entered
  → New access requests: DENIED (fail-closed)
  → In-flight sessions: continue
  → Heartbeat emits stale-policy alert
  → Retries CP connection to fetch new bundle
  → On success: apply new bundle, return to ACTIVE
```

### Availability vs. Security Trade-off

Transitioning to fail-closed on stale policy improves security at the cost of availability. The `max_valid_until` window is configurable per tenant:

- **Short window (e.g., 1 hour):** Very tight security — a 1-hour CP outage causes service interruption.
- **Long window (e.g., 24 hours):** Tolerant of longer CP outages — but policy changes (including revocations) won't propagate during that window.

There is no universally correct answer. The choice depends on the tenant's threat model and availability requirements. This must be documented as a tenant configuration decision, not an Ashrix default that is invisible to the operator.

---

## Delta vs. Full Snapshot

### Why Deltas?

Sending a full policy snapshot on every update is wasteful for large policy sets with frequent small changes. Deltas (incremental mutations) reduce bandwidth and application latency.

### How Deltas Are Computed

The CP maintains a `policy_mutations` changelog table:

```sql
CREATE TABLE policy_mutations (
    version       BIGSERIAL PRIMARY KEY,
    tenant_id     VARCHAR(64) NOT NULL,
    rule_id       VARCHAR(255) NOT NULL,
    op            VARCHAR(10) CHECK (op IN ('UPSERT', 'DELETE')),
    rule_snapshot JSONB,
    mutated_at    TIMESTAMPTZ DEFAULT NOW()
);
```

When a gateway connects with `current_policy_version = N`, the CP queries:

```sql
SELECT version, rule_id, op, rule_snapshot
FROM policy_mutations
WHERE tenant_id = ? AND version > ?
ORDER BY version;
```

The CP collapses redundant mutations in memory (if rule `R` was updated twice, only the final state is sent), then wraps the result in a `PolicyDelta` and signs it.

### When Full Snapshots Are Used

The CP sends a full `PolicySnapshot` when:

- The gateway is connecting for the first time (no `current_policy_version`).
- The gateway's `current_policy_version` is so old that the mutations log has been partially pruned.
- The gateway detects a local policy.db hash mismatch (corruption) and requests a snapshot explicitly.

---

## Gateway Local Policy Persistence

The gateway persists its policy bundle to BBolt (`/var/lib/ashrix-gateway/policy.db`). This enables:

- Serving traffic immediately on restart with the cached policy (before CP reconnects).
- Surviving transient CP unavailability without service interruption.

**Integrity check on load:** On startup, the gateway recomputes a hash of the persisted bundle and compares it to the stored hash. If they differ (corruption), the gateway discards the local policy, starts with an empty bundle, and requests a full snapshot from the CP after connecting.

---

## Policy Integrity vs. Policy Correctness

A cryptographically valid, correctly versioned policy bundle proves that the policy was authored and signed by the CP. It does **not** prove that the policy is correct from a security perspective.

Policy correctness failures are:

- A rule that grants access to `group: everyone` when it should restrict to `group: admins`.
- A rule that is missing, leaving an application with no access controls.
- A rule that was correct but was not updated when the user's role changed.

These are **operational** and **authoring** failures, not cryptographic failures. The signature only guarantees authenticity and integrity of whatever the CP chose to sign.

This distinction matters for auditing: a security review of Ashrix must include both the policy distribution mechanism (this document) and the policy authoring workflow (the CP admin interface and policy change process).

---

*Back to [Threat Model](./threat-model.md)*
