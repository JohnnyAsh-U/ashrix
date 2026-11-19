# Ashrix Benchmarks

This directory contains performance benchmarks comparing direct
application access against access through Ashrix.

## Test Environment

All benchmark components ran on the same physical machine.

 | Component  |        Configuration|
 |---|---|
  |CPU       |         Intel Core i5-8365U @ 1.60 GHz|
  |RAM         |       16 GB|
 | Operating System |  Fedora Linux 44 Workstation|
  |Kernel     |        7.2.7-200.fc44.x86_64|
 | Go      |           1.26.8-X (linux/amd64, nodwarf5)|
 | Vegeta       |      12.7.0|
 | Ashrix      |       24fb8a3 |

## Test Setup

Two request paths were compared:

**Direct:**

```
Vegeta → Application Server (HTTP)
```

**Ashrix:**
```
Vegeta → Gateway (HTTP) → QUIC Tunnel → Connector → Application Server(HTTP)
```

Both paths used the same application server instance and workload
definitions.

The Gateway's client-facing endpoint used HTTP without TLS termination.

## Benchmark Configuration

  |Setting      |           Value|
  |---|---|
 | Test duration  |    20 seconds|
  |Warmup        |      5 seconds|
  |Repetitions   |              2|
  |Vegeta workers   |         100|

## Benchmark Results

**Direct results:** `results/20261119T154110Z-direct`

**Ashrix results:** `results/20261119T155734Z-ashrix`

Values are medians across repetitions. Latencies are in milliseconds;
throughput represents completed requests per second.


| Workload | Direct p50 | Ashrix p50 | p50 overhead | Direct p95 | Ashrix p95 | p95 overhead | Direct req/s | Ashrix req/s | Throughput change | Success direct | Success Ashrix |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| capacity-1000rps | 0.544 | 1.182 | 0.638 | 0.789 | 2.343 | 1.553 | 1000.03 | 999.98 | -0.01% | 100.00% | 100.00% |
| capacity-100rps | 0.889 | 5.093 | 4.204 | 1.085 | 6.354 | 5.269 | 100.04 | 100.02 | -0.02% | 100.00% | 100.00% |
| capacity-1500rps | 0.537 | 1.319 | 0.782 | 0.787 | 4.677 | 3.890 | 1500.02 | 1499.90 | -0.01% | 100.00% | 100.00% |
| capacity-2000rps | 0.508 | 1.905 | 1.398 | 1.109 | 6.171 | 5.063 | 2000.04 | 1999.92 | -0.01% | 100.00% | 100.00% |
| capacity-2500rps | 0.502 | 4.144 | 3.642 | 0.958 | 16.819 | 15.861 | 2500.06 | 2499.72 | -0.01% | 100.00% | 100.00% |
| capacity-3000rps | 0.450 | 6.055 | 5.605 | 0.958 | 39.161 | 38.203 | 3000.03 | 2999.33 | -0.02% | 100.00% | 100.00% |
| capacity-500rps | 0.639 | 1.718 | 1.078 | 0.859 | 2.545 | 1.686 | 500.03 | 499.99 | -0.01% | 100.00% | 100.00% |
| download-1024 | 0.915 | 5.261 | 4.345 | 1.133 | 6.275 | 5.143 | 50.05 | 50.04 | -0.03% | 100.00% | 100.00% |
| download-1048576 | 4.367 | 20.819 | 16.452 | 7.419 | 24.792 | 17.373 | 5.05 | 5.05 | -0.04% | 100.00% | 100.00% |
| download-10485760 | 19.646 | 59.685 | 40.039 | 58.797 | 77.619 | 18.822 | 1.05 | 1.05 | -0.23% | 100.00% | 100.00% |
| download-16384 | 1.045 | 5.446 | 4.400 | 1.292 | 7.117 | 5.825 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| download-262144 | 1.658 | 9.450 | 7.792 | 2.946 | 14.221 | 11.275 | 20.05 | 20.04 | -0.03% | 100.00% | 100.00% |
| health | 0.954 | 5.355 | 4.401 | 1.201 | 6.610 | 5.408 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| upload-1024 | 0.956 | 5.442 | 4.487 | 1.188 | 6.588 | 5.399 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| upload-1048576 | 6.791 | 53.399 | 46.608 | 10.924 | 60.159 | 49.234 | 5.05 | 5.04 | -0.23% | 100.00% | 100.00% |
| upload-10485760 | 50.435 | 115.886 | 65.450 | 86.045 | 165.265 | 79.220 | 1.05 | 1.05 | -0.28% | 100.00% | 100.00% |
| upload-16384 | 1.211 | 5.980 | 4.769 | 1.504 | 7.237 | 5.733 | 50.05 | 50.03 | -0.03% | 100.00% | 100.00% |
| upload-262144 | 2.506 | 15.870 | 13.364 | 3.826 | 18.744 | 14.918 | 20.05 | 20.04 | -0.06% | 100.00% | 100.00% |



## Memory Usage

Memory usage was monitored using Fedora System Monitor.

 | Component    | Idle Before Test  | Peak During Test |  15 Minutes After|
 |---|---|---|---|
 | Gateway        |         5.5 MB      |        35 MB      |        12 MB|
  |Connector      |         3.3 MB      |        28 MB        |     9.6 MB|

Values are approximate observations from System Monitor.

## Notes

-   Positive latency overhead means Ashrix was slower than the direct
    path.
-   Negative throughput change means lower achieved throughput through
    Ashrix.
-   Both paths reported 100% success across the tested workloads.
-   Upload tests echo the request body, so measurements include both
    inbound and outbound transfer.
-   All components ran on one physical machine. These results do not
    represent geographically distributed deployments.
-   Hardware, network conditions, transport configuration, and workload
    characteristics affect performance.

