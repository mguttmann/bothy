# M8-10: Login and API client of the import tool

Status: ready | Depends on: M1-38, M1-71 | Milestone: M8

## Goal

After this ticket the import tool can talk to a Bothy server as its owner without ever holding step-up: `bothy import login --server <url>` creates a device with `platform = cli` (an Ed25519 DPoP key, no step-up key), starts a device pairing (`beginPairing`), prints the pairing code alone on standard output, polls `completePairing` until the owner approves the code in the app or the web console (`approvePairing` with step-up there) and stores device ID, DPoP key and tokens in a session file that only the user can read; `bothy import logout` ends the session on the server and deletes the file. A `bothyapi` package gives the later tickets an authenticated client of the API generated from `docs/api/openapi.yaml`: every request carries a DPoP proof (RFC 9449, `htu` from the server URL, `ath`, nonce handling with one retry on `use_dpop_nonce`), an expired access token is refreshed once under a file lock, the rotated refresh token is written to the session file before it is used, `429` of the server is honoured with `Retry-After` within the tool's wait limits, and no token, key or session file content ever reaches output, log or error text. Why: the tool reads and writes only through the public API (22 2, R-IM-01), the dry run must be able to read Bothy, and a `cli` device is the only way a tool logs in (08 6.4); the step-up steps stay in the web console (22 8, D-W-124).

## Read first

