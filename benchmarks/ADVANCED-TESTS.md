# Ashrix advanced benchmark and resilience runbook

Run the automated HTTP benchmark suite first. This runbook covers experiments that need deliberate network/process control and should not be mixed into ordinary throughput runs.

## A. Concurrency and sustained load

The suite runs `/health` at offered rates of 1, 10, 50, 100, 250, and 500 requests/sec, with three repetitions by default. It also measures five payload sizes in both directions. Raise `WORKERS` and `DURATION` only after checking CPU, memory, file descriptors, and error rates.

Recommended progression:

1. Establish the default baseline.
2. Repeat with `DURATION=60s WARMUP=10s REPEATS=5`.
3. Increase `WORKERS` from 100 to 250, then 500.
4. Stop increasing load when error rate rises sharply, p99 latency becomes unstable, or resource usage approaches host limits.
5. Do not interpret offered rate as achieved throughput.

## B. Separate-host experiment

For a realistic topology:

- Host A: Vegeta load generator.
- Host B: Ashrix gateway.
- Host C: Ashrix connector and benchmark test application.

The connector target should point to the test application's private address. Set `ASHRIX_BENCH_ADDR` to the connector host's private interface address. Direct baseline traffic must reach the same test application from Host A. Record RTT and bandwidth between hosts. Do not compare this result with a loopback run as if they were equivalent.

## C. Network impairment (Linux)

Use a dedicated test interface or isolated VM. `tc` changes affect real traffic on the selected interface.

Inspect interfaces and current qdisc:

```bash
ip -br link
sudo tc qdisc show dev INTERFACE
```

Apply one impairment at a time:

```bash
# Add 50 ms delay with 10 ms variation
sudo tc qdisc add dev INTERFACE root netem delay 50ms 10ms

# Or 1% packet loss
sudo tc qdisc add dev INTERFACE root netem loss 1%

# Or 20 Mbit/s rate limit
sudo tc qdisc add dev INTERFACE root tbf rate 20mbit burst 256kbit latency 400ms
```

Remove it immediately after the experiment:

```bash
sudo tc qdisc del dev INTERFACE root
```

Record the interface, qdisc command, measured RTT, and whether impairment was applied on the gateway-facing path, connector-facing path, or both. Do not stack these examples without understanding qdisc replacement/parenting behavior.

## D. Connection and failure recovery

Run each case at low request rate first. Capture gateway and connector logs, process metrics, and request results.

| Scenario | Procedure | Expected observation |
|---|---|---|
| Connector stopped | Stop connector during requests, then restart | In-flight requests fail; route becomes usable after reconnect |
| Gateway stopped | Stop gateway, then restart | Connector reconnects according to implementation; no stale session should remain |
| App stopped | Stop local test app while connector remains connected | Gateway returns an upstream failure; connector remains connected |
| App TCP reset | Restart app during sustained requests | Affected requests fail; subsequent requests recover |
| UDP path interrupted | Temporarily block QUIC UDP traffic in an isolated lab | Requests fail while tunnel is unavailable; observe reconnection behavior |
| Large transfer interrupted | Terminate connector during a 10 MiB transfer | Transfer fails; no partial response should be treated as success |
| Repeated reconnect | Restart connector repeatedly | No accumulating stale sessions, goroutines, or file descriptors |

Do not assume automatic retry semantics for application requests. Verify actual behavior; retrying non-idempotent requests could duplicate application actions.

## E. Connection establishment

The HTTP suite measures requests over an established tunnel; it does not isolate QUIC + TLS handshake duration. Measure cold connection setup separately by recording connector process start to successful authenticated registration, repeating at least 20 times. Use monotonic timestamps on one host or a single external observer; do not subtract unsynchronized wall clocks across machines.

## F. Go profiling

If the application exposes a pprof endpoint, capture CPU and heap profiles using its actual configured address:

```bash
go tool pprof -http=:8088 http://127.0.0.1:PORT/debug/pprof/profile?seconds=30
curl -o heap.pb.gz http://127.0.0.1:PORT/debug/pprof/heap
```

Do not add a public unauthenticated pprof listener to production. Bind it to loopback or an isolated management interface. The current suite does not enable pprof automatically.

## G. Results comparison

After running direct and Ashrix modes, compare matching result directories:

```bash
python3 benchmarks/scripts/compare.py \
  benchmarks/results/DIRECT_RUN_DIRECTORY \
  benchmarks/results/ASHRIX_RUN_DIRECTORY \
  benchmarks/results/comparison.md
```

Keep raw `.bin` and `.json` files. The generated comparison is a convenience summary, not a substitute for reviewing status codes, errors, resource metrics, and experimental conditions.
