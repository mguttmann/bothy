# M8-01: Discord read client with pure GET

Status: ready | Depends on: M1-22 | Milestone: M8

## Goal

After this ticket the import tool has a Go package that reads a Discord server through exactly the calls of 22 4.1 and nothing else: a client whose exported methods all send `GET`, with the bot token only in the `Authorization` header of requests to `import.discord.api_base`, a `User-Agent` in the bot format Discord requires, typed results for the structure, threads and messages the later tickets map, page loops for archived threads (newest first, deduplicated by ID, ending on a page without a new ID) and for messages (`after`, `before`, `around`), and the rate-limit guard of 22 4.3: it waits `X-RateLimit-Reset-After` when `X-RateLimit-Remaining` is `0`, waits `retry_after` (body) or `Retry-After` (header) on `429` and retries at most `import.discord.max_429_retries` times, aborts with a typed error when a requested wait is longer than `import.discord.max_rate_wait`, never retries `401` or `403`, and remembers a `403` per channel so that no further call of the run goes to that channel. The token never appears in an error text, a log line or a result. A fake Discord server for tests (fixtures, method counting, fault injection, token echo) comes with it and is reused by M8-02, M8-04, M8-05 and M8-07. Why: the import must never be able to write to Discord, must never get its address blocked by error storms (RK-76), and must never leak the bot token (22 14, R-IM-03, ADR 0043).

## Read first

