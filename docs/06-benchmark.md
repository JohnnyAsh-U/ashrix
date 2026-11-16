# Benchmarking Ashrix

## 1. Purpose

This document describes the methodology used to evaluate Ashrix performance, resource consumption, and behavior under load.

The goal is to measure the overhead introduced by forwarding application traffic through an outbound QUIC tunnel.

Benchmarking should answer:

- What throughput can Ashrix sustain?
- What latency overhead does the tunnel introduce?
- How many concurrent connections and streams can it handle?
- How much CPU and memory does it consume?
- How does performance change under network impairment?
- How does the system behave near resource limits?
- How reliably does it recover from connection failures?

Performance claims must be supported by reproducible measurements.

## 2. Benchmarking principles

All benchmarks should:

- Use documented hardware and software configurations.
- Include a direct-network baseline.
- Separate gateway and connector resource consumption.
- Measure both throughput and latency.
- Include warm-up periods.
- Repeat experiments.
- Report distributions rather than only averages.
- Record failed requests and incomplete transfers.
- Avoid comparing results from different environments without qualification.

Never publish an isolated maximum throughput result without its test conditions.

## 3. Test topology

Use three logical components:
```
+------------------+
| Load Generator   |
+------------------+
         |
         | HTTP
         v
+------------------+
| Ashrix Gateway   |
+------------------+
         |
         | QUIC tunnel
         v
+------------------+
| Ashrix Connector |
+------------------+
         |
         | TCP
         v
+------------------+
| Test Application |
+------------------+
```
The load generator should run outside the connector's private network.

The test application should run on the connector side.

## 4. Baseline measurements

Establish a direct TCP/HTTP baseline before measuring Ashrix.

Measure:

1. Direct client-to-application performance.
2. Client-to-gateway-to-connector application performance through Ashrix.

Keep the application, payload, test duration, and network conditions identical.

This provides a basis for estimating tunnel overhead.

## 5. Performance dimensions

### 5.1 Throughput

Measure:

- Requests per second.
- Upload throughput.
- Download throughput.
- Sustained throughput over time.
- Throughput under concurrent streams.

Test multiple payload sizes.

Suggested payloads:

Payload| Purpose
1 KiB| Small request overhead
16 KiB| Small application transfers
256 KiB| Medium transfers
1 MiB| Large application transfers
10 MiB| Sustained transfer behavior

Record both application-level throughput and transport-level throughput where possible.

### 5.2 Latency

Measure:

- p50 latency.
- p90 latency.
- p95 latency.
- p99 latency.
- Maximum observed latency.

Use both low-concurrency and high-concurrency tests.

Report the baseline latency alongside Ashrix latency.

Do not interpret average latency as sufficient evidence of performance.

### 5.3 Concurrency

Evaluate different levels of concurrent activity.

Suggested test levels:

Concurrent requests or streams
1
10
50
100
250
500
1,000

These are experimental targets, not guaranteed capacity claims. Adjust them to the hardware and implementation limits.

Observe throughput, latency, CPU, memory, goroutines, and failures as concurrency increases.

### 5.4 Resource consumption

Measure separately on gateway and connector:

- CPU utilization.
- Resident memory.
- Heap allocation.
- Garbage collection activity.
- Goroutine count.
- File descriptor count.
- Network throughput.
- Active QUIC connections.
- Active streams.

Use Go profiling tools where appropriate:

go tool pprof
go tool trace

Useful runtime profiling includes:

- CPU profiles.
- Heap profiles.
- Allocation profiles.
- Goroutine profiles.
- Execution traces.

### 5.5 Connection establishment

Measure:

- QUIC connection establishment time.
- TLS handshake duration.
- Successful connection rate.
- Failed handshake rate.
- Reconnection duration after network interruption.

Separate cold connection establishment from requests over an established connection.

## 6. Network impairment tests

Test Ashrix under realistic network conditions.

Use Linux traffic control ("tc netem") to introduce:

- Latency.
- Jitter.
- Packet loss.
- Packet duplication.
- Packet reordering.
- Bandwidth limitations.

Example:

sudo tc qdisc add dev eth0 root netem \
  delay 50ms 10ms \
  loss 1%

Remove the impairment:

sudo tc qdisc del dev eth0 root

Run these experiments in an isolated test environment. Traffic control affects the selected network interface and can disrupt unrelated traffic.

Record the actual network conditions with every result.

## 7. Failure and recovery tests

Performance testing must include failure scenarios.

Test:

- Gateway process termination.
- Connector process termination.
- UDP packet loss.
- Temporary network disconnection.
- QUIC connection timeout.
- Local application unavailable.
- TCP connection reset.
- Large transfers interrupted mid-stream.
- Repeated connector reconnection.
- Gateway shutdown during active requests.

Measure:

- Failed request count.
- Recovery time.
- Resource cleanup.
- Reconnection behavior.
- Whether stale sessions remain.
- Whether requests are incorrectly routed to disconnected connectors.

A system that performs well but leaks resources after failures is not production-ready.

## 8. Load testing

Load testing should evaluate the complete system rather than isolated Go functions.

