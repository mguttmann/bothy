<!-- SPDX-License-Identifier: Apache-2.0 -->
# tools/signalcheck

Gate `make signal-check` (M1-68): the files that query metrics use only what the signal
catalogue [signals.md](../../docs/observability/signals.md) declares, and every SLO threshold is
a bucket bound (R-OB-06).

| Rule | Checks |
|---|---|
| 1 | every metric row of tables 4.1 to 4.6 parses; the Prometheus column equals the translation rule of "Names and units" item 4; every histogram has exactly one row in 4.7 and 4.7 names only histograms; no attribute column names a person or ID attribute |
| 2 | every metric name in the `expr` of the dashboards (Prometheus targets), of the rule files and in `error_query`/`total_query` of the Sloth files is a catalogue name (histograms with `_bucket`, `_count`, `_sum`), a recording rule of the rule files, `target_info` (section 3), `up`, `ALERTS` or `ALERTS_FOR_STATE` |
| 3 | inside the braces of a catalogue metric every label is one of its attributes, a resource attribute, `job`, `instance`, or `le` on a `_bucket` series |
| 4 | every `le` matcher on a `_bucket` series is a bound of the 4.7 row of the histogram (numeric comparison) or `+Inf` |
| 5 | PromQL is lexed without a PromQL library (see the module docstring) |
| 6 | every dashboard query parses: it becomes a recording rule of a temporary file checked by the pinned `promtool check rules`; a failure names the dashboard and the panel |

Panels and targets whose datasource is not Prometheus (Loki) are skipped. Exit 0 prints one
summary line, exit 1 one line per finding (`<file>: <location>: rule <n>: <message>`).

```sh
make signal-check
uv run --project tools --locked --quiet python tools/signalcheck/signalcheck.py --root .
```

Not checked here, because `make ops-check` holds them (M1-04): the collector allowlist against
the catalogue, the SLO table against the Sloth files, alert thresholds against configuration
values.