- `docs/spec/22-import-migration.md`: 2 (process separation), 4.1 (endpoint table, archived-thread paging, base URL, `User-Agent`, token only to the API base), 4.2 (bot, permissions, intent, token sources), 4.3 (rate-limit guard, items 1 to 4; item 5 is the Bothy side, M8-10), 7.1 (snowflakes are digit strings), 14 (row "Discord token"), 16 (rows "Discord client without a write method", "No secret in output", "Rate limit"), 17 (R-IM-01, R-IM-03), 19 (the `import.discord.*` keys and `import.http_timeout`), the sources R1 to R7.
- `docs/adr/0043-discord-import-read-only-dry-run-origin-key.md` (rulings 1 and 2, confirmation).
- `docs/REUSE.md`: 4.1 (fake server with fault injection), 4.3 (secret leak test), L17 and L18.
- `docs/RISKS.md`: RK-76.
- `docs/spec/19-security.md` 8 (SEC-NET-1 to SEC-NET-5: the egress rules this client goes through).
- `docs/spec/02-architecture.md` 3.3 (row `bothy import`), 6 (code layout), 7.1 and 7.2 (`cmd` component).
- `docs/DECISIONS.md`: D-W-51 (the tool's configuration), D-W-100, D-W-120 and D-W-124 (the M8 briefs, the tool path under `cmd/bothy/importtool`, the step-up rule of the `cli` session).
- Briefs (merged names win, D-W-100): `M1-22-egress-client.md` (`egress.New`, `Options`, `HostPolicy`, `Deps.Resolver`, `Deps.RootCAs`, the Semgrep rule `bothy-go-egress-only`).
- Code on the base branch: `services/server/internal/platform/egress`, `services/server/cmd/bothy`, `services/server/architecture/modules.yaml`.

This ticket touches no API operation, no table, no action, no event and no configuration key.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Discord IDs in fixtures are invented digit strings; fixture guilds, channels and users have invented names.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. The client knows only the calls of 22 4.1; every exported method sends `GET`; no exported method, field or option can send another verb (22 4.1, R-IM-01, ADR 0043 ruling 1).
12. The token goes only to the host of `import.discord.api_base`, only in the header `Authorization: Bot <token>`; never to another host, never in a URL, never in an error, log line, result or `String()` output (22 4.1, 4.2, 14).
13. Rate-limit guard of 22 4.3 items 1 to 4, with the limits `import.discord.max_429_retries`, `import.discord.max_rate_wait` and the per-call time limit `import.http_timeout`.
14. Archived threads newest first, next page with `before` equal to the `archive_timestamp` of the last thread, deduplicated by ID, ending on a page without a new ID (22 4.1).
15. Every Discord ID is a string of digits; anything else is an error naming the field, never the value (22 7.1).
16. The only way to the network is `platform/egress` (19 8, rule `bothy-go-egress-only`): no environment proxy, `https` only, the host policy names exactly the host of `import.discord.api_base`.

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `cmd/bothy/importtool/discord/client.go`: `Config` (API base URL, user agent version, page size, `MaxRetries`, `MaxRateWait`, `HTTPTimeout`), `New(cfg Config, token Token, deps Deps) (*Client, error)` and the read methods of Design note 2.
- `cmd/bothy/importtool/discord/token.go`: `Token`, an opaque type whose `String`, `GoString`, `Format` and JSON and text marshalling print `[redacted]`; the only way to use the value is the unexported header setter (Design note 3).
- `cmd/bothy/importtool/discord/ratelimit.go`: the guard of Design note 4, with a `clock.Clock` and a sleeper that tests replace.
- `cmd/bothy/importtool/discord/types.go`: the typed objects of Design note 5.
- `cmd/bothy/importtool/discord/errors.go`: `ErrRateWaitTooLong`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrServer`, `*CallError` (Design note 6).
- `cmd/bothy/importtool/discord/doc.go` and `README.md` (purpose, the call list, the guard, the token rule, how to use the fake).
- `cmd/bothy/importtool/discord/discordtest/`: the fake Discord of Design note 7; only `_test.go` files import it (test like `TestNoTestPackageInBinary`, Design note 8).
- `cmd/bothy/importtool/discord/testdata/`: fixture guild JSON (Design note 7).
- Tests named under "Tests".

Change:

- `cmd/bothy` package documentation and `services/server/README.md` (section on the commands): one line naming the import tool packages under `cmd/bothy/importtool` (D-W-124).
- `services/server/CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M8-01-1` to `D-M8-01-4`.

## Out of scope

- CDN downloads of attachments (M8-02), the Bothy API client and the login (M8-10), the command `bothy import discord`, the configuration file, mapping, preflight and report (M8-04), the read mirror (M8-05).
- Any write call to Discord, the gateway (websocket) of Discord, the members endpoint and the pin endpoint (22 4.1 "Deliberately not used").
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-01-discord-read-client`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord or any other real service: every test runs against the fake of this ticket on `127.0.0.1` with port 0; no real bot token exists in the repository, the environment or a test (tokens in tests are invented strings).
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go `t.TempDir()`), removed at the end.
- Never read or print secrets.
- Time only through `kernel/clock`; tests use a fake clock and a recording sleeper, never real waits longer than a few milliseconds.
- Never lower a threshold, disable a gate, delete a test or add a `nosemgrep` for `bothy-go-egress-only`.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Where the tool lives** (D-W-124; Ruling for docs/DECISIONS.md). `bothy import` is a subcommand of the one binary `bothy` (D-L1, 02 3.3), so the tool code lives under `cmd/bothy/importtool/` (component `cmd`, 02 7.2); the plan's `cli/import/discord` is `cmd/bothy/importtool/discord`. The component `cmd` may use every package, so this ticket adds `TestDiscordClientImportsNoServerModule`: `go list -deps ./cmd/bothy/importtool/discord` contains no package under `internal/` except `internal/platform/egress` and `internal/kernel/...`. The tool talks to the server only through the public API (22 2, R-IM-01).

2. **Methods** (22 4.1), all `GET`, each with `ctx` first:
    - `Me(ctx) (User, error)`: `GET /users/@me`.
    - `Guild(ctx, guildID) (Guild, error)`: `GET /guilds/{id}`.
    - `GuildChannels(ctx, guildID) ([]Channel, error)`: `GET /guilds/{id}/channels`.
    - `ActiveThreads(ctx, guildID) ([]Channel, error)`: `GET /guilds/{id}/threads/active`.
    - `ArchivedPublicThreads(ctx, channelID, fn func([]Channel) error) error` and `ArchivedPrivateThreads(...)`: page loop of note 1 of 22 4.1 (`before` = `archive_timestamp` of the last thread of the page, `limit` = `import.discord.page_size`, dedupe by ID, stop on `has_more = false` or a page without a new ID). A `403` on the private list returns `ErrForbidden` with the hint that `MANAGE_THREADS` is missing and does not add the channel to the forbidden set of note 4 (the permission is optional, 22 4.1 and 4.2); the caller notes it and skips the private list.
    - `Channel(ctx, channelID) (Channel, error)`: `GET /channels/{id}`.
    - `MessagesAfter(ctx, channelID, afterID string, limit int)`, `MessagesBefore(...)`, `MessagesAround(ctx, channelID, aroundID string, limit int)`: `GET /channels/{id}/messages?after|before|around=...&limit=...`; results always in ascending ID order regardless of the order Discord returns (Discord returns newest first for `before`; the client sorts by snowflake so callers never see another order).
    No method takes a path, a verb or a URL from the caller; the paths are constants. Query values are encoded with `url.Values`.

