# Changelog

All notable changes to the cost reconciliation app (`plugins/cost-reconciler`) are documented in
this file. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this component adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Its version
is the [`VERSION`](VERSION) file next to this one; the app is no release component
(docs/spec/11-extensions.md section 10).

## [Unreleased]

## [0.1.0] - 2026-10-24

Summary: First version of the cost reconciliation app. It reads the daily costs your AI
provider bills and replaces the server's cost estimates with these real amounts, spread over
the runs in proportion to their estimates, so the cost views show what was actually billed.

### Added

- Cost reconciliation app (M2-28, docs/spec/10-backend-policy.md 12.4, ADR 0055): its own
  process, started as `python -m cost_reconciler --config <file> [--once]`, standard library only.
- TOML configuration with the keys `reconciler.server_url`, `reconciler.cost_api_url`,
  `reconciler.token_ref`, `reconciler.admin_key_ref`, `reconciler.min_request_interval`,
  `reconciler.lookback`, `reconciler.http_timeout` and `reconciler.workspace_map`; errors name
  the key, never the value.
- Secrets through `secret_ref` (`file:` with mode `0400` or `0600` owned by the process user,
  `env:`), read again at the start of every pass and redacted in every log line.
- Client of the provider's cost report: complete UTC days of the lookback, daily buckets grouped
  by workspace and description, all pages, at least `reconciler.min_request_interval` between
  requests, no redirects, error statuses without the body, a malformed page fails the pass
  before any submission.
- Exact cent sums with one conversion to micro units (half to even) and the distribution of
  BR-10: in proportion to the estimates, the remainder to the largest one, unassigned amounts
  logged as `not_assigned` with `no_mapping`, `no_runs` or `zero_estimates`.
- Client of `listCostEstimates` and `submitCostReconciliation` with a bearer app token and a
  UUIDv7 `Idempotency-Key` per submission; JSON log lines on stderr; exit codes `0`, `1`, `2`.
- Manifest `manifest.json` with exactly the scope `costs:reconcile`.
