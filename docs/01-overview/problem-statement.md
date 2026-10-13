# Problem Statement

## The Access Problem

Organizations need employees and contractors to use applications hosted on private networks. Traditional VPN access solves reachability by placing a remote device on a network or routing traffic into it. That can expose more network surface than the user needs and leaves application authorization dependent on controls distributed across the network and each application.

Ashrix is intended to make the access decision at the application boundary. A user requests a named application; the platform authenticates the user, evaluates the applicable policy, and allows or denies the proxied request.

## Desired Outcomes

- Grant access to registered applications rather than broad network segments.
- Associate access sessions with an authenticated identity.
- Apply authorization at the gateway before traffic is forwarded.
- Connect private applications without requiring inbound connectivity to the connector network.
- Keep application payload traffic out of the Control Plane.
- Make access and denial events attributable to a user, application, and gateway.

These are product and architecture goals. Their presence here is not proof that every outcome is implemented or tested; see the [implementation status](../index.md#implementation-status-legend) and the component documentation.

## Scope Boundaries

Ashrix does not replace network firewalls, endpoint protection, application security, or the tenant's identity provider. Network controls should still restrict which destinations the connector can reach, and protected applications should not be directly reachable through a path that bypasses the gateway and connector.

The platform is designed for application access, not arbitrary network-level TCP or UDP tunnelling. Device posture is limited to the signals the implementation can collect; passive request metadata must not be described as a trustworthy endpoint assessment.

## Security Responsibilities

Authentication establishes an identity. Authorization decides whether that identity may use an application. The gateway enforces the decision by refusing or forwarding the request. These steps are distinct: successful authentication alone must not grant application access.

See [Goals and Non-Goals](./goals-and-non-goals.md) for explicit scope and [Use Cases](./use-cases.md) for representative workflows.

---

*Next: [Goals and Non-Goals](./goals-and-non-goals.md)*