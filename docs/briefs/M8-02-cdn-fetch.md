# M8-02: CDN fetch with host allow list

Status: ready | Depends on: M8-01 | Milestone: M8

## Goal

After this ticket the import tool loads Discord attachments safely: `Fetcher.Fetch` downloads one attachment with `GET` only when the URL has the scheme `https` and a host from `import.discord.cdn_hosts`, without the bot token or any other credential, without following redirects, without an environment proxy, with a read limit of `import.attachment_max_bytes` plus one byte (a larger body or a larger `Content-Length` is rejected before it is kept) and the time limit `import.http_timeout`. When the CDN answers `403` or `404` (expired signature), the fetcher re-reads the message once through the read client of M8-01 (`MessagesAround`), takes the fresh URL of the same attachment ID and tries exactly once more. Every error names only scheme, host and path of the URL; query and fragment (the signature `ex`, `is`, `hm`) are masked, and the error body of the CDN is never read into an error or a log. The result streams to the caller (M8-04 uploads it), never to a file. Why: the server never loads external URLs (Baseline section 4, R-IM-01), so only the tool fetches; the fetch must not become an SSRF, a token leak or a disk write (22 6.5, 14, 16 row "CDN limits").

## Read first

- `docs/spec/22-import-migration.md`: 4.1 (row "Attachments"), 6.5 (items 1 to 6), 14 (rows "Discord token" and "Server-side loading"), 16 (rows "CDN limits" and "No secret in output"), 19 (`import.discord.cdn_hosts`, `import.attachment_max_bytes`, `import.http_timeout`), source R5.
- `docs/REUSE.md`: 5.2 (CDN fetch with a host list: the rule and the adaptation), L22, 4.1, 4.3.
- `docs/spec/19-security.md` 8 (SEC-NET-1 to SEC-NET-5).
- `docs/DECISIONS.md`: D-W-100, D-W-120 and D-W-124; the rows `D-M8-01-<n>` of M8-01.
- Briefs (merged names win, D-W-100): `M8-01-discord-read-client.md` (package layout, `Token`, `MessagesAround`, the fake Discord `discordtest`, the URL-safe accessor of attachment URLs), `M1-22-egress-client.md` (`egress.New`, `Options.Hosts`, `RedirectNever`, `MaxResponseBytes`, `ErrResponseTooLarge`).
- Code on the base branch: `services/server/cmd/bothy/importtool/discord`, `services/server/internal/platform/egress`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Test hosts are names under `.test` that the test resolver maps to `127.0.0.1`.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour updates `docs/spec/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`, `actions.yaml`) is not edited by this ticket. Everything this ticket adds is English.

Product requirements implemented here:

11. Only `GET`, only `https`, only hosts from `import.discord.cdn_hosts` (exact match after lowercasing, no wildcard, the default port unless the entry names one), no redirects, no environment proxy (22 6.5 item 2, REUSE 5.2).
12. No credential to the CDN: no `Authorization` header, no cookie jar; the request carries only `User-Agent` and `Accept` (22 4.1 last sentence, 14).
13. Size: an attachment whose declared `size` is above `import.attachment_max_bytes` is not requested at all (22 6.5 item 1); a response with a larger `Content-Length` is rejected before reading; the body is read through a limit of `import.attachment_max_bytes` plus one byte and rejected when that byte arrives (REUSE 5.2).
14. Expired signature: on `403` or `404` re-read the message once (`MessagesAround` with limit 1 of the message ID), take the attachment with the same ID, try once more; no further loop (22 6.5 item 3).
15. Errors show only scheme, host and path; query and fragment are masked; the CDN error body is never read into an error or output (22 6.5 item 4).
16. `429` of the CDN uses the wait rules of the read client (`retry_after` or `Retry-After`, the limits `import.discord.max_429_retries` and `import.discord.max_rate_wait`), never with the token.

## Scope