Use a load generator such as:

- "k6" (https://k6.io/)
- "vegeta" (https://github.com/tsenart/vegeta)
- "wrk" (https://github.com/wg/wrk)

Use different workload patterns:

1. Constant request rate.
2. Gradually increasing request rate.
3. Burst traffic.
4. Concurrent long-lived transfers.
5. Mixed small and large requests.
6. Sustained load over an extended period.

Record:

- Offered request rate.
- Achieved request rate.
- Successful request rate.
- Error rate.
- Latency percentiles.
- Throughput.
- CPU and memory.
- Active connections and streams.

Do not confuse offered load with successfully completed throughput.

## 9. Go microbenchmarks

Use Go benchmarks to measure individual components.

Example:
```go
func BenchmarkForwardingCopy(b *testing.B) {
    src := make([]byte, 32*1024)
    dst := make([]byte, 32*1024)

    b.SetBytes(int64(len(src)))
    b.ReportAllocs()
    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        copy(dst, src)
    }
}
```
Run:

go test ./... -bench=. -benchmem

Use microbenchmarks for isolated operations such as:

- Protocol encoding and decoding.
- Routing lookup.
- Buffer handling.
- Stream setup.
- Request metadata processing.

Microbenchmarks do not replace end-to-end tests.

## 10. Benchmark methodology

For each experiment:

1. Record the hardware.
2. Record the operating system and kernel.
3. Record the Go version.
4. Record the Ashrix commit or release.
5. Record the configuration.
6. Record the network topology.
7. Run a warm-up.
8. Execute multiple measured runs.
9. Capture resource metrics.
10. Preserve raw results.

Avoid running unrelated workloads on benchmark hosts.

Repeat experiments to identify variance.

## 11. Result reporting

Use a consistent report format.

Metric| Direct baseline| Ashrix| Difference
|---|---|---|---|
Throughput| TBD| TBD| TBD
p50 latency| TBD| TBD| TBD
p95 latency| TBD| TBD| TBD
p99 latency| TBD| TBD| TBD
CPU usage| TBD| TBD| TBD
Memory usage| TBD| TBD| TBD
Error rate| TBD| TBD| TBD

These are placeholders, not measured results.

Include the complete test environment and workload configuration alongside every published table.

## 12. Performance optimization

Optimization should follow measurement.

A useful workflow:

1. Establish a reproducible baseline.
2. Identify the dominant bottleneck.
3. Capture CPU, memory, and execution profiles.
4. Form a specific hypothesis.
5. Make one focused change.
6. Run correctness and race tests.
7. Repeat the benchmark.
8. Compare results against the baseline.
9. Keep the change only if it produces a meaningful improvement without compromising correctness.

Avoid premature optimization.

Pay particular attention to:

- Unbounded buffering.
- Excessive allocations.
- Unnecessary data copies.
- Goroutine leaks.
- Lock contention.
- Poor backpressure handling.
- Excessive stream creation overhead.
- Connection lifecycle inefficiencies.
- Unnecessary serialization.

Correctness and bounded resource consumption take priority over marginal throughput gains.

## 13. Publishing benchmark results

Every public benchmark report should include:

- Ashrix version or Git commit.
- Hardware specifications.
- OS and kernel version.
- Go version.
- Gateway and connector configuration.
- Network conditions.
- Load generator and version.
- Workload parameters.
- Number of repetitions.
- Raw or machine-readable results.
- Known limitations.

Never present synthetic or estimated results as measured results.

## 14. Benchmark history

Keep historical benchmark results so performance changes can be compared across releases.

A possible structure:

```
benchmarks/
├── README.md
├── methodology.md
├── results/
│   ├── v0.1.0.md
│   ├── v0.2.0.md
│   └── latest.md
└── scripts/
    ├── run-throughput.sh
    ├── run-latency.sh
    └── run-load.sh
```

Keep benchmark scripts and workload definitions version-controlled where practical.

## 15. Engineering objective

The objective is not to claim that Ashrix is faster than every alternative.

The objective is to understand its performance characteristics, identify bottlenecks, establish realistic operating limits, and make improvements that can be reproduced independently.


## 16. Executable benchmark suite

The repository includes an executable end-to-end HTTP benchmark suite under `benchmarks/`.

See [`benchmarks/README.md`](../benchmarks/README.md) for setup and commands. It runs the same health, upload, download, and offered-load workloads against the direct test application and the Ashrix gateway, preserving raw Vegeta data and JSON/text reports. The suite includes warm-ups, repeated runs, environment metadata, and a deterministic local test application.

The benchmark suite does not claim performance results. Results must be generated on the target host(s). The loopback baseline is a controlled software-path baseline; distributed tests are required before making WAN performance claims.

For the complete executable workload and resilience procedures, see:

- [`benchmarks/README.md`](../benchmarks/README.md) — automated direct-vs-Ashrix HTTP benchmark suite.
- [`benchmarks/ADVANCED-TESTS.md`](../benchmarks/ADVANCED-TESTS.md) — separate-host tests, network impairment, connection establishment, failure recovery, and profiling.

