# agents: conformance suite of the agent contract (`services/server/test/agents`)

Status: built (M2-31). Component `testsupport` (M1-05), Go package `agents`, only `_test.go`
files, so nothing of it reaches the product binary.

The mandatory agent scenarios AG-01 to AG-32 of docs/spec/09-agents.md 16.2 run here data driven
against the real server: one scenario file per test ID under `scenarios/`, one subtest
`TestConformance<N>/<ID>` per file (the scenarios round-robin over `TestConformance1` to
`TestConformance4`, so the CI test shards can split the suite) and one below it per case. The conductor (Go) starts the real
`internal/wire` graph in the test process through the shared harness (`services/server/test/harness`,
M1-71) on its own PostgreSQL 18 from `pgtest` with a fake clock, acts for the people through the
public API, checks the stored state through `AdminDB` and drives one Fake-Agent process per agent
and consumer. The Fake-Agent (`bothy_sdk.testing.fake_agent` in `sdk/python`) is a scriptable
adapter on the Python SDK; only the public API lies between the two sides (ADR 0022
"Confirmation").

## Running

```sh
cd services/server
go test -race -count=1 ./test/agents/...                            # every scenario
go test -race -count=1 -run 'TestConformance/AG-14' ./test/agents/   # one scenario
```

The suite needs a Docker-compatible runtime (`pgtest`) and `uv` on `PATH`: the Fake-Agent runs
with `uv run --project <repository root>/sdk/python --locked --quiet python -m
bothy_sdk.testing.fake_agent`, with `PYTHONPATH` on `sdk/python/src` (the SDK is not an installed
package), after one `uv sync --project sdk/python --locked` per test binary. Without either it
fails with a message that names what is missing; it never skips (plan P-09). Every listener binds
`127.0.0.1` and port 0 (P-10).

## Files

| File | Content |
|---|---|
| `main_test.go` | `TestMain` (pgtest templates), the operations of `docs/api/openapi.yaml`, the configuration keys of `docs/operations/configuration.md`, the repository root |
| `scenario_test.go` | the scenario format (Go types), the strict loader, `Validate`, the row expansion; `TestScenarioFiles_Load`, `TestScenarioFiles_NoSecretsAndWhiteLabel` |
| `conformance_test.go` | `TestConformance1` to `TestConformance4` |
| `conductor_test.go` | the step execution: human calls, Fake-Agent ops, conductor actions, parallel groups, repeats, async calls, keeps, references and expectations |
| `checks_test.go` | the state checks through `AdminDB`; `expectations_test.go`: `TestExpectations_Bite` |
| `fakeagent_test.go` | process manager and control client of the Fake-Agent; `fakeagent_unit_test.go`: its process tests |
| `proxy_test.go` | the fault proxy; `proxy_unit_test.go`: its tests |
| `world_test.go` | the world extensions of the matrix world |
| `fixtures_test.go` | the generated upload fixtures (`voice_m4a`, `text_note`) and `run_jobs` of the file jobs; `files_unit_test.go`: their tests and the column limit of the check `file` |
| `registry_test.go` | coverage of 09 16.2 and the deferred golden list |
| `scenarios/ag-NN.yaml` | one file per test ID |
| `testdata/deferred.golden` | the scenario parts later tickets bring |

## Scenario format

The Go types in `scenario_test.go` and `Validate` are the format; there is no separate schema.
Decoding is strict (unknown keys fail). Top level:

- `id` (equals the file name), `title` (the row text of 09 16.2), `sources` (chapter references).
- `world: matrix`: `harness.BuildMatrixWorld` plus `world_test.go`: the tool classes of 07 10
  (Lotse `chat`, Texter `scoped`, Runner `host`, Assistent `scoped`) through `setAgentToolClass`,
  and the `extras` `tracker` (the app Tracker with scope `messages:write`, its grant G-6 `write` on
  Support/Eskalation and its DPoP-bound token through `installApp`, `createGrant`, `createToken`)
  and `webhook` (an incoming webhook of Tracker on Support/Eskalation, its token only in memory).
