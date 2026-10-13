# Use Cases

These scenarios describe the intended product workflows. They do not imply that every workflow is complete; consult the implementation and test status in the relevant component documentation.

## Employee Access to an Internal Web Application

An employee opens a registered application URL in a browser. The gateway redirects an unauthenticated user through the configured identity provider. After authentication, the gateway evaluates the user's session and application policy, then proxies permitted requests through a connector to the internal application.

The user receives access to that application, not general reachability to the subnet where it runs. See the [request lifecycle](../02-architecture/request-lifecycle.md).

## Restrict or Revoke Access

An administrator changes an application policy or revokes an active session. The Control Plane distributes the relevant update to the gateway, which enforces the resulting state. The time to enforcement and behavior during disconnection depend on the policy freshness and revocation mechanisms documented in [Policy Integrity](../04-security/policy-integrity.md); they should not be assumed to be instantaneous without verified measurements.

## Connect Applications in a Private Network

A connector inside the application environment initiates outbound connections to its gateway. The gateway routes authorized traffic over the connector tunnel, and the connector forwards it to applications on its configured list. Operators should also use host and network controls to limit the connector's reachable destinations.

See [Connector](../02-architecture/connector.md) and [Network Topology](../03-networking/network-topology.md).

## Continue During a Control Plane Interruption

The architecture aims to let a gateway continue using its locally cached policy during a temporary Control Plane outage. This trades availability for the possibility that a recent policy change is not yet known to the gateway. The documented stale-policy deadline and fail-closed behavior must be verified against the running implementation before being treated as an operational guarantee.

See [Component Failures](../05-distributed-systems/component-failures.md) and [Policy Consistency](../05-distributed-systems/policy-consistency.md).

## Out of Scope

Ashrix is not intended to provide unrestricted VPN access, inspect application payloads as a WAF or DLP system, or replace tenant firewalls and endpoint controls. These boundaries are detailed in [Goals and Non-Goals](./goals-and-non-goals.md).

---

*Next: [Terminology](./terminology.md)*