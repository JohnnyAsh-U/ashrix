# 🛡️ Ashrix

> **ALPHA / EXPERIMENTAL — WORK IN PROGRESS**

Ashrix is an experimental **Zero Trust Network Access (ZTNA)** platform for securely publishing and controlling access to internal applications without exposing those applications directly to the public internet.

The project is being developed as a practical exploration of **zero-trust access, secure networking, identity-aware authorization, distributed systems, and high-performance network transport in Go**.

> ⚠️ **Security & production disclaimer**
>
> Ashrix is currently an experimental alpha project and **must not be used to protect production systems**. APIs, protocols, security mechanisms, and deployment procedures are subject to change. The project has not undergone an independent security audit or formal production-readiness review.

---

## 🧭 Architecture

Ashrix separates **control-plane responsibilities** from the **data plane**.

```text
                         ┌──────────────────────────┐
                         │      Ashrix Control      │
                         │          Plane           │
                         │                          │
                         │  Identity / Policies     │
                         │  Gateway Registry        │
                         │  Audit / Management      │
                         │  PostgreSQL              │
                         └────────────┬─────────────┘
                                      │
                              mTLS / gRPC
                                      │
                    ┌─────────────────┴─────────────────┐
                    │                                   │
             ┌──────▼──────┐                     ┌──────▼──────┐
             │   Gateway   │                     │   Gateway   │
             │ Enforcement │                     │ Enforcement │
             │    Node     │                     │    Node     │
             └──────┬──────┘                     └──────┬──────┘
                    │                                   │
                 QUIC                              QUIC
              mTLS tunnel                        mTLS tunnel
                    │                                   │
          ┌─────────▼─────────┐               ┌─────────▼─────────┐
          │    Connector      │               │    Connector      │
          │                    │               │                    │
          │ Internal Network   │               │ Internal Network   │
          └─────────┬─────────┘               └─────────┬─────────┘
                    │                                   │
              Internal Apps                        Internal Apps
```

### Control Plane

The control plane is responsible for centralized management and coordination:

* Identity provider configuration
* Users and groups
* Application registration
* Access policies
* Gateway and connector registration
* Policy distribution
* Component revocation
* Security and access events
* Persistent state in PostgreSQL

The control plane is **not intended to carry application traffic**.

### Gateway

The gateway is the primary enforcement point.

It:

* Authenticates incoming sessions
* Evaluates access policies
* Routes traffic to the appropriate connector
* Maintains connections with the control plane
* Maintains connector management and transport sessions
* Enforces component and session revocation
* Provides the data-plane entry point for protected applications

Gateways maintain cached, cryptographically verified policy state so that application traffic does not need to traverse the control plane.

### Connector

Connectors run inside the network containing the protected applications.

A connector establishes **outbound connections** to an Ashrix gateway rather than requiring inbound connectivity from the public internet.

This allows protected applications to remain isolated behind existing network boundaries without requiring public inbound firewall rules for each application.

### Data Plane

Application traffic is transported through **multiplexed QUIC tunnels secured with mutual TLS**.

The current transport implementation supports experimentation with:

* HTTP
* WebSocket
* Raw TCP streams
* Application-to-application connectivity

The data plane is designed to remain independent from the control plane.

---

## 🔐 Security Model

Ashrix is being developed around several zero-trust security principles.

### Mutual Authentication

Internal Ashrix components authenticate each other using a private PKI and mutual TLS.

The architecture currently includes authenticated communication between:

* Control Plane ↔ Gateway
* Gateway ↔ Connector

### Signed Policy Distribution

Authorization state can be distributed to gateways as cryptographically signed policy bundles.

Gateways verify bundles before applying them and maintain the last trusted policy state locally.

Policy updates are applied atomically to avoid partially applied authorization state.

### Revocation

The architecture includes mechanisms for revoking:

* Gateway credentials
* Connector credentials
* User sessions
* Other security-sensitive identities

Revocation state is propagated to enforcement components so that access can be terminated without requiring the control plane to become part of the application data path.

### Fail-Closed Authorization

Ashrix is designed so that loss of connectivity with the control plane does not automatically mean loss of authorization guarantees.

Gateways can continue enforcing their last trusted policy state for a bounded period while preventing unauthorized policy rollback or indefinite offline operation.

---

## ⚙️ Current Capabilities

The project is actively evolving, but the current implementation includes experimentation with:

* [x] Control Plane ↔ Gateway synchronization
* [x] Gateway ↔ Connector synchronization
* [x] User and group based authorization
* [x] Application access policies
* [x] Signed policy bundles
* [x] Policy version enforcement
* [x] Component certificate revocation
* [x] Session revocation
* [x] mTLS authentication
* [x] QUIC-based application transport
* [x] HTTP forwarding
* [x] WebSocket forwarding
* [x] Raw TCP forwarding
* [x] Application-to-application connectivity
* [ ] SSH access
* [ ] Database access
* [ ] Browser-based terminal
* [ ] OIDC/identity integrations
* [ ] Advanced audit and observability
* [ ] High-availability gateway deployment

> The checklist describes development status, not production readiness.

---

## 🛠️ Technology

### Core

* **Go** — control plane, gateway, and connector
* **PostgreSQL** — persistent control-plane state
* **Redis / Valkey** — session and ephemeral state
* **bbolt** — local trusted state
* **gRPC** — control and management communication
* **QUIC** — application data transport
* **mTLS / private PKI** — component authentication

### Infrastructure

* Podman
* Ansible
* Linux
* OpenTelemetry

---

## 🗺️ Roadmap

### Access

* [ ] SSH access
* [ ] Browser-based SSH terminal
* [ ] Short-lived SSH certificates
* [ ] Database access
* [ ] RDP access

### Identity

* [ ] OIDC provider integrations
* [ ] Microsoft Entra ID
* [ ] Keycloak
* [ ] Google
* [ ] Additional enterprise identity providers

### Security

* [ ] More granular authorization policies
* [ ] Improved session revocation
* [ ] Security event pipeline
* [ ] Expanded audit logging
* [ ] Security hardening
* [ ] Independent security review

### Infrastructure

* [ ] Gateway high availability
* [ ] Connector failover
* [ ] Multi-region control plane
* [ ] Production deployment tooling
* [ ] Performance and scale testing

---

## 🚀 Deployment

Deployment tooling is currently experimental and primarily reflects the development environment used to build and test Ashrix.

Generic deployment workflows using containers and infrastructure automation will be documented as the architecture stabilizes.

---

## 🧪 Why This Project Exists

Ashrix is primarily a systems and security engineering project.

It is an attempt to explore how a modern application-access platform can combine:

* Zero Trust authorization
* Identity-aware access control
* Private networking
* Mutual TLS
* Cryptographic policy distribution
* Distributed state synchronization
* QUIC-based transport
* High-concurrency networking in Go

The project is intentionally being developed in the open so that the architecture, implementation decisions, limitations, and security assumptions can be reviewed and challenged.

---

## 🤝 Contributing

Ashrix is currently undergoing active architectural development, so the API and internal protocols may change significantly.

Pull requests are not currently guaranteed to be accepted.

However, **architectural discussions, bug reports, security reviews, threat-model feedback, and implementation critiques are welcome.**

If you identify a potential security issue, please avoid publishing sensitive exploitation details in a public issue. A private security-reporting process will be documented as the project matures.

---

## 📜 License

License: **TBD**

The licensing model will be finalized before the project moves beyond its experimental stage.