3. **Token** (22 4.2, 14). `Token` holds the value unexported; `NewToken(s string) (Token, error)` rejects empty and white space. The client sets `Authorization: Bot <token>` in one unexported function that first checks that the request host equals the host of `import.discord.api_base` (else it refuses without sending, `ErrTokenHostMismatch`); a redirect is never followed (`egress.RedirectNever`), so the header cannot travel to another host. Error texts built by this package never quote headers, bodies or the full URL with query; they name method, path template (for example `GET /channels/{channel.id}/messages`) and status.

4. **Rate-limit guard** (22 4.3; Ruling for docs/DECISIONS.md). Per call: before sending, if the last response of the same route bucket (header `X-RateLimit-Bucket`, falling back to the path template) said `X-RateLimit-Remaining: 0`, sleep `X-RateLimit-Reset-After` seconds (fractional allowed). On `429`: read `retry_after` from the JSON body, else `Retry-After` from the header (seconds), and sleep that long, then retry, at most `import.discord.max_429_retries` retries; a missing or unparsable wait counts as `import.discord.max_rate_wait` (fail towards waiting, never towards hammering). Any single wait greater than `import.discord.max_rate_wait` ends the call at once with `ErrRateWaitTooLong` (no sleep). `401` returns `ErrUnauthorized` and `403` `ErrForbidden`, never retried; a `403` on any call except the private archived thread list adds the channel ID to the run's forbidden set, and every later call on that channel returns `ErrForbidden` without a request (22 4.3 item 4); the set lives as long as the `Client` value (one run; the mirror of M8-05 creates a new client per round). `5xx` and network errors are not retried here (the mirror's backoff is M8-05; an import run ends with exit `1`, 22 10.3); they return `ErrServer` or the wrapped transport error with the token stripped. A global `429` (`X-RateLimit-Global`) uses the same rules.

5. **Types** (22 5, 6; [R3], [R4]). Only the fields the later tickets need, with `json` tags and unknown fields ignored: `User{ID, Username, GlobalName, Bot}`; `Guild{ID, Name, Roles []Role}` with `Role{ID, Name}`; `Channel{ID, Type, GuildID, ParentID, Name, Position, Topic, LastMessageID, PermissionOverwrites (count only, a slice of empty structs is enough), AvailableTags []Tag, AppliedTags []string, ThreadMetadata *ThreadMetadata, OwnerID}`; `ThreadMetadata{Archived *bool, ArchiveTimestamp string, Locked bool}` (pointer so that "missing" and `false` differ, 22 5.1.1); `Tag{ID, Name, EmojiName}`; `Message{ID, ChannelID, Type, Author User, Mentions []User, WebhookID, Content, Timestamp, EditedTimestamp, Pinned, Attachments []Attachment, Embeds []Embed, StickerItems []Sticker, Poll *Poll, MessageReference *MessageRef, MessageSnapshots []Snapshot, Reactions []Reaction, Thread *Channel}`; `Attachment{ID, Filename, Size, URL, ContentType}`; `Reaction{Count, Emoji{ID, Name}}`. Every ID field is validated as digits on decode (`ErrInvalidID` with the JSON field name). `Attachment.URL` and every field that can carry a signed URL have a `String()`-safe accessor for logs that returns scheme, host and path only (used by M8-02).

6. **Errors.** `*CallError{Op string; Status int; Kind error}` with `Unwrap` to the sentinel; `Error()` is `discord: <op>: <status text>`. No error carries the response body. Tests assert `errors.Is` for every sentinel.

