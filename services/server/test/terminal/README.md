# services/server/test/terminal: conformance suite of the terminal

The tests of [24-terminal.md](../../../../docs/spec/24-terminal.md) section 14 "Tests" run here
(brief M2-48). Scenario files with literal expectations from chapter 24 and the chapters it names
run end to end against the real parts of the terminal: the server image built from the worktree
with `services/server/Dockerfile`, real host services `bothy-host` in Linux containers with
systemd, and a Go test device that enrols with device keys and DPoP, confirms step-ups with its
device key and speaks Noise through the host service's own `internal/noise` and `internal/frame`.
The suite adds no product behaviour; a non-conformance it finds is fixed in the owning module.

Everything here belongs to the architecture component `testsupport` and is never imported by
production code.

## Layout

| Path | Content |
|---|---|
| `catalogue.yaml` | the list of places: for every ID TM-T01 to TM-T29 and every AZ-M, ID-T and SEC-T ID of [threat model](../../../../docs/security/threat-model.md) 6.7, its runs, the implementing tickets and the threats |
| `profiles.yaml` | the stack profiles `default`, `mobile` (the phone suite of M5-13 through `clientstack`, a quick reattach window of at least three patiences of the test kits), `limits` and `twonode`: server keys and host-service keys with their test values and why each is needed |
| `scenarios/<id>.yaml` | one scenario file per row or matrix (`tm-t03.yaml`, `az-m54-61.yaml`, `id-t28.yaml`, ...) |
| `suite/` | package `suite`: loads catalogue, profiles and scenarios, checks them, builds one world per case and runs the steps (the conductor) |
| `stack/` | package `stack`: preflight, server and host images, test CA, TLS front, the Compose project and the host containers |
| `device/` | package `device`: the Go test device (HTTP, DPoP and the WebSocket of the device stream) and the driver of the Noise sidecar [`services/host/test/noisedevice`](../../../host/test/noisedevice/main.go) |
| `e2e/<package>/` | one test package per stack: `conformance`, `hostpolicy`, `limits`, `twonode`, `restart`, `hostplatform`; each with `main_test.go` (`TestMain` with one stack) and `scenarios_test.go` |
| `fakehost/` | M2-44's Fake-Host ([README](fakehost/README.md)); the suite uses it for the manipulated host service of TM-T11 |
| `multinode/`, `*_test.go` | the server-level tests of M2-46 and M2-47; they stay as they are, and the catalogue names them as runs of kind `unit` |

The packages split the scenarios as follows: `conformance` (profile `default`) TM-T03 to TM-T10,
TM-T12, TM-T17, TM-T19, TM-T20, TM-T22, TM-T23, TM-T28, AZ-M54 to AZ-M61 and ID-T28;
`hostpolicy` TM-T21, TM-T24 and TM-T29; `limits` (profile `limits`) TM-T11; `twonode` (profile
`twonode`) TM-T15; `restart` TM-T16 on a stack of its own, because it restarts the server;
`hostplatform` the platform suite of `services/host/test/platform` in a fresh host container.
TM-T16 also runs under load in `make perf` (`TestChaosTMT16` in
[`test/chaos`](../chaos/README.md), build tag `perf`).

## Catalogue

`catalogue.yaml` has `format: 1` and one entry per ID with `id`, `title` and `expected` (the
cells of the row, literally), `runs` (each `{kind, ref, os}`), `implemented_by` (tickets and the
merged tests that recorded the red run), `deferred` (`{ticket, reason}`, only for IDs outside M2
exit criterion 8), `deferred_parts` (`{part, ticket}` for parts another ticket proves) and
`threats` (T-TM IDs of threat model 6.7). Run kinds:

- `e2e`: `e2e/<package>:scenarios/<file>.yaml` or `e2e/<package>:<TestFunc>`, relative to this
  folder.
- `platform`: a test of `services/host/test/platform` with `os` one of `linux`, `macos`,
  `windows10`, `windows11`; the `linux` runs execute in `make check` through `e2e/hostplatform`.
- `perf`: the chaos test of `make perf`.
- `unit`: `<package dir>:<TestFunc>` for Go, `<file>.dart:<test name>` for Dart.

`make docs` runs [`tools/docs/terminal_catalogue.py`](../../../../tools/docs/terminal_catalogue.py)
`--check`: the TM-T entries equal the table of 24 section 14; every ID of the PLAN row M2-48 and
of exit criterion 8 has a run in `make check` and no `deferred`, TM-T16 a `perf` run, and the
`doctor` matrix platform runs on macOS, Windows 10 and Windows 11; every T-TM row of threat model
6.7 names a test that exists and the catalogue lists the threat; every `ref` exists.
`suite/catalogue.go` decodes the same file strictly for the Go tests (`TestCatalogue_Load`).

## Profiles