All paths under `services/server/` unless they start with `docs/`. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`.

Create:

- `cmd/bothy/importtool/discord/cdn.go`: `CDNConfig{Hosts []string; MaxBytes int64; Timeout time.Duration}`, `NewFetcher(cfg CDNConfig, reader MessageReader, deps Deps) (*Fetcher, error)`, `(*Fetcher).Fetch(ctx, ref AttachmentRef) (*Download, error)` with `AttachmentRef{ChannelID, MessageID, AttachmentID string; URL string; Size int64}` and `Download{Body io.ReadCloser; Size int64; ContentType string}`; `MessageReader` is the one-method interface `MessagesAround` that `*Client` satisfies (Design notes 1 to 4).
- `cmd/bothy/importtool/discord/cdnurl.go`: `SafeURL(raw string) string` (scheme, host and path only) used by every error and log of the package, and the URL check of Design note 1.
- `cmd/bothy/importtool/discord/discordtest/cdn.go`: a fake CDN (Design note 5).
- Tests named under "Tests".

Change:

- `cmd/bothy/importtool/discord/README.md` (the CDN section), `errors.go` (new sentinels `ErrCDNHostNotAllowed`, `ErrCDNScheme`, `ErrCDNRedirect`, `ErrTooLarge`, `ErrCDNGone`).
- `services/server/CHANGELOG.md` under `[Unreleased]`; `docs/DECISIONS.md` section 10 rows `D-M8-02-1` to `D-M8-02-3`.

## Out of scope

- Uploading the bytes to Bothy and the jump-link line for a failed attachment (M8-04 with `createImportFile` of M8-11).
- Images inside embeds (never loaded, 22 6.3).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M8-02-cdn-fetch`; one writer per worktree.
- Conventional Commits with the component as scope (for example `feat(import): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Never contact Discord, its CDN or any other real service: tests use the fakes of M8-01 and this ticket on `127.0.0.1` with port 0.
- No byte of a download touches the disk: no temp file, no cache (22 10.1, the dry run proof of M8-04 relies on it).
- Stop only processes you started, by PID. Temporary directories only via `t.TempDir()`, removed at the end.
- Never read or print secrets; signed URLs count as secrets.
- Time only through `kernel/clock`; tests use a fake clock.
- Never lower a threshold, disable a gate, delete a test or add a `nosemgrep` for `bothy-go-egress-only`.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M8 by D-W-124). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **URL check before any connection** (22 6.5 item 2; REUSE 5.2). Parse with `url.Parse`; reject a URL with user information, a scheme other than `https` (`ErrCDNScheme`), a host not in `CDNConfig.Hosts` after lowercasing and without a trailing dot (`ErrCDNHostNotAllowed`), or an explicit port other than the port of the matching allow-list entry (`443` when the entry names none; production entries from `import.discord.cdn_hosts` name none, while tests write the fake's port into the entry as `host:port`). Then the egress client of M1-22 with `Options{Purpose: "import_cdn", Hosts: <exactly these hosts>, Redirects: egress.RedirectNever, Timeout: cfg.Timeout, MaxResponseBytes: cfg.MaxBytes + 1}` does the address checks after resolution (no private or blocked addresses; tests add the test-only rule with `allow_private` for their `.test` host). One egress client per fetcher is enough: the host policy already restricts every request to the allow list (REUSE 5.2 asks for "one transport per allowed host"; Ruling for docs/DECISIONS.md: the per-request host policy of `platform/egress` gives the same guarantee, because no connection is shared across a host that the policy did not allow).

2. **Redirects.** Any `3xx` is `ErrCDNRedirect` with the safe URL of the request; the `Location` header is never read into an error.

3. **Size.** `ref.Size > MaxBytes` returns `ErrTooLarge` without a request. A response with `Content-Length > MaxBytes` is closed unread and returns `ErrTooLarge`. `Download.Body` wraps the response body with a reader that returns `ErrTooLarge` once `MaxBytes + 1` bytes were read (the caller's upload then fails and reports "too large", 22 6.5 item 6). The fetcher never buffers the whole body.

4. **Re-read once** (22 6.5 item 3; Ruling for docs/DECISIONS.md). On `403` or `404`: close the body unread; call `reader.MessagesAround(ctx, ref.ChannelID, ref.MessageID, 1)`; find the attachment with `ref.AttachmentID`; if the message or the attachment is gone, return `ErrCDNGone`; else repeat the URL check on the fresh URL and fetch once. A second `403` or `404` returns `ErrCDNGone`. A `429` follows requirement 16; `5xx` returns `*CallError` without retry (the caller counts the attachment as failed and writes the jump-link line).

5. **Fake CDN** (`discordtest.NewCDN`). TLS `httptest` on `127.0.0.1:0` under two host names (`cdn.discord.test`, `media.discord.test`) and one foreign host (`evil.test`) for the negative cases; files of a given size served with or without `Content-Length`; fault modes: `302` to the foreign host, `403` once then success with a new signature (the fake Discord answers the re-read with the fresh URL), `404` always, `429` with `retry_after`, oversize body behind a small `Content-Length` lie, an error body that echoes the request URL with its query. It logs method, host, path, query and whether `Authorization` or `Cookie` was present.

6. **Rulings for `docs/DECISIONS.md`** (`D-M8-02-<n>`): one egress client with a host policy instead of one transport per host (note 1); the re-read rule and `ErrCDNGone` (note 4); the port rule of note 1 (`443` unless the allow-list entry names a port).

## Tests (write them first)

- `discord/cdn_test.go` (22 16 row "CDN limits", plan acceptance): `TestCDNRejectsForeignHost`, `TestCDNRejectsHTTP`, `TestCDNRejectsUserInfoAndPort` (all without any request in the fake log); `TestCDNRejectsRedirect` (the foreign host never receives a request); `TestCDNSizeDeclared` (no request), `TestCDNContentLengthTooLarge`, `TestCDNBodyLargerThanHeader` (the lie case fails at `MaxBytes + 1`); `TestCDNNoCredential` (no `Authorization`, no `Cookie` in any CDN request, also on the retry after a re-read); `TestCDNExpiredSignatureRereadsOnce` (exactly one `MessagesAround` and exactly two CDN requests); `TestCDNGoneAfterSecondFailure`; `TestCDNRateLimitUsesLimits`; `TestCDNStreamsWithoutDisk` (`TMPDIR` set to an empty `t.TempDir()` is still empty after a fetch of a large file).
- `discord/cdn_secret_test.go`: every error text and every captured log line at debug level contains neither the query string nor any of `ex=`, `is=`, `hm=` values, nor the echoed error body; `SafeURL` table test (query, fragment, user info, IPv6 host, encoded path).

Commands that must pass, from the repository root:

- `cd services/server && go test ./cmd/bothy/importtool/... -race -count=1`
- `make arch`, `make lint`, `make fmt-check`, `make semgrep`, `make coverage`
- `make check`

## Acceptance criteria

- [ ] A foreign host, `http`, a redirect and an oversized response are rejected; the CDN never sees a token.
- [ ] An expired signature is re-read once and retried once.
- [ ] Errors and logs show only scheme, host and path.
- [ ] No download touches the disk.
- [ ] README, CHANGELOG and the rulings are written.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comments on every exported Go symbol; comments explain why.
- `cmd/bothy/importtool/discord/README.md` updated in English.
- Rulings of Design note 6 recorded in `docs/DECISIONS.md` section 10.
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
4. What is not proven (the real CDN was never contacted; signature expiry follows source R5).
5. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