7. **Fake Discord** (`discordtest`; REUSE 4.1 and 4.3). `discordtest.New(t, fixture)` starts an `httptest` TLS server on `127.0.0.1:0` and returns the server, a base URL with the host name `discord.test` and the `egress` test dependencies (resolver mapping `discord.test` to the server address, root CAs of the test certificate) plus the test-only egress rule `discord.test;allow_private`. It serves the endpoints of note 2 from a fixture (`testdata/guild_basic.json`: one guild with two categories, a text channel, an announcement channel, a forum with tags, a voice channel, a channel without category, active and archived threads over three archive pages, a private archived list that answers `403`, messages of every type of 22 6.1 including replies, forwards, stickers, polls, embeds, attachments, pins and reactions, a webhook author and a bot author). Fault injection per route: status codes, `429` with body `retry_after`, with header `Retry-After` only, with both, with a wait above the limit, `X-RateLimit-Remaining: 0` with `Reset-After`, a `403` on one channel, and "echo": the token and a signed query string placed into the error body and into a header. `Requests()` returns the request log (method, path, query, whether `Authorization` was present, host). Every request with a method other than `GET` answers `405` and is logged, so tests can assert the set of methods.

8. **Static check** (22 16 row "Discord client without a write method"; Ruling for docs/DECISIONS.md). `TestClientHasOnlyGet` parses the package with `go/parser` and `go/types`: every `http.NewRequest` or `http.NewRequestWithContext` call in non-test files has the constant `http.MethodGet` as method argument, there is exactly one such call site, and no exported identifier of the package mentions `Post`, `Put`, `Patch` or `Delete` (case-insensitive) or takes a method string. In addition `TestFakeSeesOnlyGet` drives every exported method against the fake and asserts that the logged method set is exactly `{GET}`. `TestNoTestPackageInBinary`: `go list -deps ./cmd/bothy` does not contain `discordtest`.

9. **Rulings for `docs/DECISIONS.md`** (`D-M8-01-<n>`): the tool path and the import check of note 1; the guard details of note 4 (bucket key, missing wait counts as the maximum, no retry of `5xx` in the client); the static check of note 8; the ascending order of every message result (note 2).

## Tests (write them first)

- `discord/client_test.go`: `TestClientHasOnlyGet`, `TestFakeSeesOnlyGet`, `TestUserAgentFormat` (the `DiscordBot (<url>, <version>)` form of [R6]); `TestTokenOnlyToAPIBase` (a base URL whose host differs from the request host refuses before sending; the fake logs `Authorization` only on its own host); `TestArchivedThreadPaging` (three pages, a duplicated thread across pages appears once, a page without a new ID ends the loop, `before` equals the last `archive_timestamp`); `TestPrivateArchivedForbidden`; `TestMessagesAscending` for `after`, `before` and `around`; `TestInvalidSnowflake` (a fixture with a non-digit ID gives `ErrInvalidID` naming the field).
- `discord/ratelimit_test.go` (22 16 row "Rate limit"; plan acceptance): `TestRemainingZeroWaits` (the sleeper records `Reset-After`); `TestRetryAfterBody`, `TestRetryAfterHeader`; `TestRetryLimit` (`import.discord.max_429_retries` plus one `429`s end with the last error); `TestWaitAboveLimitAborts` (no sleep, `ErrRateWaitTooLong`); `Test401And403NeverRetried` (exactly one request each); `Test403EndsChannel` (a second call on the same channel sends nothing, another channel still works); `TestPrivateArchived403KeepsChannel` (after a `403` on the private archived list, `MessagesAfter` on the same channel still sends a request and succeeds); `TestMissingWaitCountsAsMax`.
- `discord/secret_test.go` (22 16 row "No secret in output"; REUSE 4.3): with echo faults on every route, capture all `slog` output at debug level and every `error.Error()`, `fmt.Sprintf("%+v")` of results and errors, and assert that neither the token, nor `Bot <token>`, nor its base64 form, nor the signed query appears.
- `discord/arch_test.go`: `TestDiscordClientImportsNoServerModule`, `TestNoTestPackageInBinary`.

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/importtool/... -race -count=1`
- `make arch`, `make lint`, `make fmt-check`, `make semgrep`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] The static check finds only `GET` and the fake sees only `GET`.
- [ ] `429` waits `retry_after`; a wait above `import.discord.max_rate_wait` aborts; `401` and `403` are never retried; a `403` ends the channel for the run.
- [ ] The token appears in no error text, log line or result, also with echoing fakes.
- [ ] The fake Discord and its fixture are reusable by later tickets and stay out of the binary.
- [ ] README, CHANGELOG and the rulings are written.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `cmd/bothy/importtool/discord/README.md` in English.
- Rulings of Design note 9 recorded in `docs/DECISIONS.md` section 10.
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
4. What is not proven (in particular: the client was never run against the real Discord; header and wait semantics follow the cited documentation, R-IM-03).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
