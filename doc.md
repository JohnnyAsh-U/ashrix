/etc/ashrix/
├── gateway.yaml
├── pki/
│   ├── connector-ca.crt
│   └── connector-ca.key
└── connectors/
    └── office-01/
        ├── enrollment.json
        └── status.json


Connector:
  ID: office-01
  Status: active
  Certificate serial: 42A9...
  Issued: 2026-11-01
  Expires: 2026-12-01


ashrix/
├── cmd/
│   ├── gateway/
│   └── connector/
│
├── internal/
│   ├── gateway/
│   ├── connector/
│   ├── tunnel/
│   ├── transport/
│   ├── proxy/
│   ├── routing/
│   ├── config/
│   ├── pki/
│   ├── enrollment/
│   ├── admin/
│   └── metrics/
│
├── benchmarks/
│   ├── latency/
│   ├── throughput/
│   ├── connections/
│   ├── recovery/
│   ├── memory/
│   └── scripts/
│
├── tests/
│   ├── integration/
│   ├── security/
│   └── resilience/
│
├── examples/
│   ├── gateway.yaml
│   └── connector.yaml
│
├── docs/
│   ├── architecture.md
│   ├── configuration.md
│   ├── networking.md
│   ├── security.md
│   ├── pki.md
│   ├── enrollment.md
│   ├── operations.md
│   └── benchmarks.md
│
├── go.mod
└── README.md

9. Benchmark suite
This should be a major part of the project, not an afterthought.

I would build a dedicated benchmarks/ directory with repeatable scripts, configuration files, raw results and documentation.
Benchmark 1 — Latency overhead
Compare:
- Direct client-to-application access.
- Client-to-gateway-to-connector-to-application through Ashrix.
Measure:
- p50 latency.
- p95 latency.
- p99 latency.
- Maximum observed latency.
- Request failure rate.
- Ashrix's added latency.
Test several request sizes and response types.

The direct path is your baseline. Compare equivalent workloads, payloads and network conditions.
Benchmark 2 — Throughput
Measure maximum sustainable throughput.
Workload	Measurements
Small responses	Requests/sec, latency
Large downloads	MB/s, CPU
Large uploads	MB/s, memory
Streaming responses	Throughput, buffering
Concurrent requests	Throughput and errors

Compare direct forwarding with Ashrix forwarding.
Benchmark 3 — Concurrent connections
Test increasing numbers of simultaneous clients and streams.
Suggested connection test matrix
Planned benchmark levels, not measured results.


01,5 k3 k4,5 k6 kSmallMediumLargeStressExtended



And the initial Admin API:
Method	Endpoint	Purpose
GET	/health	Gateway health
GET	/config	Read configuration
POST	/config/validate	Validate configuration
PUT	/config	Update configuration
POST	/connectors	Create connector
GET	/connectors	List connectors
GET	/connectors/{id}	Connector details
DELETE	/connectors/{id}	Delete connector
POST	/connectors/{id}/certificates	Submit CSR and issue certificate
GET	/connectors/{id}/certificates	List certificates
POST	/connectors/{id}/certificates/{serial}/revoke	Revoke certificate




# Initialize gateway configuration and CA
ashrix gateway init

# Inspect and validate configuration
ashrix gateway config validate
ashrix gateway config show

# Connector management
ashrix gateway connector create
ashrix gateway connector list
ashrix gateway connector show office-01
ashrix gateway connector delete office-01

# Certificate management
ashrix gateway cert init-ca
ashrix gateway cert sign office-01 --csr connector.csr
ashrix gateway cert list office-01
ashrix gateway cert revoke office-01 --serial <serial>







Measure:
- Memory usage.
- CPU consumption.
- Goroutine count.
- File descriptors.
- Connections per second.
- Streams per connection.
- Latency under load.
- Error rates.
Important: distinguish the number of QUIC connections from the number of streams. One connector can carry many logical streams.
Benchmark 4 — Memory and allocation profiling
Use Go benchmarks, pprof, and allocation profiling.

Investigate:
- Bytes allocated per request.
- Allocations per operation.
- Memory per active connection.
- Memory per active stream.
- Buffer allocation patterns.
- Garbage collection frequency.
- Heap growth under sustained load.
Don't optimize buffer pooling or introduce sync.Pool until profiling identifies a real bottleneck.
Benchmark 5 — Reconnection and recovery
Simulate:
- Gateway restart.
- Connector restart.
- Temporary network loss.
- Long network interruption.
- Packet loss and jitter.
Measure:
- Time to detect disconnection.
- Time to reconnect.
- Time until applications become available again.
- Failed requests during recovery.
- Resource leaks after repeated reconnect cycles.
Benchmark 6 — Resource efficiency
Measure the complete gateway and connector processes.
- Idle RSS.
- CPU at idle.
- CPU under load.
- Memory under load.
- Goroutines.
- File descriptors.
- Network bandwidth.
- Resource growth over long-running tests.
Benchmark 7 — Long-running stability
Run sustained workloads for several hours, ideally extending to 24 hours in a dedicated environment.

Look for:
- Memory leaks.
- Goroutine leaks.
- File descriptor leaks.
- Stale stream registrations.
- Unbounded buffers.
- Reconnection storms.
- Increasing latency over time.
Benchmark 8 — Configuration and certificate lifecycle
Measure operational behavior:
- Configuration reload latency.
- Time to enroll a connector.
- Certificate renewal success rate.
- Time to revoke a connector.
- Time to terminate its active tunnel.
- Gateway recovery after restart.
These are operational benchmarks rather than raw throughput benchmarks, but they matter to the product.






Implement a minimal local administrative API for Ashrix Gateway.

This API is not a replacement control plane. It is a local interface for administering one gateway.

Implement the smallest useful endpoint set:

- GET /config/
- POST /config/validate
- PUT /config/
- GET /connectors
- GET /connectors/{id}
- POST /connectors/{id}/enrollment-token
- POST /connectors/{id}/revoke
- GET /health
- GET /metrics

Requirements:

1. Bind to loopback by default.
2. Support Unix sockets if compatible with the existing architecture.
3. Validate configuration before applying it.
4. Apply changes safely and atomically.
5. Keep the previous working configuration if an update fails.
6. Never expose private keys, enrollment tokens or sensitive credentials.
7. Separate public application traffic from administrative traffic.
8. Add authorization to administrative operations if they can be reached beyond the local host.
9. Test malformed requests, invalid configuration, concurrent updates and unauthorized requests.

Use the existing configuration and gateway registry implementations. Do not introduce a database or dashboard.