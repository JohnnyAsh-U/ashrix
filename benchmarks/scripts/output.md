# Ashrix benchmark comparison

Direct results: `../results/20261116T160523Z-direct`  
Ashrix results: `../results/20261116T163602Z-ashrix`  

Values are medians across repetitions. Latencies are milliseconds; throughput is completed requests/sec.

| Workload | Direct p50 | Ashrix p50 | p50 overhead | Direct p95 | Ashrix p95 | p95 overhead | Direct req/s | Ashrix req/s | Throughput change | Success direct | Success Ashrix |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| capacity-100rps | 0.662 | 5.211 | 4.548 | 1.114 | 6.667 | 5.553 | 100.04 | 100.02 | -0.02% | 100.00% | 100.00% |
| capacity-10rps | 1.044 | 5.425 | 4.381 | 1.368 | 6.600 | 5.231 | 10.05 | 10.05 | -0.02% | 100.00% | 100.00% |
| capacity-1rps | 1.006 | 5.822 | 4.816 | 2.003 | 27.912 | 25.909 | 1.05 | 1.05 | -0.03% | 100.00% | 100.00% |
| capacity-250rps | 0.666 | 3.520 | 2.853 | 1.052 | 6.095 | 5.043 | 250.04 | 249.98 | -0.02% | 100.00% | 100.00% |
| capacity-500rps | 0.576 | 2.693 | 2.116 | 0.761 | 4.133 | 3.371 | 500.03 | 499.99 | -0.01% | 100.00% | 100.00% |
| capacity-50rps | 0.699 | 5.504 | 4.805 | 0.968 | 6.771 | 5.803 | 50.05 | 50.03 | -0.03% | 100.00% | 100.00% |
| download-1024 | 0.919 | 5.499 | 4.580 | 1.121 | 6.519 | 5.399 | 50.05 | 50.03 | -0.02% | 100.00% | 100.00% |
| download-1048576 | 4.321 | 26.057 | 21.736 | 7.548 | 32.422 | 24.874 | 5.05 | 5.04 | -0.10% | 100.00% | 100.00% |
| download-10485760 | 18.719 | 72.150 | 53.431 | 56.799 | 111.229 | 54.431 | 1.05 | 1.05 | -0.28% | 100.00% | 100.00% |
| download-16384 | 1.027 | 6.028 | 5.001 | 1.361 | 7.138 | 5.777 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| download-262144 | 1.677 | 11.145 | 9.468 | 2.979 | 13.144 | 10.165 | 20.05 | 20.04 | -0.05% | 100.00% | 100.00% |
| health | 0.950 | 5.339 | 4.389 | 1.184 | 6.375 | 5.191 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| upload-1024 | 0.958 | 5.462 | 4.504 | 1.237 | 6.174 | 4.937 | 50.05 | 50.04 | -0.02% | 100.00% | 100.00% |
| upload-1048576 | 5.766 | 59.895 | 54.128 | 9.336 | 67.923 | 58.587 | 5.05 | 5.04 | -0.25% | 100.00% | 100.00% |
| upload-10485760 | 51.148 | 143.020 | 91.872 | 96.117 | 245.172 | 149.056 | 1.05 | 1.04 | -0.40% | 100.00% | 100.00% |
| upload-16384 | 1.138 | 6.335 | 5.197 | 1.390 | 7.321 | 5.932 | 50.05 | 50.03 | -0.03% | 100.00% | 100.00% |
| upload-262144 | 2.281 | 18.256 | 15.976 | 3.261 | 20.948 | 17.687 | 20.05 | 20.03 | -0.08% | 100.00% | 100.00% |

## Interpretation cautions

- Positive latency overhead means Ashrix was slower than the direct path for that workload.
- Negative throughput change means lower achieved throughput through Ashrix.
- Review raw reports for status codes, errors, and offered rate; low latency with failed requests is not a good result.
- Comparisons require equivalent hardware, topology, payloads, rates, TLS mode, and application behavior.
- Upload cases echo the body, so results include both inbound and outbound transfer.
