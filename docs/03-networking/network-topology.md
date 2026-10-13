# Network Topology

## Overview

Ashrix uses different transport technologies for different communication paths. This separation reflects the different requirements of management traffic (reliable, ordered, typed) versus application-data traffic (low-latency, multiplexed, resilient).

| Path | Protocol | Direction | Purpose |
|---|---|---|---|
| Gateway → Control Plane | gRPC over mTLS (HTTP/2) | Outbound from GW | Management, config, policy, audit |
| Connector → Gateway (management) | gRPC over mTLS (HTTP/2) | Outbound from Connector | Heartbeat, logs, app health |
| Connector → Gateway (data plane) | QUIC over mTLS | Outbound from Connector | Multiplexed application traffic streams |
| User Browser → Gateway | HTTPS (public CA TLS 1.3) | Inbound to GW | User access, session establishment |
| Gateway → Internal App | Via Connector QUIC tunnel | — | Proxied application traffic |
| Connector → CP (break-glass) | HTTPS (public CA TLS 1.3) | Outbound from Connector | Certificate renewal, recovery |

---

## Outbound-Only Connectivity

A core design principle of Ashrix is that **no inbound firewall rules are required** for the data-plane components.

| Component | Initiates outbound to | Requires inbound from |
|---|---|---|
| **Control Plane** | IdP callbacks, SCIM | Gateways |
| **Gateway** | CP (gRPC/mTLS) | Users (HTTPS), Connectors (gRPC/QUIC) |
| **Connector** | Gateway (gRPC, QUIC), CP break-glass (HTTPS) | None |
| **Browser** | Gateway (HTTPS, port 443) | None |

The only component that needs inbound ports exposed is the **gateway** — and that is acceptable because gateways are explicitly deployed in a DMZ or with public IP addresses to serve user traffic.

---

## Network Zones

```
┌─────────────────────────────────────────────────────────┐
│                     INTERNET                             │
│  User Browser  ───────────────────────────────►  (HTTPS) │
└────────────────────────────────┬────────────────────────┘
                                 │ port 443 (HTTPS)
┌────────────────────────────────▼────────────────────────┐
│                TENANT DMZ / PUBLIC SUBNET                │
│                                                          │
│   ┌───────────────────────────────────────────────────┐  │
│   │                    GATEWAY                         │  │
│   │  • User-facing HTTPS listener                     │  │
│   │  • gRPC listener for connectors                   │  │
│   │  • QUIC listener for connector data plane         │  │
│   │  • Outbound gRPC to CP                            │  │
│   └─────────────────┬─────────────────────────────────┘  │
└─────────────────────│───────────────────────────────────┘
                      │ gRPC + QUIC (outbound from connector)
┌─────────────────────▼───────────────────────────────────┐
│               TENANT PRIVATE NETWORK                     │
│                                                          │
│   ┌──────────────┐                                       │
│   │  CONNECTOR   │  (no inbound ports)                  │
│   └──────┬───────┘                                       │
│          │ localhost / private network                   │
│   ┌──────▼──────────────────────────┐                   │
│   │  Internal Applications           │                   │
│   │  (jenkins:8080, postgres:5432)   │                   │
│   └──────────────────────────────────┘                   │
└─────────────────────────────────────────────────────────┘
```

---

## Multi-Region Topology

Enterprise tenants may deploy multiple regional gateways under a single tenant identity. Gateways do not share session state — each gateway independently validates CP-signed session tickets.

```
                ┌─────────────────────────────┐
                │     ASHRIX CONTROL PLANE     │
                │     (multi-region, HA)       │
                └──┬───────────────┬───────────┘
                   │               │  gRPC/mTLS Control Streams (outbound from GW)
         ┌─────────▼──┐      ┌─────▼──────────┐
         │  GW-US      │      │  GW-Africa     │
         │  us-east-1  │      │  af-south-1    │
         └─────────────┘      └────────────────┘
              │                      │
         ┌────▼────┐            ┌────▼────┐
         │ Conn-US │            │ Conn-AF │
         └────┬────┘            └────┬────┘
          [US Apps]              [Africa Apps]
```


---

## Port Requirements Summary

| Component | Port | Protocol | Direction | Purpose |
|---|---|---|---|---|
| CP | 443 | HTTPS + HTTP/2 (gRPC) | Inbound | User auth, gateway C2, SCIM, break-glass |
| Gateway | 443 | HTTPS | Inbound | User access |
| Gateway | 443 | gRPC/mTLS + QUIC | Inbound | Connector management + data plane |
| Connector | 443 | TCP (gRPC outbound) | Outbound | Management stream to Gateway |
| Connector | 443 | UDP (QUIC outbound) | Outbound | Data plane to Gateway |
| Connector | 443 | TCP (HTTPS outbound) | Outbound | Break-glass renewal/recovery to CP |

> **Note**: These port are configurable

> **Note:** QUIC uses UDP. Environments that block UDP on port 443 will prevent Connector data-plane connections.

---

*Next: [QUIC Transport →](./quic-transport.md)*
