# sdkserver: real server of the SDK tests (`services/server/test/sdkserver`)

Status: built (M2-29). Component `testsupport` (M1-05), build tag `sdkserver`: never part of
`go test ./...`; `make lint` vets and lints it with the tag.

The real-server tests of the Python SDK (`sdk/python/tests/realserver`) run the scenarios AG-08
(adapter side), AG-09, AG-12, AG-28 and AZ-P6 against the server of this tree. The live files of the TypeScript SDK
(`sdk/typescript/test/live/`, M2-30) drive it the same way over its stdin protocol, and so does the live test of the
GitHub app (`plugins/github-app/test/live/`, M2-49) with the app ops below. sdkserver is that
server: the test `TestSDKServer` of a test binary, because `pgtest` and the harness need a
`testing.TB` (brief M2-29 Design note 16).

```sh
cd services/server
go test -c -tags sdkserver -o "$TMP/sdkserver.test" ./test/sdkserver
"$TMP/sdkserver.test" -test.run '^TestSDKServer$'
```

Run any other way, `TestSDKServer` skips, so `go test -tags sdkserver ./test/sdkserver/...` never
waits for stdin. It needs the Go tool chain and a Docker-compatible runtime (`pgtest`); without one
the binary exits with code 1 before it reads a line.

## What runs

- The real `internal/wire` graph with role `all` through `harness.Start` with `Options.Config`,
  on a disposable PostgreSQL of `pgtest`, listeners on `127.0.0.1` port 0, every rate-limit class
  raised (`harness.RaisedRateLimits`), outgoing requests refused (`EgressDenyAll`).
- A TCP forwarder on `127.0.0.1` port 0 in front of the public listener. Its origin is
  `server.public_url` of every server generation, so discovery, the gateway URL and the DPoP
  `htu` of the SDK stay valid across `restart`, and the SDK reconnects to the same address without
  a test seam. While the server restarts the forwarder closes every new connection at once.
- The world of 07 "Examples" as `harness.BuildMatrixWorld` builds it: Alex with `trigger` on the
  space Support (G-1), Lotse operator of Support with billing mode `organization` and tool class
  `chat`, Texter operator of Marketing, Sam and Robin, the other agents and grants. The owner is
  the first owner of the harness (handle `owner`); Robin is a person of the world.
- Lotse's API token is created by the owner through `createToken` with `dpop_jkt` set to the
  thumbprint the caller sends, so the SDK's DPoP runs against the real server.

## Protocol

One JSON object per line on stdin, `{"op": ..., <arguments>}`. Each request gets one answer line
on stdout, `SDKSERVER <json>`, in request order; every other line on stdout or stderr is noise
the reader ignores. A failed request answers `{"op": "error", "detail": <text>}`; the program
keeps serving. No token is ever printed: tokens go into files with mode `0600`, and every answer
is redacted against the tokens the program handled.

| Request | Answer | Effect |
|---|---|---|
| `start {config, agent_jkt, dir, app_front}` | `ready {origin, instance_id, agent_id, other_agent_id, token_file, config, front_port}` | Starts the server with `config` (a map of dotted keys of `docs/operations/configuration.md` to string values, merged over the raised rate limits; an unknown key fails the start), builds the world, writes Lotse's API token bound to `agent_jkt` to `token_file` in `dir` (an absolute, existing directory). `origin` is the forwarder origin (`http://127.0.0.1:<port>`, API base `origin + /api/v1`); `agent_id` is Lotse, `other_agent_id` Texter; `config` echoes the effective value of every key of the request. Once only. |
| `trigger {text}` | `trigger {run_id, thread_id, message_id}` | Alex posts a message without mention into the main thread of Support/Allgemein and opens a thread on it whose first message is `text` (`createMessage`, `createThread`). Lotse operates the channel, so the trigger decision (M2-04) creates the run and the delivery (M2-08) sends `agent.invocation` with a fresh token to every connected consumer of Lotse. `message_id` is the triggering message. |
| `forge_token {run_id, variant}` | `forge_token {token_file}` | Writes a forged invocation token for the run to a new file in `dir`. The base is a token of the issuer of M2-06 (`runsapp.InvocationTokens.Issue`) over the server's active signing key of purpose `invocation` and the grant of a delivery of the run now (`domain.DeliveryGrant`), so only the variant is wrong: `bad_signature` keeps header and payload and signs with a foreign Ed25519 key under the real `kid`; `alg_hs256` sets `alg` `HS256` with an HMAC keyed by the raw public key (algorithm confusion); `typ_access` sets `typ` `at+jwt`, signed with the real key; `aud_other_agent` is a real token whose `aud` is Texter. The issued tokens are never the run's current token. |
| `run {run_id}` | `run {state, state_reason, version, starts}` | Reads the run: `state`, `state_reason` (`null` when none), `version`, and `starts`, the number of changes into `running`. |
| `restart {consumer, acked_seq}` | `restarted` | Stops the server (`SIGTERM`: gateway sessions close, acknowledgements are flushed), sets `acked_seq` of Lotse's consumer `consumer` with `holder_session_id` `NULL` (a cursor rewound as after a crash; the next connection takes it over, 04 15.1), and starts a new server on the same database, secrets and signing keys behind the forwarder. The server then redelivers from `acked_seq + 1`, each event with a fresh token (M2-07, M2-08). An unknown consumer answers `error` after the new server runs. |
| `app_backend {port}` | `app_backend` | Makes the TLS front of `app_front` forward each accepted connection, TLS terminated, as plain TCP to `127.0.0.1:<port>`, the listener of the app under test; before this op the front closes every connection. Requires `app_front` at `start`. |
| `create_forum {name}` | `create_forum {channel_id}` | The owner creates a forum with `name` in the space Support (`createChannel` with kind `forum`); Alex holds `trigger` there through G-1. |
| `call {as, operation_id, path_params, query, body}` | `call {status, file}` | Performs the operation `operation_id` of `docs/api/openapi.yaml` through the real HTTP chain as `owner` (with a fresh step-up before the call) or `alex`: the route table (`api/routes`) gives method and path, `path_params` fill the path, `query` the query, `body` (JSON) is sent as it is; a `POST` gets an `Idempotency-Key`. Status and body are written as `{"status", "body"}` into a new file with mode `0600` in `dir`; the answer names status and file only, and a `secret` of the body (`createToken`) joins the redaction list. |
| `stop` | `stop` | Answers, then ends the program: the server stops, the database is dropped, `pgtest` removes its container and the process exits (code 0 when everything stopped cleanly). Wait for the exit. The end of stdin does the same without an answer. |

