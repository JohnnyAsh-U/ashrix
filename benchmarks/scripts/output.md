# Ashrix benchmark comparison

Direct results: `../results/20261116T212652Z-direct`  
Ashrix results: `../results/20261116T215008Z-ashrix`  

Values are medians across repetitions. Latencies are milliseconds; throughput is completed requests/sec.

| Workload | Direct p50 | Ashrix p50 | p50 overhead | Direct p95 | Ashrix p95 | p95 overhead | Direct req/s | Ashrix req/s | Throughput change | Success direct | Success Ashrix |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| capacity-100rps | 0.917 | 4.743 | 3.826 | 1.062 | 5.942 | 4.880 | 100.04 | 100.03 | -0.01% | 100.00% | 100.00% |
| capacity-10rps | 1.010 | 5.111 | 4.101 | 1.317 | 6.117 | 4.800 | 10.05 | 10.05 | -0.02% | 100.00% | 100.00% |
| capacity-1rps | 0.936 | 4.502 | 3.566 | 1.967 | 7.157 | 5.190 | 1.05 | 1.05 | -0.02% | 100.00% | 100.00% |
| capacity-250rps | 0.681 | 4.007 | 3.326 | 0.993 | 5.051 | 4.058 | 250.04 | 250.00 | -0.02% | 100.00% | 100.00% |
| capacity-500rps | 0.639 | 4.183 | 3.543 | 0.751 | 6.601 | 5.850 | 500.03 | 499.96 | -0.01% | 100.00% | 100.00% |
| capacity-50rps | 0.948 | 5.070 | 4.122 | 1.112 | 6.322 | 5.210 | 50.05 | 50.04 | -0.01% | 100.00% | 100.00% |
| download-1024 | 0.938 | 5.439 | 4.500 | 1.133 | 6.779 | 5.645 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| download-1048576 | 4.852 | 20.171 | 15.319 | 8.933 | 24.662 | 15.729 | 5.05 | 5.05 | -0.07% | 100.00% | 100.00% |
| download-10485760 | 19.298 | 137.601 | 118.303 | 56.308 | 165.649 | 109.341 | 1.05 | 1.05 | -0.57% | 100.00% | 100.00% |
| download-16384 | 1.068 | 5.583 | 4.515 | 1.362 | 7.730 | 6.368 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| download-262144 | 1.672 | 9.339 | 7.667 | 3.259 | 12.660 | 9.400 | 20.05 | 20.04 | -0.04% | 100.00% | 100.00% |
| health | 0.948 | 5.088 | 4.140 | 1.140 | 6.090 | 4.950 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| upload-1024 | 0.941 | 5.548 | 4.607 | 1.116 | 7.462 | 6.346 | 50.05 | 50.03 | -0.03% | 100.00% | 100.00% |
| upload-1048576 | 7.265 | 52.902 | 45.637 | 11.993 | 56.956 | 44.963 | 5.05 | 5.04 | -0.22% | 100.00% | 100.00% |
| upload-10485760 | 46.586 | 425.677 | 379.091 | 81.843 | 520.393 | 438.550 | 1.05 | 1.03 | -2.01% | 100.00% | 100.00% |
| upload-16384 | 1.200 | 6.190 | 4.990 | 1.483 | 8.599 | 7.115 | 50.05 | 50.03 | -0.03% | 100.00% | 100.00% |
| upload-262144 | 2.491 | 16.122 | 13.630 | 4.000 | 23.105 | 19.105 | 20.05 | 20.03 | -0.07% | 100.00% | 100.00% |

## Interpretation cautions

- Positive latency overhead means Ashrix was slower than the direct path for that workload.
- Negative throughput change means lower achieved throughput through Ashrix.
- Review raw reports for status codes, errors, and offered rate; low latency with failed requests is not a good result.
- Comparisons require equivalent hardware, topology, payloads, rates, TLS mode, and application behavior.
- Upload cases echo the body, so results include both inbound and outbound transfer.
