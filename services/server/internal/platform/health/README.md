# Package `platform/health`

Status: built (M1-11). Normative are
[02-architecture.md](../../../../../docs/spec/02-architecture.md) section "Health" and
[16-operations.md](../../../../../docs/spec/16-operations.md) section "Two listeners" (R-AR-13,
R-AZ-22); the chapters win on any difference.

## Purpose

`/healthz` (the process answers) and `/readyz` (the process is ready) on the internal listener
`server.internal_listen_addr`. Health is outside the public API, needs no authentication and has
no action.

## Check names

Names are stable `snake_case`; they are the only detail an answer carries.

| Name | Endpoint | Fails when | Added by |
|---|---|---|---|
| `startup` | `/readyz` | the start is not complete (`SetStarted` not called) | built in |
| `shutting_down` | `/readyz` | the shutdown has begun (`SetShuttingDown`) | built in |
| `database` | `/readyz` | `pool.Ping` fails within `database.acquire_timeout` | `internal/wire` (M1-11) |
| `schema_requires_newer_server` | `/readyz` | `instance.min_server_version` is newer than the binary | M1-15 |
| `signing_keys` | `/readyz` | a purpose has no active signing key, or its private key cannot be read | M1-32 |
| `trusted_header_requires_loopback` | `/readyz` | `trusted_header` is active on a non-loopback listener | M1-43 |

## Answer format

- `GET` and `HEAD`; other methods `405` with `Allow: GET, HEAD`, other paths `404`, both with
  an empty body.
- `Content-Type: application/json`, `Cache-Control: no-store`.
- Success `200` with `{"status":"ok"}`; failure `503` with
  `{"status":"unavailable","failed":["<name>", ...]}`, names sorted. `HEAD` sends the status and
  headers only.
- Checks run concurrently with the request context; a panicking check counts as failed.

## Adding a check

Call `AddReadiness(name, check)` (or `AddLiveness`) in `internal/wire` with a new `snake_case`
name, add the row above and the name to 02 "Health". A check that asks a dependency bounds its
own wait. Registering may happen while the handler serves; an invalid, reserved or duplicate
name panics, because it is a programming error.

## Code

| File | Content |
|---|---|
| `health.go` | `Check`, `Registry` (`NewRegistry`, `AddLiveness`, `AddReadiness`, `SetStarted`, `SetShuttingDown`, `Handler`), the path and state constants |

## Dependencies

None in the project.
