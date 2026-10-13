# Connection Lifecycle

Ashrix uses separate connections for user requests, component management, and application data. A failure in one connection does not necessarily imply that the others have failed.

## Gateway to Control Plane

1. The gateway establishes an outbound gRPC connection using mutual TLS.
2. It identifies itself and reports its locally applied policy version.
3. The Control Plane can send configuration, policy, trust, and revocation updates over the management stream.
4. The gateway reports health and relevant session or audit events.
5. If the stream ends, the gateway reconnects according to its retry policy and synchronizes state.



## Connector to Gateway

The connector initiates its management gRPC and data-plane QUIC connections to the gateway. The peers authenticate with their component certificates. Once the QUIC connection is available, the gateway can open independent streams for application traffic.

If a stream fails, that application request may fail while other streams remain usable. If the QUIC connection itself fails, its streams are lost and the connector must reconnect. Existing user requests should not be assumed to resume transparently on a new connection.

## User Session and Requests

The browser first establishes an authenticated gateway session. For each subsequent request, the gateway validates the session, evaluates policy, selects a connector, and proxies the request. User-session lifetime and tunnel lifetime are related operationally but are separate state: reconnecting a connector does not itself re-authenticate a user or restore an interrupted HTTP request.

See [Request Lifecycle](../02-architecture/request-lifecycle.md) for the request path and [QUIC Transport](./quic-transport.md) for stream behavior.

## Lifecycle and Failure Summary

| Event | Expected scope | Recovery or consequence |
|---|---|---|
| Control stream ends | Management connection | Reconnect and synchronize; cached policy behavior applies meanwhile |
| Connector management stream ends | Connector management | Reconnect and refresh connector state |
| QUIC stream resets | One proxied request | Return an application proxy error; other streams may continue |
| QUIC connection closes | All streams on that connection | Reconnect; interrupted requests may fail |
| Gateway restarts | Gateway sessions and connections | Re-registration/reconnection behavior depends on persisted state |

These are intended lifecycle semantics; session persistence, retry bounds, and shutdown behavior need tests before they are operational guarantees.

---

*Back: [QUIC Transport](./quic-transport.md)*