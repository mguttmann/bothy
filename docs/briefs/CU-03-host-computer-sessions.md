# CU-03: Host service: computer sessions and frames

Status: ready | Depends on: none (specification merged) | Milestone: CU

## Goal

After this ticket `bothy-host` conducts computer sessions: the frame types of `docs/spec/27-computer-use.md` 6.3 with their directions, the session kind `computer` with the Noise prologue `bothy-computer/1` and the host checks of 5.3, the host control ops `computer.open`, `computer.close`, `computer.disable`, `computer.started`, `computer.state`, `computer.ended` and the `computer` object of `report`, the enablement mirror, one controller per host, the pause while locked, the coordinate mapping with scale IDs, image chunking at the Noise limit, the local channel to the helper with the peer rule of 9.4, the doctor section `computer`, and a Linux test backend of the helper (Python, Xlib and XTest) for the end-to-end test.

## Read first

- `docs/spec/27-computer-use.md` sections 3 to 9 (all), 12.3, 15 (CU-T07 to CU-T16, CU-T23), 16, 18.
- `docs/spec/24-terminal.md` 2.2 (host service, doctor, install), 3.5 (control connection), 5.2 to 5.4 (Noise, token checks, frames), 6.1 and 6.4 (opening, confirmation endpoint and its peer rule), 10.3 (close details).
- `services/host/testdata/computer/channel-vectors.json`.
- Code on the base branch: `services/host/internal/frame`, `internal/noise`, `internal/session` (`manager.go`, `verify.go`, `handshake.go`, `dataconn.go`), `internal/control` (`frames.go`, `sessions.go`, `data.go`), `internal/confirm` (`server.go`, the console-user rule), `internal/localchan` (peer credentials), `internal/doctor`, `internal/config`, `cmd/bothy-host`, `docs/api/asyncapi.yaml` (channels `hostControl` and `hostData`).
- Design note `docs/design/computer-use.md` (context and evidence; where it differs from the chapters, the chapters win); ADR 0078; `docs/decisions.d/CU-01.md`; `docs/security/threat-model.md` 5.18 and 6.10.

## Fixed requirements that apply

Owner's fixed build rules (apply verbatim):

1. No MVP. New from scratch and directly in the final state; stages are only the order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined
   interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface with a docstring/doc
   comment (purpose, parameters, return value, errors, example where useful); comments explain the
   why, not the what. API documentation is generated from code (OpenAPI from the routes, dartdoc,
   Python API docs in MkDocs).
4. Documentation in the repo: README per module/service, ADR for every architecture decision,
   CHANGELOG per release, keep the documentation site current. A change without matching
   documentation is not done.
5. Quality gates (SonarQube, linters, type checking, security scanners, test coverage) block the
   merge.

Rules for every brief:

6. Test first: write the failing test, run it and keep the red output for the report, then make it green, then clean up.
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them. Use a hyphen or rephrase.
8. White label: examples, templates and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no internal script names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) changes in this ticket only where Scope names it.
11. Additive only: every new key defaults to off or to the behaviour before, and an installation that sets none of the new keys (`runner.computer_use`, `host_service.computer_use`) and enables no helper behaves exactly as before.
12. Full operation (owner ruling OR-1, D-CU-01-1): build no app tier, no key or app block list, no confirmation per session or action and no lock on the Bothy windows; the only protections are those of `docs/spec/27-computer-use.md` section 1 item 3.

Note on rule 1: "stages" are the milestones and the stages A1, AR and CU of the plan (PLAN P-27, P-29, P-30).

Product requirements implemented here:

