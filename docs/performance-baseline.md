# Networking Test and Benchmark Baseline

## Scope

The Go integration tests use a real localhost gateway QUIC listener, a connector QUIC client and accept loop, and local HTTP upstreams. Test certificates are generated per test and use a temporary private CA. The tests do not start the CP, Redis, or the full gateway CLI startup flow; HTTP requests inject the app and session context directly, so they exercise forwarding but not OIDC, policy middleware, or Redis sessions.

The benchmarks are defined in `internal/gateway/server/http/proxy_benchmark_test.go`. They cover direct TCP/HTTP upload and download, HTTP forwarding through the gateway TLS handler and mTLS QUIC tunnel, parallel HTTP requests, and parallel gateway-opened QUIC streams. Upload/download payloads are 1 MiB; concurrent echo payloads are 16 KiB. Concurrent benchmarks use Go's `RunParallel` with `SetParallelism` values 1, 4, and 16. The integration test also checks 2 MiB request/response bodies and 24 simultaneous requests.

The direct baseline is unencrypted HTTP over loopback TCP to the app. The Ashrix path includes the local TLS frontend and a QUIC connection using mTLS; it excludes CP, Redis, policy evaluation, and production process startup. This is a component benchmark, not an end-to-end deployment comparison.

## Captured Environment

Captured from the current workspace host on 2026-11-11:

| Item | Value |
|---|---|
| OS/kernel | Fedora Linux 44, Linux 7.2.7-200.fc44.x86_64 |
| Architecture | linux/amd64, x86_64 |
| Go | go1.26.8-X:nodwarf5 |
| CPU | Intel Core i5-8365U, 4 cores / 8 logical CPUs, 1.60 GHz base |
| RAM | 15,690,804 KiB reported by `/proc/meminfo` (about 15 GiB) |
| QUIC | mTLS, `ashrix-quic-v1`, 15 s keepalive, 30 s idle timeout, 1,000 incoming streams |
| Network | Loopback only; local test gateway, connector, and app |

For future runs, record the commit/worktree state, exact command, date, CPU governor/load, `GOMAXPROCS`, and any environment overrides alongside benchmark output. Keep the host idle and use the same Go version and payload sizes when comparing runs.

## Commands

Run the unit and integration suite with a bounded timeout:

```sh
go test -timeout=120s ./...
```

Run race detection:

```sh
go test -race -timeout=180s ./...
```

Run serial direct and Ashrix upload/download benchmarks, collecting Go allocation data and five samples per CPU setting:

```sh
go test ./internal/gateway/server/http -run '^$' \
  -bench 'Benchmark(DirectTCPHTTP|AshrixHTTPForwarding)$' \
  -benchmem -count=5 -cpu=1,2,4,8
```

Run parallel request/stream benchmarks:

```sh
go test ./internal/gateway/server/http -run '^$' \
  -bench 'BenchmarkAshrixConcurrent(Requests|QUICStreams)$' \
  -benchmem -count=5 -cpu=1,2,4,8
```

Capture CPU profile or system CPU counters for a separate run:

```sh
go test ./internal/gateway/server/http -run '^$' \
  -bench '^BenchmarkAshrixHTTPForwarding/upload-1MiB$' \
  -benchtime=30s -cpuprofile=cpu.out
perf stat -d -- go test ./internal/gateway/server/http -run '^$' \
  -bench '^BenchmarkAshrixHTTPForwarding/upload-1MiB$' -benchtime=30s
```

Inspect peak process memory separately from Go allocation metrics:

```sh
/usr/bin/time -v go test ./internal/gateway/server/http -run '^$' \
  -bench '^BenchmarkAshrixHTTPForwarding/download-1MiB$' -benchtime=30s
go test ./internal/gateway/server/http -run '^$' \
  -bench '^BenchmarkAshrixHTTPForwarding/download-1MiB$' \
  -benchtime=30s -memprofile=mem.out
go tool pprof -top mem.out
```

The benchmarks report `ns/op`, bytes/op, allocations/op, and process goroutine/file-descriptor deltas where `/proc/self/fd` is available. Use `/usr/bin/time -v` for peak RSS and `perf stat` for CPU counters; do not infer CPU utilization or peak memory from `ns/op` or Go allocation counts alone. Preserve raw output and profiles with the machine/configuration record.

## Current Results

No benchmark performance results have been collected. The tests are baseline instrumentation only; do not treat the host metadata or workload definitions as performance claims.

The existing SOCKS package initially timed out because its tests held the reserved port open while the server attempted to bind it. The test-only setup now releases the port before server startup, retries readiness, and uses client deadlines; the focused package test passes. The connector bootstrap test passes against a fake CP, including CSR validation and ECDSA verification of the signed status request.

Focused gateway/connector tests pass startup, sequential 2 MiB request/response forwarding, multiple-app routing, reconnect, and graceful shutdown. The full command `go test -timeout=120s ./...` exits nonzero only at `TestConcurrentHTTPRequestsUseIndependentQUICStreams`. The race command `go test -race -timeout=180s ./...` reports the same failure and no race warnings. In that case the upstream parses the declared 65,536-byte request but returns a 512-byte response. Keep this regression visible until the concurrent forwarding behavior is understood and fixed.

Benchmark functions compile but have not been executed. No performance numbers have been recorded.