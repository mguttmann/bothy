# Package `test/reconcile`

Status: built (M2-28). Component `testsupport`, never imported by production code.

The tests of the cost reconciliation (docs/spec/10-backend-policy.md section 12.4, ADR 0055) over
the real server: `internal/wire` with role `all` through `test/harness`, so River works the job
`billing.reconcile`, PostgreSQL 18 from `pgtest` with the server on `bothy_app`, HTTP on
`127.0.0.1` port 0, runs and estimates booked through `finishRun` (M2-22) and the app installed with
the manifest of `plugins/cost-reconciler` (`installApp`, `createToken`). The world wraps the use
case of the job in a gate, so a test sees the state between a request and its job.

## Tests

| Test | What it proves |
|---|---|
| `TestReconcile` | the integration cases of the brief, one after the other on one server: the owner path with step-up, `401 step_up_required` and the retry with the same key, app, agent and personal tokens refused with `owner_only` and audited (R-AZ-03), the app path and its audit details, one job per request through one handler, BR-10 on the server (every estimate in the period superseded once, remainder on the largest, the other day untouched), only inserts and no `UPDATE` or `DELETE` grant, unmatched runs, the no-op repeat and the correction, concurrent submissions, the totals of runs and sessions and their versions, the repeated job, validation, `Idempotency-Key`, an unknown member with a canary, `listCostEstimates` (R-AZ-28), `scope_missing` and the subject kinds |
| `TestReconcileApp_EndToEnd` | BR-10 end to end and the canary "the server stores no vendor key": the real app (`uv run --directory plugins/cost-reconciler --locked python -m cost_reconciler --once`) against a fake cost API serving the fixture pages of `plugins/cost-reconciler/tests/fixtures` (moved to the day of the runs) and the server behind a recording proxy; the shares are literals computed from the fixture; a random admin key appears in no table of `public` and `jobs` (one static statement with `query_to_xml`), no recorded request or response of the app's traffic, no server log line and no output of the app, while the fake received it in every request |

## Prerequisites

A Docker-compatible runtime for `pgtest` and `uv` on the `PATH` (it installs the app's locked
environment on first use). Without either the tests fail; they never skip.

```sh
cd services/server
go test -count=1 ./test/reconcile/...
```
