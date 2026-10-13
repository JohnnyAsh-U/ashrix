# Ashrix Documentation

> **Ashrix ZTNA** — Zero Trust Network Access Platform  
> Version 1.1 · Go · Production Architecture

---

## Documentation Map

| Section | Contents |
|---|---|
| [01 — Overview](./01-overview/introduction.md) | What Ashrix is, the problem it solves, goals, use cases, and terminology |
| [02 — Architecture](./02-architecture/architecture-overview.md) | System components, control plane, gateway, connector, identity, and request lifecycle |
| [03 — Networking](./03-networking/network-topology.md) | Transport protocols, QUIC tunnels, gRPC channels, and connection lifecycle |
| [04 — Security](./04-security/threat-model.md) | Threat model, PKI hierarchy, mTLS, certificate lifecycle, and policy integrity |
| [05 — Distributed Systems](./05-distributed-systems/component-failures.md) | Failure modes, policy consistency, reconnection, retries, and state management |
| [08 — Operations](./08-operations/development-environment.md) | Dev environment, local/Linux/container deployment, config reference, upgrades, troubleshooting |
| [11 — Roadmap](./11-roadmap/implemented.md) | What is implemented, in progress, planned, and known limitations |

### Overview Reading Path

Start with the [Introduction](./01-overview/introduction.md), then continue through the [Problem Statement](./01-overview/problem-statement.md), [Goals and Non-Goals](./01-overview/goals-and-non-goals.md), [Use Cases](./01-overview/use-cases.md), and [Terminology](./01-overview/terminology.md).

### Architecture Decision Records

- [ADR-001: Control Plane Separation](./10-engineering-decisions/adr-001-control-plane-separation.md)
- [ADR-002: QUIC Transport](./10-engineering-decisions/adr-002-quic-transport.md)
- [ADR-003: gRPC Control Channel](./10-engineering-decisions/adr-003-grpc-control-channel.md)
- [ADR-004: Application-Level Access](./10-engineering-decisions/adr-004-application-level-access.md)

---

## Quick Reference

| Component | Binary | Default Port | Protocol |
|---|---|---|---|
| Control Plane (CP) | `cmd/cp` | `443` | HTTPS + gRPC/mTLS |
| Gateway | `cmd/gateway` | `443` | HTTPS (users) + gRPC/mTLS (CP + Connector) |
| Connector | `cmd/connector` | outbound only | gRPC/mTLS + QUIC |

---

Ashrix ZTNA Production Architecture v1.1