- `config`: dotted keys of `docs/operations/configuration.md` with test values; every key a step
  moves the clock by is set here. The conductor adds the raised rate limits of the harness.
- `setup`: steps before every case. `shared_world: true` runs all cases on one server in order.
- `agents`: name to `{agent, consumer, transport, launch, runtime, settings}`; `transport` is
  `rest` (default) or `gateway`; `launch: later` starts the process only at a step `do: launch`;
  `runtime` is the actual `billing_mode`, `billing_evidence`, `tool_class`, `capabilities` and
  `model` (defaults: the configured ones); `settings` are SDK keys (`adapter.*`, `sdk.*`).
- `cases`: a list of `{name, rows, steps}`. `rows` repeats the steps once per row: a scalar that
  is exactly `{{name}}` takes the row's value with its type, `{{name}}` inside a text its text
  (the matrix of AG-02 and AG-12).
- `deferred`: `{part, ticket, reason}` of the parts a later ticket brings.

Step kinds, exactly one per step (`note` is a free comment):

- `as: <subject>` with `call: <operationId>` and `with`: a call of `owner`, `alex`, `sam`, `robin`
  (the harness clients, DPoP, `Idempotency-Key`, step-up when asked), `tracker` (the app token) or
  `webhook` (`postIncomingWebhook` with the world's token). `with` holds the path parameters by
  name, `query`, `if_match` (`current` reads the ETag of the resource first), `merge: true` (the
  body is the current document with the given members, for `putChannelBehavior`) and the body
  members; client IDs and the `reason` of administrative operations are added when missing.
  `fixture: <name>` takes the bytes of a generated upload fixture (`fixtures_test.go`, never a file
  of the repository): `uploadPart` sends them as its body with `Content-Digest`, `createUpload`
  gets their `size_bytes` and `sha256`. `voice_m4a` is an MP4 audio file (`ftyp` brand `M4A `,
  `mvhd` with timescale and duration, one `trak` with `hdlr` `soun` and an `mp4a` sample entry),
  `text_note` a small text that asks for something. Every
  answer is validated against `docs/api/openapi.yaml` (M1-31) and must hold no JSON `null`.
  `async: <name>` starts the call in the background; a later `await: <name>` takes its answer.
- `agent: <name>` with `do: <op>` and `with`: an op of the Fake-Agent (below), or `launch`.
- `server:` `advance_clock` (`by`: keys of `config`, their sum plus one millisecond),
  `run_jobs` (`kind`: `runs.queued_expiry`, `runs.cancel_timeout`, `runs.stall_check`,
  `instance.key_rotation`, through `harness.Server.RunDueJobs`; and the file jobs `files.scan`,
  `files.thumbnail`, `files.transcribe`, which River works at once on the wall clock: the step
  waits, bounded by `testkit.Patience`, until no job of the kind is open and fails on a discarded
  one, unless the harness lists the kind in `ManualJobKinds`), `snapshot_cursors` (`name`),
  `restart` (`restore_cursors: <name>`), `proxy` (`drop_next: <operationId>`), `dropped`
  (`drop_next`: the answer the proxy kept, for `keep`) and `fixture` (`with.kind`
  `imported_session` through `agentstest.InsertImportedSession`, `imported_message` through
  `messagingtest.InsertImportedMessage`; only for state no operation writes).
- `check:` `run`, `message`, `thread` (one row by `id`), `runs`, `messages`, `followup`,
  `component`, `interaction`, `session`, `events`, `audit`, `consumer`, `commands`,
  `signing_keys` (rows by `where`: columns or dotted paths into JSON columns, `<absent>` for
  NULL), and `file` (one row by `id`, only `upload_state`, `scan_status`, `transcript_status`
  and `transcript_source`; the loader refuses any other column, so no check reads a transcript
  or a file name). `eventually: true` polls until the check holds (asynchronous effects such as the
  aggregated denials of the audit log), bounded by `testkit.Patience`.
- `parallel:` steps released together behind a barrier, with an optional `tally` (`match`,
  `count`); `repeat: {times, steps}`.

`expect` holds literal data: `status`, `code`, `reason`, `errors` (`{code, pointer}`), `none`,
`one_of`, `count` and `rows` (set checks), and any other member by dotted path (`\.` is a dot in a
member name, `${name}` puts a kept ID into a path). Values: `<absent>`, `<present>`, `<=n`, a
map (partial match), a list (exact length), a scalar (compared as text). `keep` binds names to
members of the answer or of the first row. References: `$name` (a kept value), `$name.member`,
`$name+n` (a kept number plus n), `${name}` inside a text, and the world names `$channel.<Space/Channel>`,
`$main.<Space/Channel>` (its main thread), `$thread.<Space/Channel>` (the world's thread),
`$space.<Space>`, `$user.<Name>`, `$agent.<Name>`, `$app.Tracker`, `$grant.<G-n>`, `$group.<Name>`,
`$message.<Space/Channel>.<n>`. Nothing in the suite computes an expectation with server code.

A failed expectation names the scenario, case, step, the expected and the observed value; every
text that looks like a token is replaced by `<redacted>`.

## Fake-Agent control protocol

JSON lines. The conductor writes one request per line to the stdin of the process and reads one
reply per line from its stdout, in request order; the Fake-Agent handles one request at a time.
Logs go to stderr at level `WARNING` and never contain a token or a credential.

Request: `{"id": <int>, "op": <text>, "args": {...}, "now": <RFC 3339 UTC with milliseconds>}`.
`now` is the harness clock; the Fake-Agent sets the wall time of its SDK clock to it before it acts
(token expiry, `iat` of DPoP proofs, the JWKS `max-age`; the clock never goes back). Waits of the
SDK (retry and reconnect backoff) use real time with the small values the conductor passes.

Reply: `{"id": <same>, "ok": true, "result": {...}}`, or `{"id": <same>, "ok": false, "error":
{"kind": <kind>, "detail": <text>}}` with `kind` `usage` (unknown op, malformed line, missing
`hello`, bad argument), `transport` (no HTTP answer) or `internal`. A malformed line answers with
`id` 0; after an error the process keeps serving. An HTTP answer is never an error: `4xx` and
`5xx` are results with `status` and the problem members `code`, `reason` and `errors`.

Token names: `inbox` (default) is the newest token of the run (09 4.3: a redelivery replaces it),
a kept name refers to `keep_token` of `receive` and `renew` or to `import_token`, `empty` sends an
empty text, `absent` leaves the member out, `forged:<variant>` is built from the newest token
(`bad_signature`, `alg_hs256`, `alg_none`, `typ_access`, `bothy_sdk.testing.token_faults`).
`jti_hash` is the first 16 hex characters of the SHA-256 of a token's `jti`; the checks compare
`runs.invocation_jti`, `started_jti` and `renewed_from_jti` only in this form. Every op that sends
a request of a run binds it to the run's token (`Invocation-Token`, 07 SR9) unless `bind: false`.

| Op | Arguments | Result |
|---|---|---|
| `hello` | `base_url`, `gateway_url`, `instance_id`, `agent_id`, `consumer`, `transport`, `credentials` (`token`, `dpop_private_jwk`), `runtime`, `settings` | `{}` |
| `connect` | none | `head_seq` (REST: the first poll creates the cursor at the head, and the events a poll of a known consumer delivers wait for `receive`; gateway: `identify` to `ready`) |
| `receive` | `type`, `run`, `accept` (default true), `hold`, `keep_token`, `skip_own_checks`; the conductor adds `head` | `event`, `id`, `seq`, `redelivered`, `run_id`, `thread_id`, `message_id`, `interaction_id`, `command_id`, `request_id`, `trigger_kind`, `session_mode`, `verdict`, `jti_hash`, `finished`; or `none: true` |
| `verify` | `event` (a held event) or `token` (a name) with `run` | `verdict`, `jti_hash` |
| `start`, `resume` | `run`, `token`, `billing_mode_used`, `tool_class_used`, `billing_evidence`, `model`, `skip_own_checks` | problem members, `run_state`, `run_version`, `replay`; or `refused` (the adapter's own check sent nothing) |
| `renew` | `run`, `token`, `retry` (false: one attempt), `keep_token` | problem members, `jti_hash` |
| `context` | `run` | problem members, `trigger_complete`, `truncated_before_seq`, `elements` (`message_id`, `role`, `trust`, `truncated`, `bytes`, `marked_only`, `attachments`: `file_id`, `transcript`, and `content_marked_only` for a text read with `read_file`, `transcript_marked_only` and `transcript_in_trigger` for a transcript), `by_message`, `context_bytes` |
| `patch` | `run`, `ops`, `base_version` (`stale`), `rebase` (default true) | problem members, `current_version`, `rebased` |
| `await_input`, `finish` | `run`, `status_line`; `state`, `reason`, `summary` | problem members |
| `heartbeat` | `active_runs` (default: the started, unfinished runs), `capabilities`, `billing_mode`, `billing_evidence`, `tool_class` | problem members |
| `post` | `thread`, or `channel` with `new_thread`, `title`, `parent_message_id`; `content`, `run`, `card` (`approval` or `buttons` with `buttons`, `single_use`, `restricted_to`) | problem members, `message_id`, `thread_id`, `component_ids` (by `custom_id`) |
| `respond`, `update_component` | `interaction`, `content`; `component` and the members of `updateComponent` | problem members |
| `set_title` | `thread`, `title` (with the `ETag` of `getThread`) | problem members, `title_source` |
| `report_session` | `thread`, `run`, the members of `reportAgentSession` (`If-Match` of the current session) | problem members |
| `register_commands` | `commands` (declarations) | problem members of the first refusal, `registered`, `command_id` |
| `autocomplete_mode` | `mode` (`answer` or `ignore`), `choices` | `{}` |
| `read_file` | `file`, `run` | problem members, `bytes` (the length; the content never leaves the process) |
| `submit_transcript` | `file`, `run`, `text`, `language`, `if_match` (default: the `ETag` of `getFile`; `absent` sends none) | problem members |
| `export_token`, `import_token` | `run` or `token`; `token`, `name`, `run` | `jti_hash` (and `token` only in `export_token`) |
| `crash` | none | none: the process exits at once with status 3, without a request or a flush |

`receive` consumes events of its consumer (REST pages of `pollAgentEvents`, or dispatch frames of
the gateway); events of other types wait in a backlog. A token event (09 4.4) is verified with the
SDK verifier, stored and acknowledged, unless `accept` is false, `hold` is true or the verdict is
`unknown_key` (09 4.5: neither accepted nor acknowledged). An event without a token is
acknowledged once received; the acknowledgement covers only settled events. A REST `receive` that
reaches the head the conductor read from the event log without a match answers `none: true`; a
gateway `receive` waits for the event. Well-behaved default (09 4.9): after a verdict `invalid`,
`billing_mode_mismatch` or `tool_class_mismatch` the Fake-Agent calls `finishRun` with `failed`
and the matching reason, unless `skip_own_checks` or `accept: false` is set.

## Deferred parts

`testdata/deferred.golden` lists the scenario parts that need a later ticket, one line
`<ID> <part> <ticket>`; `TestScenarioRegistry_DeferredGolden` compares it with the `deferred`
entries of the files. The ticket of a line adds its part to the scenario file (a case or steps,
with the expectations of 09 16.2 written out) and removes the line in the same change.

## Time, jobs and restarts

One fake clock per case; the conductor moves it only between steps. Jobs run only on request
(`run_jobs`), because River schedules by the wall clock. A server restart is emulated in the test
process: `restart` stops the wire graph gracefully and starts it again on the same database; with
`restore_cursors` the stored cursors go back to a snapshot, the state of a process that died
before it stored later acknowledgements. Every client reaches the server through the fault proxy
of the case, whose origin is `server.public_url`, so DPoP proofs stay valid across a restart.
