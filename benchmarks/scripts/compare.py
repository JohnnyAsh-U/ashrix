#!/usr/bin/env python3
"""Compare matching Vegeta JSON reports from direct and Ashrix result folders."""
import json
import statistics
import sys
from pathlib import Path


def load(folder: Path):
    cases = {}
    for path in folder.glob("*.json"):
        label = path.stem.rsplit("-r", 1)[0]
        with path.open(encoding="utf-8") as stream:
            cases.setdefault(label, []).append(json.load(stream))
    return cases


def median_metric(reports, *keys):
    values = []
    for report in reports:
        value = report
        for key in keys:
            value = value[key]
        values.append(float(value))
    return statistics.median(values)


def fmt_ms(ns):
    return f"{ns / 1_000_000:.3f}"


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: compare.py DIRECT_RESULTS_DIR ASHRIX_RESULTS_DIR OUTPUT.md")
    direct_dir, ashrix_dir, output = map(Path, sys.argv[1:])
    direct, ashrix = load(direct_dir), load(ashrix_dir)
    labels = sorted(set(direct) & set(ashrix))
    if not labels:
        raise SystemExit("no matching JSON benchmark cases found")
    lines = [
        "# Ashrix benchmark comparison", "",
        f"Direct results: `{direct_dir}`  ", f"Ashrix results: `{ashrix_dir}`  ", "",
        "Values are medians across repetitions. Latencies are milliseconds; throughput is completed requests/sec.", "",
        "| Workload | Direct p50 | Ashrix p50 | p50 overhead | Direct p95 | Ashrix p95 | p95 overhead | Direct req/s | Ashrix req/s | Throughput change | Success direct | Success Ashrix |",
        "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
    ]
    for label in labels:
        d, a = direct[label], ashrix[label]
        dp50, ap50 = median_metric(d, "latencies", "50th"), median_metric(a, "latencies", "50th")
        dp95, ap95 = median_metric(d, "latencies", "95th"), median_metric(a, "latencies", "95th")
        dr, ar = median_metric(d, "throughput"), median_metric(a, "throughput")
        ds, ass = median_metric(d, "success"), median_metric(a, "success")
        change = ((ar / dr) - 1) * 100 if dr else float("nan")
        lines.append(
            f"| {label} | {fmt_ms(dp50)} | {fmt_ms(ap50)} | {fmt_ms(ap50-dp50)} | "
            f"{fmt_ms(dp95)} | {fmt_ms(ap95)} | {fmt_ms(ap95-dp95)} | "
            f"{dr:.2f} | {ar:.2f} | {change:+.2f}% | {ds*100:.2f}% | {ass*100:.2f}% |"
        )
    lines += ["", "## Interpretation cautions", "",
        "- Positive latency overhead means Ashrix was slower than the direct path for that workload.",
        "- Negative throughput change means lower achieved throughput through Ashrix.",
        "- Review raw reports for status codes, errors, and offered rate; low latency with failed requests is not a good result.",
        "- Comparisons require equivalent hardware, topology, payloads, rates, TLS mode, and application behavior.",
        "- Upload cases echo the body, so results include both inbound and outbound transfer.", ""]
    output.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {output}")


if __name__ == "__main__":
    main()