- `docs/spec/22-import-migration.md`: 2, 4.3 item 5 (Bothy `429`), 8 (the paragraph on the `cli` session and the owner steps in the web console), 10.1 (rows `bothy import login` and `logout`), 10.2 rules 4 and 5 (no secret in output; the dry run may refresh and rewrite only the session file), 14, 19.
- `docs/spec/08-identity.md`: 5.1 (registration fields), 5.2, 5.3 (device pairing), 6.4 (command-line tools: no step-up key, no step-up), 7.1, 7.2, 7.3 (DPoP: algorithms, nonces, `htu`, `ath`, errors), 7.4 (refresh rotation, reuse grace, reuse revokes the family), 13 (rate limits of public auth endpoints), ID-T29.
- `docs/spec/06-api.md`: "Code generation" (clients are generated from `openapi.yaml`), 7 (idempotency keys for the tool's writes).
- `docs/spec/02-architecture.md` 3.3 (row `bothy import login|logout`), 6, 7.2.
- `docs/spec/19-security.md` 8 (egress), SEC-TLS-4.
- `docs/api/openapi.yaml`: `beginPairing`, `completePairing`, `refreshToken`, `logout`, `getMe`, schemas `DeviceRegistration`, `PairingStarted`, `PairingCompleteRequest`, `TokenResponse`, `DevicePlatform`, parameter `DpopProof`, the `401` problem for DPoP.
- `docs/DECISIONS.md`: D-W-100, D-W-120, D-W-124; section 10 format.
- Briefs (merged names win, D-W-100): `M1-33-identity-devices-dpop.md` (`kernel/jose`, the DPoP checks this client must pass, `platform = cli` without step-up key), `M1-34-identity-sessions.md` (refresh families, reuse grace), `M1-38-identity-device-approval.md` (pairing operations and states), `M1-17-api-server-stubs.md` (oapi-codegen pinned in `go.mod`, the `api` component), `M1-22-egress-client.md`, `M1-71-contract-tests.md` (the real-server harness `harness.Start`, `harness.NewUser`), `M8-01-discord-read-client.md` (tool layout, the rate-limit guard).
- Code on the base branch: `services/server/api`, `services/server/internal/kernel/jose`, `services/server/internal/platform/egress`, `services/server/cmd/bothy`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory (tests pass a session file path inside `t.TempDir()`).
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. The tool logs in only as a device with `platform = cli`, without a step-up key, through device pairing; it never asks for or stores a password, passkey or step-up (08 6.4, R-ID-17).
12. DPoP on every authenticated request and on the pairing calls: `typ = dpop+jwt`, `alg = EdDSA`, `jwk` without private part, `jti`, `htm`, `htu` (origin of the server URL plus path, without query and fragment), `iat`, `ath` with a token, `nonce` once the server sent one (08 7.3).
13. Refresh rotation: the new refresh token is stored before the new access token is used; a refresh happens under an exclusive lock of the session file so two tool processes never rotate the same token (08 7.4: a reuse would revoke the session).
14. The session file is created with mode `0600` in a directory with mode `0700`; a file or directory that group or others can read or write is refused (on Windows the file lives in the user profile and the mode check is skipped, documented). Its content never reaches output, log or error.
15. Every connection goes through `platform/egress`: `https` only, except a loopback server (`127.0.0.0/8`, `::1`, `localhost`) for which `http` is allowed; the server host is the only allowed host of the client and may resolve to a private address (the owner names it explicitly); no environment proxy; redirects are not followed.
16. Server `429`: wait `Retry-After` and retry at most `import.discord.max_429_retries` times, abort when the wait is longer than `import.discord.max_rate_wait` (22 4.3 item 5).
17. Clients are generated from `openapi.yaml`; no hand-written request or response type for an operation of the contract (06 "Code generation", owner rule 3).

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `api/restclient/oapi-codegen.yaml` and the generated `api/restclient/restclient.gen.go` (Design note 1); a `go:generate` line in `api/generate.go`.
- `cmd/bothy/importtool/bothyapi/`: `session.go` (session file format, load, atomic store, lock), `dpop.go` (proof builder over `kernel/jose`), `transport.go` (the `http.RoundTripper` chain of note 3), `client.go` (`New(ctx, Options) (*Client, error)` returning the generated client with the transport, plus `Me(ctx)` and `Limits(ctx)` helpers), `login.go` (`Login`, `Logout`), `errors.go`, `doc.go`, `README.md`.
- `cmd/bothy/importtool/bothyapi/bothytest/`: a fake Bothy for the auth paths (note 6); only tests import it.
- `cmd/bothy/import.go` (or the file layout the merged `cmd/bothy` uses for subcommands): the command group `bothy import` with `login` and `logout` (note 5); `discord` follows in M8-04.
- Tests named under "Tests".

Change:

- `docs/spec/22-import-migration.md` 19: new tool key `import.login_poll_interval` (default `5s`, "interval between polls of `completePairing` during `bothy import login`; the server limits public auth calls by `ratelimit.rl.public_auth.*`"), and the session file and lock in 10.1; `make config-reference`, `make config-doc-check`.
- `services/server/api/README.md` (the client package), `services/server/README.md` (the import commands), `services/server/CHANGELOG.md`; `docs/DECISIONS.md` section 10 rows `D-M8-10-1` to `D-M8-10-7`.

## Out of scope

- `bothy import discord` and its options, the tool configuration loader (`--config`, environment `BOTHY_IMPORT__...`) beyond what `login` needs (M8-04 owns the loader; this ticket reads `import.login_poll_interval`, `import.discord.max_429_retries` and `import.discord.max_rate_wait` through a small `Limits` struct that M8-04 fills from its loader; until then the defaults of 22 19).
- Any step-up, any `import.manage` call (web console, M8-08).
- Device approval in the apps or the console (M1-38, M4, M6).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-10-import-login-client`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`, `build(api): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord or any other real service: tests use only fixtures and fakes on `127.0.0.1` with port 0 (D-W-124); the real-server test uses the harness of M1-71 with `pgtest` (P-09, P-10). Without a Docker-compatible runtime that test fails; it never skips.
- Never write a session file outside `t.TempDir()` in a test; never read the developer's real configuration directory.
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Generated client** (06 "Code generation"; Ruling for docs/DECISIONS.md). `api/rest` holds the server of the enabled operations and filters the spec by the enable list, so a client of all operations cannot share its request body types. Generate a separate package `api/restclient` with the oapi-codegen pinned in `go.mod` directly from `docs/api/openapi.yaml` with `generate: {client: true, models: true}`, `output-options: {skip-prune: true}`, package `restclient`, output `restclient.gen.go`, no embedded spec. `make codegen-check` regenerates it. The tool uses `ClientWithResponses` with its own `HTTPClient` (the egress client's `*http.Client` with the transport of note 3). If oapi-codegen v2.8.0 cannot generate a part of the spec as a client (report which), stop with `Verdict: BLOCKED` rather than writing that part by hand.

2. **Session file** (08 6.4, 7.4; Ruling for docs/DECISIONS.md). Default path `<os.UserConfigDir()>/bothy/import-session.json`, overridable with `--session-file` on every `bothy import` command. JSON `{version: 1, server_url, device_id, user_id, dpop_private_jwk, access_token, access_expires_at, refresh_token, refresh_expires_at}`. Store atomically: write a temp file in the same directory with mode `0600`, `fsync`, rename, `fsync` the directory. Lock: an exclusive advisory lock on `<file>.lock` (`flock` on Unix, `LockFileEx` on Windows) around every read-refresh-store sequence; after acquiring it, re-read the file, and when another process already rotated the tokens use those instead of refreshing again. A missing file means "not logged in" (exit `2` in M8-04 with the hint to run `bothy import login`). `String`, `GoString` and `%+v` of the session type print only `server_url` and `device_id`.

3. **Transport chain** (08 7.3, 22 4.3 item 5). Per request: take the access token (refresh first when the stored `access_expires_at` has passed; the tool knows no server key and trusts only the stored expiry and a `401`); set `Authorization: DPoP <token>` and a fresh proof (`jti` random 128 bits, `iat` now, `htu` from the stored `server_url` origin plus the request path, `ath` base64url SHA-256 of the token, `nonce` the last `DPoP-Nonce` seen); on `401` with `WWW-Authenticate: DPoP error="use_dpop_nonce"` take the new nonce and retry once; on a `401` whose `WWW-Authenticate` says `error="invalid_token"` refresh once and retry once; on `429` wait per requirement 16; never follow redirects. Request bodies are replayable (`GetBody`) for the retries. Writes that the contract makes idempotent by header carry an `Idempotency-Key` generated per logical call and kept across retries.

4. **Refresh** (08 7.4). `refreshToken` with the DPoP proof of the same key; on success store first, then return; a `401` with a family revocation (`refresh_reuse` or `invalid_grant` as the merged server names it) deletes nothing but reports "session revoked, run bothy import login" (exit `2` in callers). The tool never retries a refresh with an older token.

5. **`bothy import login` and `logout`** (22 10.1, 02 3.3). `login --server <url> [--name <device name>] [--session-file <path>] [--config <file>]`: refuse when a valid session file for the same server exists (exit `2`, hint `logout`); check the URL (requirement 15; the URL is an origin without path, like `server.public_url`); generate an Ed25519 key and a UUIDv7 device ID; `beginPairing` with `device{device_id, name (default "bothy import on <hostname>" cut to the device name limit), platform: cli, app_version: <build version>, public_jwk, alg: EdDSA}` and no `step_up_key`; print the `pairing_code` alone on standard output and the instruction on standard error ("In the Bothy app or web console choose Add device and enter this code; it expires at <time>"); poll `completePairing` every `import.login_poll_interval` until `201` (store the session, print "logged in" on standard error, exit `0`), `410` (exit `1`, expired or rejected) or `expires_at` passes (exit `1`). Ctrl-C ends the poll with exit `1` and writes nothing. `logout`: take the lock of the session file, call `logout`, delete the session file (the lock file stays, so a process that holds or waits for the lock never ends up in a second critical section on a new inode), release the lock, exit `0`; a revoked session still deletes the file. Neither command writes any other file.

6. **Fake Bothy for auth** (`bothytest`). An `httptest` server on `127.0.0.1:0` that implements `beginPairing`, `completePairing` (`202` until the test approves, then `201`; `410` modes), `refreshToken` (rotation with family and reuse detection, a reuse revokes), `logout`, `getMe`, and one protected `GET` route; it verifies every DPoP proof (signature, `htm`, `htu`, `ath`, `iat`, `jti` replay, nonce with `use_dpop_nonce` on the first request and on rotation), counts methods per route, can answer `429` with `Retry-After`, and echoes tokens into error bodies for the leak test. M8-04 extends it with the import routes.

7. **Rulings for `docs/DECISIONS.md`** (`D-M8-10-<n>`): the separate generated client package (note 1); the session file format, location and modes (note 2, requirement 14); the lock and re-read rule (note 2); `http` only for loopback servers (requirement 15); the new key `import.login_poll_interval`; the default device name; `EdDSA` as the DPoP algorithm of the tool.

## Tests (write them first)

- `bothyapi/dpop_test.go`: proofs verified by the verifier of M1-33 (`kernel/jose` and the identity DPoP check through `identitytest`, test-only import): `htu` without query, `ath` matches, nonce present after the first `use_dpop_nonce`.
- `bothyapi/login_test.go` against `bothytest`: `TestLoginPairing` (code on standard output only, session file with mode `0600` after approval, `platform = cli`, no step-up key in the registration); `TestLoginExpired`, `TestLoginRejected`, `TestLoginInterrupted` (no file written); `TestLoginRefusesHTTPForNonLoopback`; `TestLogoutDeletesFile`.
- `bothyapi/session_test.go`: `TestSessionFileModes` (a file or directory readable by group or others is refused; skipped only on Windows with a reason); `TestAtomicStore` (a crash simulated between temp write and rename leaves the old file intact); `TestConcurrentRefreshRotatesOnce` (two clients in parallel on one session file: exactly one `refreshToken` at the fake, no reuse revocation).
- `bothyapi/transport_test.go`: `TestNonceRetryOnce`; `TestRefreshOnInvalidToken`; `TestRefreshStoredBeforeUse` (the fake asserts that the file already holds the new refresh token when the first request with the new access token arrives); `TestBothy429` (wait recorded, limit abort); `TestNoRedirectFollowed`.
- `bothyapi/secret_test.go`: with echoing fakes, no access token, refresh token, private key or file content in any log line at debug level, standard output, standard error or error text.
- `test/importtool/login_test.go` (component `testsupport`): against the real server of the harness (M1-71) with Robin as owner on an active device: `bothy import login` pairs, Robin approves with `approvePairing` and step-up from his test device, the tool reaches `getMe`; `createStepUpChallenge` from the tool's session answers `403` (ID-T29); `approvePairing` (an action with step-up of M1-38) from the tool's session answers `401 step_up_required` (08 6.4 item 3).

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/... ./test/importtool/... -race -count=1`
- `make codegen-check`, `make config-reference`, `make config-doc-check`, `make arch`, `make lint`, `make fmt-check`, `make semgrep`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] Login through pairing works against the fake and the real server; the device is `cli` without a step-up key and cannot provide step-up.
- [ ] The refresh rotation is stored before use, happens once under the lock, and a DPoP nonce retry works.
- [ ] A session file readable by others is refused; the file content never reaches output or log.
- [ ] The API client is generated from `openapi.yaml`.
- [ ] 22, the configuration reference, READMEs, CHANGELOG and rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `cmd/bothy/importtool/bothyapi/README.md` and the touched READMEs in English.
- Rulings of Design note 7 recorded in `docs/DECISIONS.md` section 10.
- `services/server/CHANGELOG.md` entry under `[Unreleased]`.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (for example the Windows lock and file rules when no Windows runner ran the tests).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