`profiles.yaml` keys every value by the dotted name of
[configuration.md](../../../../docs/operations/configuration.md); the suite never names a number
outside this file. Server keys reach `api`, `worker` (and `api-2`) as `BOTHY_*` variables through
the `env_file` of the terminal overlay; host-service keys go into `/etc/bothy-host/config.yaml` of
every host container. The loader (`suite/profiles.go`) checks the relations the scenarios rely on
and names both keys when one breaks (for example `terminal.session_token_ttl` plus
`auth.clock_leeway` below `terminal.open_timeout`, `terminal.quick_reattach_window` below
`terminal.detach_grace`). `auth.session_max_age` is never shortened, because it would also end the
session of the stack's bootstrap owner.

## Scenario files

A scenario file is decoded strictly (known fields only). Top level: `id` (equals the file name),
`title`, `sources` (the places of the specification), `profile`, `world` and `cases`.

**World.** `people` (names such as `alex`, `sam`, `robin`; the display name is the name with a
capital letter; the case's Owner `owner` always exists), `owners` (made owners with `addOwner`),
`devices` (`{name, person}`: a second enrollment), `groups` (`{name, members, expires_in}`),
`agents`, `apps`, `hosts`, `grants` (`{name, subject or group, host, expires_in}` of the right
`terminal`) and `fake_hosts` (`{name, assigned_to, assign, never_idle}`, TM-T11). A host is
`{name, assigned_to, llm_credentials, credential_holder, declared_server_host, session_users,
assign, config, setup, pair}`: `session_users` are installed with `bothy-host install` and
`session-user add`, `assign` maps people to session users in `confirmHost`, `config` adds
host-service keys on top of the profile, `setup` is a root script run after install, and `pair`
is one of `paired` (default), `unconfirmed`, `unpaired`, `installed` or `container`.

Every case gets its own instance of the world: fresh people with unique handles, created by the
stack's bootstrap owner, and its own host containers, so no case can disturb another. Cases run
with `t.Parallel()`; `hosts` in a case limits the hosts it starts. A test constant bounds the
number of host containers that start at once.

**Steps.** Each step has exactly one kind:

| Kind | Form | Values |
|---|---|---|
| call | `as: <principal>`, `call: <operationId>`, `params`, `with`, `if_match`, `step_up` | an operation of `docs/api/openapi.yaml`; method and path come from there, `params` fill the path, `with` is the body, `if_match` reads a path for its ETag first, `step_up` confirms a step-up before the call |
| device | `device: <name>`, `do: <op>`, `session`, `on`, `profile`, `workdir_index`, `text`, `token`, `ephemeral`, `from` | `enroll`, `step_up`, `open` (on a host), `attach`, `connect` (the device stream with `terminal.hello` and `terminal.identify`), `handshake` (Noise message 1 with the session token and message 2; `token` selects the own token, the token `of` another session or a `forge` of `foreign_kid` or `invocation_typ`; `ephemeral: other` hands another ephemeral key), `start`, `send` (`text`), `expect_output`, `replay_message1` (message 1 of connection `from`), `cut` (the TCP connection without a close frame), `close`, `wait_close` |
| host | `host: <name>`, `do: <op>`, `run` or `script`, `user`, `to`, `text` | `exec` (as root, the operator), `exec_as` (as `user`), `restart_service` (`bothy-host restart`), `doctor`, `pair` (`bothy-host pair` with the code in `text`), `reboot` (`docker restart` of the container), `pause`, `unpause`, `net_down`, `net_up` (the container leaves and rejoins `termhosts`), `clone_state` (the state directory into the container `to`) |
| server | `server: <op>`, `service`, `mode`, `sql`, `bypass` | `restart` (`docker compose restart` of `service`, default `api`), `front_mode` (`single`, `hash`, `split`), `sql` (read only; a write needs `bypass: true` and is allowed only in `tm-t28.yaml`) |
| wait | `wait: <key>` or a list of keys | waits for the sum of the profile values of the configuration keys; the next step polls |
| check | `check: <kind>` with `session`, `host`, `user`, `marker`, `device` | see below |
| parallel | `parallel: [<steps>]` | runs its steps at once; `expect.statuses` compares the statuses of parallel openings as a multiset |

`expect` holds the literal expectations; every call and stream step has one, and every case ends
with a check. A call compares `status`, `code`, `reason`, `errors` (`{code, pointer}`), `fields`
(JSON pointers to literal values), `absent` and `lengths`; a stream step compares `close`
(`{code, reason}`), `expect_output` the `output` text and `replay` (one `replay_begin` and
`replay_end` pair); a host command compares `exit`, `exit_not`, `stdout_contains`, `stdout_lacks`
and `stderr_contains`. `keep` binds names to JSON pointers of the answer (or columns of a `sql`
row) for later steps.

**Checks.** Each check polls until it holds or the bound of the test passes, except `markers`,
`canary` and `readyz`, which read once:

| Check | Reads | Expectations |
|---|---|---|
| `session` | `getTerminalSession` as the device of the session | `state`, `close_reason`, `close_detail`, `exit_code` |
| `host` | `getHost` as the case's Owner; `hosts.disabled_reason`; `systemctl show` of `bothy-host.service`; the journal of the units | `state`, `connected`, `doctor` and `doctor_lacks` (`{check, ok, code, code_in, session_user}`), `reported_session_users`, `session_users`, `disabled_reason`, `no_keys`, `unit` (`n_restarts`, `exec_main_status`, `active`), `journal` (codes) |
| `audit` | `audit_events` of the case's `actor` and `resource` since the start of the case | `action`, `decision`, `deny_reason`, `details` (keys and codes, never a value), `count`, `at_least` |
| `processes` | `ps` in the host container for the session user `user` and the `marker` argument | `shells`, `processes`, `markers`, `users` |
| `markers` | the folder `/var/tmp/termsuite-markers` of the host container, where every program that must never start leaves its name | `markers` |
| `canary` | the device's recorded binary messages, `compose logs`, a database dump, the collector export and the host journal, scanned with `test/canary` | `output` (the positive control), `findings` |
| `metric` | the rise of a counter in the collector export since the start of the case | `metric`, `attrs`, `rise` |
| `readyz` | `bothy healthcheck` against `/readyz` in every api container | `ready` |

**References.** Strings may name values of the case: `${<name>}` (a `keep` value),
`${<name>.id}` (a person or device, agent, app, host, Fake-Host, group, grant or session),
`${<person>.device}` and `${<person>.session}`, `${<host>.pairing_code}`, `${<host>.bad_code}`,
`${<host>.fp_prefix}` and `${<host>.wrong_prefix}`, `${marker...}` (any name that starts with `marker`:
a random marker argument, the same within the case), `${canary:<part>}` (canary parts of `test/canary`), `${now+<duration>}` and
`${random:<name>}` (a UUID that names no object). Codes, tokens and keys behind references stay
in memory and never appear in a failure message.

A failed expectation names scenario, case, step index and the expected and observed codes, states
and counts, never a body, a token or terminal output.

## Running

Every e2e package needs the Docker runtime below; the unit tests of `suite`, `stack` and `device`
need none (except `TestComposeTerminalOverlay`, which needs `docker compose`, not a running stack).
From `services/server`:

```sh
go test -count=1 ./test/terminal/suite/ ./test/terminal/stack/ ./test/terminal/device/
go test -count=1 ./test/terminal/e2e/conformance/
go test -count=1 -run 'TestScenarios/tm-t08/own_host' ./test/terminal/e2e/conformance/
```

The `-run` pattern is `TestScenarios/<scenario>/<case>`. Each package starts its own stack in
`TestMain` (the image builds included) and removes it at the end, also when a test fails; on a
failure it first prints `compose ps --all` and the last log and journal lines, filtered to codes.
`make check` runs every package through `make test`.

## Prerequisites

Docker Engine and Compose at the minimum versions of M1-73
([infra/compose/test/README.md](../../../../infra/compose/test/README.md)), `docker buildx`, a
Docker host on cgroup version 2 that starts privileged containers (the host containers run
systemd as process 1), and `kernel.yama.ptrace_scope` of at least `1` on the Docker host (HD10 of
the host service refuses the start below it with `ptrace_scope_low`). The preflight fails with a
message that names the fix; nothing is skipped (P-09, R-QA-02). With colima or a Docker VM,
`TMPDIR` must lie in a folder the VM shares, as for the smoke test.

The stack binds only `127.0.0.1`: the one published port is the TLS front's, with a random host
port. Keys, tokens, pairing and enrollment codes of the throwaway stack stay in memory or in files
of mode 0600 in its temporary folder and are deleted with it.

## Platform runs

The host-local parts (the `doctor` matrix of M2 exit criterion 8, the PTY and process tree tests,
TM-T13 and the platform-only parts of TM-T22, TM-T24, TM-T28 and TM-T29) are the platform suite
[`services/host/test/platform`](../../../host/README.md). `e2e/hostplatform` builds its Linux
bundle with `make -C services/host platform-bundle OUT=<dir>` and runs it as root in a fresh host
container in every `make check`. On disposable macOS, Windows 10 and Windows 11 test machines the
same bundle (built for that platform) runs as administrator; its result file lists every test.

The platform test binary refuses to run without the marker file `/etc/termsuite-disposable`
(Windows `%ProgramData%\termsuite-disposable`): it prints `platform: not a disposable test machine
(marker file missing)` and exits `2`. The host image carries the marker; on a test machine the
build session creates it before the run, and nowhere else.

## Cached images

The server image `termsuite-server:<12 hex>` and the host image `termsuite-host:<12 hex>` stay as
local build cache under their content-addressed tags. `docker image ls 'termsuite-server'` and
`docker image ls 'termsuite-host'` list them, `docker image rm <tag>` removes one; the next run
builds it again ([infra/compose/test/README.md](../../../../infra/compose/test/README.md)).