`starts` is counted in the `run.state_changed` events of the run in `seq` order (a `running`
event after a non-`running` one): every state change of a run writes exactly one in its
transaction (`internal/runs/README.md`), and `event_log` survives `restart`, unlike the metrics of
a process.

### App front

`app_front {host}` is optional (M2-29's and M2-30's clients send none of it). With it, before the
server starts, the program opens a TLS front on `127.0.0.1` port 0 with a leaf certificate for
`host` from a certificate authority created in memory; the egress clients of the apps module
(outgoing webhooks and requests to `interactions_url`, `harness.Options.AppsEgress`) trust that
authority and resolve `host` to `127.0.0.1` (`fakeapp.NewResolver`), while every other outgoing
client still refuses every target. The program adds `webhook.allowed_hosts` = `<host>:<front
port>` and `egress.allowed_hosts` = `<host>:<front port>;allow_private` to `config`, and `ready`
carries `front_port`. The server runs its jobs, so `runs.app_dispatch` and `apps.webhook_deliver`
reach the app through the front, the layout of a TLS proxy in front of an app in small.

## Timing values

The SDK and the server run in two processes with no shared clock, so the real-server tests use
real time with short values that the caller passes in the `config` of `start`. The keys are
`agent.invocation_token_ttl`, `auth.clock_leeway`, `agent.ack_timeout`,
`agent.ack_persist_interval` and `sync.heartbeat_interval`; the SDK side gets small
`sync.reconnect_backoff_*` values through `SdkSettings`.

Constraints:

- Each key accepts down to its `min` in `architecture/config-schema.json` (duration text, a whole
  number with one unit). `agent.invocation_token_ttl` must also be at least one second for the
  issuer, `auth.clock_leeway` must not be negative.
- `sync.heartbeat_interval` must stay below `agent.presence_timeout`
  (`presence_timeout_above_heartbeat_interval` in `internal/wire/configchecks.go`); no other
  cross-check of `internal/wire/configchecks.go` involves these keys.
- `adapter.invocation_token_leeway` is a key of the adapter that the server never reads: the
  caller keeps it at most `auth.clock_leeway` (09 4.5 "Keys and rotation" point 2) by setting it
  in `SdkSettings`.
- A run must start within `agent.invocation_token_ttl` of its delivery in the scenarios without
  renewal, and AG-28 waits `agent.invocation_token_ttl` plus `auth.clock_leeway` for the expiry.

The set the real-server tests use is `realServerTimings` in `protocol_test.go`;
`TestProtocolEndToEnd` starts the program with it, so the server accepts it.

## Tests

- `TestForwarder`: the forwarder passes connections to its target, closes new ones while it has
  none, ends open ones and reaches a new target under the same origin.
- `TestProtocolEndToEnd` runs this binary as the program and drives it like the Python side:
  `start` with the thumbprint of a P-256 key of the test, a REST consumer of Lotse with DPoP,
  `trigger`, the delivered `agent.invocation`, `run` (`queued`, no start), each `forge_token`
  variant refused by `startRun` with `403 invocation_token_invalid` and reason `invalid`, error
  answers for an unknown variant and an unknown op, `restart` with the cursor before the event
  (the redelivery carries a token with a new `jti`, the first token is now `superseded`), a start
  with the redelivered token (`running`, one start), `stop` and exit code 0. No token occurs in the
  output of the program.

```sh
cd services/server
go test -tags sdkserver -count=1 ./test/sdkserver/...
```