13. `frame.Decode` takes the session kind: on a `computer` session it accepts exactly `close` and the types `0x20` to `0x25` with their directions and contents; on a terminal session it refuses them, and the reverse (CU-T09). The terminal behaviour is unchanged byte for byte (the existing frame and recording tests stay green).
14. Session kind `computer`: the data connection `/api/v1/hosts/computer-sessions/{session_id}/stream` after `computer.open`, the checks of 5.3 in their order with their close codes and details, `cu_state` after message 2, `computer.started`, `computer.state`, `computer.ended` with action counts, app identities (bounded by the server value it got) and `paused_ms`; the attach token verified with the JWKS of purpose `terminal` and `typ = computer-session+jwt`.
15. Enablement: the helper's `hello` and `state` become the `report` object `computer` (27 12.3, with `locked`, and `helper: refused` after a refused peer); `computer.disable` goes to the helper as `disable`; with `host_service.computer_use` off nothing listens and `report` says `helper: absent`. `computer.state` carries `action_count` and `front_app`, sent on every state change and at most every `computer.status_interval` (from `settings`) while actions run.
16. One controller per host: a lock held for the life of a session; a second `computer.open` answers `computer.ended` `open_failed` with `busy`.
17. Pause: `locked` from the helper sets `paused`, drops pending actions, sends `release`, `cu_state` and `computer.state`; actions answer `screen_locked`; the unlock resumes.
18. Coordinates: a `scale_id` per distinct geometry from `done.screen`, kept for the session; actions map back with the factor and round inside the screen; an unknown or outdated `scale_id` answers `stale_screenshot`, a point outside the image `invalid_action` (CU-T11, CU-T12). `wait` and `cursor_position` are answered by the host service itself (`wait` up to `computer.wait_max`, then a capture).
19. Images: `cu_result`, then `cu_image_begin`, `cu_image_data` frames of at most 65518 bytes and `cu_image_end` (CU-T10); images only in memory, never on disk; the automatic image after `computer.settle` when `auto_screenshot` is on.
20. Ends: every end path of 27 8.3 that the host sees (`done`, Esc, indicator stop, `computer.close`, disable, local switch-off, lost connection, stream fault, helper crash) sends `release` and `hide`, `cu_state` `stopped` and `computer.ended` once (CU-T16 host part).
21. Local channel (27 9.4): endpoint only with `host_service.computer_use`, created by `install`; the configuration is refused with `host_service.computer_use` and `host_service.owner_account` together; on Linux only the test peer of `host_service.computer_test_peer` (empty admits nobody); the peer rule (console user, not the service user or a session user, executable at the installed helper path whose file and folder chain belong to a trusted owner as in HD10, and the code signing requirement of the build when it carries one); refused peers are logged with `computer_peer_refused` (CU-T23); the codec reads every channel vector and refuses frames above `host_service.computer_frame_max_bytes`.
22. `bothy-host doctor` prints a section `computer` (installed, trusted path, signed or development build, connected, switched on, each permission); it is no HD check, because the server accepts exactly HD1 to HD11 in `report` (24 3.5), and the same state reaches the server only in the `report` object `computer`.
23. Linux test backend `services/host/test/computer_backend/` in Python with Xlib and XTest (python-xlib), speaking the channel protocol over the same endpoint as the helper, plus a fixture app with known buttons; used by the Compose end-to-end test of CU-06.
24. Configuration keys `host_service.computer_use`, `host_service.computer_frame_max_bytes` and `host_service.computer_test_peer` in the host configuration with their defaults of 27 section 18.
## Scope

All paths under `services/host/` unless they start with `docs/` or `tools/`.

Create:

- `internal/computer/` (session, enablement, controller lock, scale map, chunking, channel server, peer rule) with tests; `test/computer_backend/` (Linux test backend and fixture app, `README.md`).
- `docs/decisions.d/CU-03.md`.

Change:

- `internal/frame` (computer types, kind-aware decode), `internal/session` and `internal/control` (the new ops and data path), `internal/config` (the two keys), `internal/doctor` (section `computer`), `internal/install` (endpoint folder), `cmd/bothy-host`, `services/host/README.md`, `docs/spec/27-computer-use.md` where the code needs a precise name, `docs/operations/configuration.md` through `make config-reference`.

## Out of scope

