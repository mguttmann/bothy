# cost-reconciler

Status: built (M2-28), version in [`VERSION`](VERSION), changes in [`CHANGELOG.md`](CHANGELOG.md).

## Purpose

The server books the cost of every agent run as an estimate from tokens and a price table
(docs/spec/10-backend-policy.md 12.1 to 12.3). The estimate misses surcharges, discounts and price
tiers. This app replaces the estimates with the amounts of the provider's daily cost report
(10 12.4, ADR 0055): it fetches the report with the provider's admin key, distributes each day's
amount of a provider workspace over the estimated runs of the agents mapped to that workspace and
submits the result to the server, which stores new `reconciled` entries next to the estimates.

It is its own process outside the server, off by default: nothing runs until an operator installs
and starts it. It uses only the public REST API (no SDK) and the Python standard library.

## Contract

- Flow and rules: docs/spec/10-backend-policy.md 12.4 (points 1 to 5, "Authorization") and the
  keys `reconciler.*` of 10 17; ADR 0055.
- Server operations: `listCostEstimates` (`GET /api/v1/costs/estimates`, action
  `cost.reconcile.read`) and `submitCostReconciliation` (`POST /api/v1/costs/reconciliations`,
  action `cost.reconcile.submit`), both with the scope `costs:reconcile` and without step-up
  (docs/api/openapi.yaml, docs/spec/05-actions.md 2.6 and 4.12).
- App manifest: docs/spec/11-extensions.md 3; secret references: docs/spec/16-operations.md 5.2;
  durations: docs/operations/configuration.md section 2.

## Security model

- The admin key of the provider organisation can manage members, workspaces and keys. It exists
  only in this process: it is read from the app's own secret provider and sent only in the header
  `x-api-key` of requests to `reconciler.cost_api_url`. It is never sent to the server, never
  logged and never part of an error message.
- The server sees only the app token, which carries the one scope `costs:reconcile`. That token
  reaches the two cost actions above and nothing else; it cannot run `reconcileCosts`
  (`cost.reconcile`), which needs an owner with step-up.
- No inbound port: the app only makes outgoing requests.
- No redirects: both HTTP clients treat a `3xx` answer as an error, so a header with the key or
  the token never goes to another host. Proxy settings of the environment are ignored.
- Both addresses must use `https`; plain `http` is accepted only for a loopback address (tests,
  a local proxy). TLS certificates are verified against the system's trust store.
- Secret values are wrapped in a type whose `str` and `repr` show `[redacted]`, and a logging
  filter replaces the current values of the token and the key in every log record.
- An error answer of the cost API is reported with its status only; its body is never read.

## Layout

| Path | Content |
|---|---|
| `cost_reconciler/__main__.py` | Command line, the loop of passes, signals and exit codes |
| `config.py` | TOML configuration and its checks |
| `secretref.py` | `secret_ref` values (`file:`, `env:`) and the redacting secret type |
| `transport.py` | HTTP opener without redirects and proxies; the pacer of the request interval |
| `costapi.py` | Client of the provider's cost report: window, pages, checks of every page |
| `amounts.py` | Exact cent sums and their one conversion to micro units |
| `server.py` | Client of `listCostEstimates` and `submitCostReconciliation` |
| `distribute.py` | The distribution rule (BR-10), pure |
| `reconcile.py` | One pass |
| `logs.py` | JSON log lines on stderr |
| `uuid7.py` | UUIDv7 values for `Idempotency-Key` |
| `manifest.json` | The app manifest (`app_manifest.v1`) |
| `tests/` | Unit and process tests with a fake cost API and a fake server (`fakes.py`) |
| `tests/fixtures/` | Cost report pages written after the provider documentation ([README](tests/fixtures/README.md)) |

## Setup

1. **Install the app.** An owner installs `manifest.json` with `installApp`
   (`POST /api/v1/apps`, a fresh step-up and a `reason`), granting the scope `costs:reconcile`:
   `{"manifest": <manifest.json>, "granted_scopes": ["costs:reconcile"], "reason": "Cost reconciliation"}`.
   The manifest asks for no events, commands, components or grants.
