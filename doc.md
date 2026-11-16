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