- The native helpers (CU-02), the server (CU-04), the MCP server (CU-05), the runner and the Compose test itself (CU-06), the clients (CU-07).
- Any change of terminal sessions, profiles, the PTY helper or the owner account mode.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/CU-03-host-computer-sessions`, created from the branch of the stage that holds CU-01; one writer per worktree.
- Conventional Commits with scope and a DCO sign-off on every commit (`git commit -s`), no co-author trailers.
- Do not push and do not open a pull request: the stage lands as one pull request with CU-01 to CU-07 and one CI round (PLAN P-30, `AGENTS.md`); the coordinator merges your branch into the stage branch. Never push to `main`, never tag, never create remote repositories.
- Do not modify files outside the scope above, except the CHANGELOG fragments, README and `docs/decisions.d/` entries named in this brief, each listed in the report.
- Follow the host rules supplied by the launcher (ports, machine rules); no GUI or input test on a maintainer's machine and no run with a real model; native GUI tests run only in CI jobs.
- Tests bind only `127.0.0.1` and port 0; PostgreSQL and other services run only as throwaway containers. Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, `t.TempDir()` or `tmp_path`, removed at the end.
- Never read or print secrets. Tokens, attach tokens, DPoP keys, signing material and screenshots never appear in logs, test output, failure messages, audit entries or files outside the places this brief names.
- No real model call and nothing that costs money; a check of the pinned CLI runs only without a login against a local stand-in for the API on `127.0.0.1`.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs beyond what Scope names, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before its dependencies were built, under the rule of D-W-100: where the merged code of a dependency names a package, type, function, file, operation or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from `docs/spec/27-computer-use.md`, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Start check.** Run `make check` on the fresh worktree; if it is red, stop with `Verdict: BLOCKED base branch red`. Confirm `docs/spec/27-computer-use.md` sections 5.3, 6.3 and 9.4, else `Verdict: BLOCKED specification missing`.
2. **Reuse, do not fork.** The Noise handshake, the JWKS handling, the `jti` memory and the close-code path of terminal data connections are reused with a parameter for prologue and token type; the server side builds the matching ops in parallel (CU-04) from 27 12.3, so keep the JSON member names exactly as there.
3. **Signer check without cgo.** Read the peer's executable path from its process ID without cgo (macOS `kern.procargs2`, Windows `QueryFullProcessImageNameW`), check the path chain like HD10, then the signature: macOS by running `/usr/bin/codesign --verify --strict -R=<requirement>` on that path, Windows with `WinVerifyTrust` and the subject of the signer through `golang.org/x/sys/windows`. The requirement comes from `buildinfo` at build time; an empty one means a development build.
4. **Race of lock and action.** The helper checks the lock state right before every injection (CU-02); the host service additionally drops actions queued before a `locked` state. Test both orders with the fake channel.
5. **Endpoint owners the helpers check (CU-02, 27 9.4 "Host side").** The macOS socket's folder chain is root-owned and not writable by group or others, the endpoint folder belongs to root or the service user, and the socket itself to the service user; on Windows the host service creates `\\.\pipe\bothy-host-computer` with its own account as the owner (`O:SY` or the service SID in the security descriptor), because a helper of a standard user cannot read a service's token and checks the pipe's owner instead. A pipe owned by Administrators is refused by the helper.
6. **Rulings for `docs/decisions.d/CU-03.md`:** package names, the scale ID scheme, the number of kept geometries (a named key if it is a limit), the backend's dependencies, every name that differs from chapter 27.

## Tests (write them first)

- Go unit tests: frame encode, decode and directions per kind (CU-T09); chunking edges (CU-T10); scale mapping with factors 1, 2, 1.25, 1.5 and a non-zero origin (CU-T11); `stale_screenshot` and `invalid_action` (CU-T12); pause and resume with key release (CU-T13); the host checks of 5.3 one by one with close code and detail (CU-T07), the terminal token on a computer connection and the reverse (CU-T08); every end path with `release`, `hide` and exactly one `computer.ended` (CU-T16 host part); the peer rule with a fake process table (CU-T23); the channel vectors; the doctor section `computer`; the refused combination with `host_service.owner_account`.
- An integration test of the host service against the test server of `internal/control` tests with a fake helper on the real endpoint in a temporary folder.
- Python tests of the Linux backend against Xvfb where available in CI.

```sh
(cd services/host && go test -race -count=1 ./internal/frame/... ./internal/computer/... ./internal/session/... ./internal/control/... ./internal/doctor/...)
make config-reference && make config-doc-check
make check
```

## Acceptance criteria

- [ ] CU-T07 to CU-T13, CU-T16 (host part) and CU-T23 are green; the existing terminal tests and recordings are unchanged.
- [ ] Without `host_service.computer_use` nothing listens and the `report` says `helper: absent`.
- [ ] The Linux backend drives the fixture app in Xvfb through the channel.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- An entry `changelog.d/CU-03.md` next to the `CHANGELOG.md` of every component you changed (`docs/quality/gates.md` "Fragments"); rulings in `docs/decisions.d/CU-03.md`, one row each with date, ruling and reason; every name that differs from chapter 27 is a ruling and a change of chapter 27 in the same commit.
- An ADR only if you take an architecture decision beyond ADR 0078 and this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green for the components you changed.
- An independent review of your diff by a fresh reviewer (one follow-up round at most); record its verdict. The red team of the stage runs once over the whole stage diff before the pull request; it checks at least, for this ticket: no computer frame is accepted on a terminal session and the reverse; the checks of 5.3 cannot be passed with a terminal token, a replayed token or another run's token; only the trusted helper reaches the channel; every end releases every key; no image touches the disk.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. The review verdict and the findings fixed.
4. What is not proven (at least: the real helpers on macOS and Windows (CU-02) and the whole path with the server and the MCP server (CU-06)), each probe of `docs/spec/27-computer-use.md` section 16 you ran with its result, and each difference between this brief and the merged names of its dependencies.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