2. **Create the token.** The owner creates the app's token with `createToken`
   (`POST /api/v1/principals/{principal_id}/tokens` for the app's principal). The secret is shown
   once.
3. **Store the secrets.** As the service user that runs the app, store the token and the
   provider's admin key as files with mode `0600` (or `0400`), owned by that user, for example
   with `umask 077` before writing them. A file with another mode or owner is refused.
4. **Write the configuration.** One TOML file; the keys of the table `[reconciler]`:

   | Key | Default | Meaning |
   |---|---|---|
   | `server_url` | required | Base address of the server; the API lies below `/api/v1` |
   | `cost_api_url` | required | Base address of the provider's API from the documentation cited in 10 12.4 |
   | `token_ref` | required | `secret_ref` of the app token |
   | `admin_key_ref` | required | `secret_ref` of the provider's admin key |
   | `min_request_interval` | `60s` | Least time between two requests to the cost API, also across passes |
   | `lookback` | `3d` | This many complete UTC days before today are reconciled; a whole number of days |
   | `http_timeout` | `30s` (the default of `sdk.http_timeout`) | Time limit of one HTTP request |
   | `workspace_map` | none, at least one entry | Sub-table: provider workspace ID to the list of agent IDs |

   Durations use the format of docs/operations/configuration.md section 2 (`500ms`, `60s`, `1m`,
   `3d`). `secret_ref` is `file:<absolute path>` (recommended) or `env:<NAME>` (allowed, not
   recommended: child processes and crash reports inherit the environment). Unknown keys, a
   missing required key, an empty map, an agent listed twice and an agent ID that is no UUID are
   configuration errors that name the key, never the value.

   ```toml
   [reconciler]
   server_url = "https://chat.example.com"
   cost_api_url = "https://costs.example.com"
   token_ref = "file:/etc/cost-reconciler/token"
   admin_key_ref = "file:/etc/cost-reconciler/admin_key"
   min_request_interval = "60s"
   lookback = "3d"

   [reconciler.workspace_map]
   wrkspc_example1 = ["0192f4a0-7b1c-7250-807f-c268a7b21797", "0192f4a0-7b1c-7250-807f-c268a7b21798"]
   wrkspc_example2 = ["0192f4a0-7b1c-7250-807f-c268a7b21799"]
   ```

   Recommended is one provider workspace per agent with its own key (10 12.4 point 1): the cost
   report groups by workspace, not by key.
5. **Run it** from this folder:

   ```sh
   uv run --locked python -m cost_reconciler --config /etc/cost-reconciler/config.toml
   uv run --locked python -m cost_reconciler --config /etc/cost-reconciler/config.toml --once
   ```

   Without `--once` the app runs passes until `SIGTERM` or `SIGINT`, which end it after the
   current request, and waits `reconciler.min_request_interval` between passes. With `--once` it
   runs one pass. A schedule (cron, a timer unit) that runs `--once` must not start the app more
   often than `reconciler.min_request_interval`, because the interval is kept only inside one
   process. Secrets are read again at the start of every pass, so a rotation needs no restart.

### Authentication: bearer only

The app sends its token as `Authorization: Bearer <token>`; it does not use DPoP. By default
`auth.dpop_required_for` names only agents, and the token reaches only the two cost actions. If an
operator adds `app` to `auth.dpop_required_for`, the server refuses the app's requests and the app
cannot work.

## One pass

1. Read the token and the admin key.
2. Fetch the cost report of the window from the start of today (UTC) minus `reconciler.lookback`
   days to the start of today: daily buckets (`bucket_width=1d`) grouped by `workspace_id` and
   `description`, every page. Every page is checked before anything is submitted; a page without
   the documented fields fails the pass.
3. Per bucket, workspace and currency, sum the cent strings exactly and convert once to micro
   units (one cent is ten thousand micro units, rounded half to even).
4. Per bucket in ascending order: read the estimates of all mapped agents for the bucket's day
   once (`listCostEstimates`, all pages), distribute each workspace's amount, and submit all
   entries of the bucket in one request (entries sorted by run ID, provider `anthropic`,
   `period_start` and `period_end` the bucket bounds, a new UUIDv7 as `Idempotency-Key`). A bucket
   without entries is not submitted.
5. The first failure ends the pass; there are no retries inside a pass. The next pass submits the
   same days again; the server treats a run whose reconciled amount is unchanged as a no-op, so a
   repeat has no second effect.

### Distribution (BR-10)

For the amount T of one workspace, day and currency and the estimates e_i of the runs of the
workspace's agents:

- Runs in another currency are left out and logged as `currency_mismatch`; runs with estimate `0`
  are left out and stay estimates; runs of another provider are left out.
- Each remaining run gets floor(T times e_i divided by the sum of e); the remainder goes to the run
  with the largest estimate, ties to the smallest run ID. The shares sum to T exactly. Integer
  arithmetic only.
- Not assigned (logged as `not_assigned` with the amount, never submitted):

  | Reason | When |
  |---|---|
  | `no_mapping` | the workspace has no entry in `reconciler.workspace_map`, also the default workspace (`workspace_id` `null`) |
  | `no_runs` | no run of the mapped agents in the amount's currency that day |
  | `zero_estimates` | the runs in the amount's currency all have estimate `0` |

This is how the app reports an amount without a matching run as unassigned (other usage in the
workspace, 10 12.4 point 3).

## Log events

JSON lines on stderr with `ts` (RFC 3339 UTC with milliseconds), `level`, `event` and fixed
fields; nothing goes to stdout. No line holds a header, a token, a key or a body.

| Event | Fields |
|---|---|
| `pass_started` | `window_start`, `window_end` |
| `pass_finished` | `bucket_count`, `submission_count` |
| `pass_failed` | `stage` (`cost_api`, `estimates`, `submit`), `error` (`status`, `network`, `malformed`, `interrupted`), `status`, `code`, `reason`, `errors` (pointer and code), `bucket_start` |
| `not_assigned` | `bucket_start`, `workspace_id`, `currency`, `amount_micros`, `reason` |
| `currency_mismatch` | `bucket_start`, `workspace_id`, `currency`, `run_ids` |
| `submitted` | `bucket_start`, `entry_count`, `reconciled` |
| `unmatched_runs` | `bucket_start`, `run_ids` (run IDs the server could not match) |
| `config_error` | `key`, `rule` |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | `--once`: every submission succeeded; loop: ended by `SIGTERM` or `SIGINT` |
| `1` | `--once`: the pass failed (cost API, estimates, a submission, or interrupted) |
| `2` | configuration error: the file, a key, or a secret that cannot be read safely (also a file with an open mode). In the loop a secret error fails only that pass |

## Limits

- Costs of the provider's priority tier are missing from the cost endpoint (10 12.4 point 2), so
  they are never reconciled.
- One provider only. Other providers need their own reconciliation app (ADR 0055).
- The cost endpoint reports only daily buckets; a run with turns on two days is reconciled per
  day.
- A negative amount (a credit) in the report is outside the documented shape and fails the pass.

## Tests

```sh
uv run --locked pytest --cov --cov-report=term
uv run --locked ruff format --check .
uv run --locked ruff check .
uv run --locked mypy
```

The tests never call a real service. `tests/fakes.py` holds a fake cost API and a fake server,
both `http.server.ThreadingHTTPServer` on `127.0.0.1` port 0 that record every request, and a
fake clock, so no test waits real time for the request interval. Admin keys and tokens in the
tests are random canary values created at run time; `tests/test_canary.py` checks that the key
reaches the cost API in every request and appears in no request to the server, no log line and no
error. `tests/test_doc_gate.py` proves that the docstring gate of ruff fails on an undocumented
public function.
