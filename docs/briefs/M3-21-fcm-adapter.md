# M3-21: FCM adapter with Fake-FCM

Status: ready | Depends on: M3-19 | Milestone: M3

## Goal

After this ticket the job `notify.push_send` of M3-19 can deliver hints to Android devices: `internal/notify/adapters/fcm` implements the provider port `PushProvider` of M3-19 for registrations with `provider = fcm`. The adapter sends one HTTP v1 request `POST https://fcm.googleapis.com/v1/projects/{project_id}/messages:send` per hint (13 5.2), only through the egress client of M1-22, as a pure data message: no `notification` block, no `collapse_key` (R-PN-08), `android.priority` `high` or `normal` for quiet hints, `android.ttl` from `push.ttl`, and a `data` map that holds only the string keys of 13 6.2 (IDs, numbers and fixed keys, never a name or a text), plus the sync form `k = "sync"` with priority `normal` of 13 8. It authenticates with an OAuth 2.0 access token of the service account that the secret provider of M1-09 returns for `push.fcm.service_account_ref`: an `RS256` assertion signed with the account's key goes to the token endpoint, and the access token is reused for half its lifetime, with at most one token request in flight however many sends run in parallel. Every answer of 13 7.2 maps to exactly one result class of M3-19 (`domain.SendResult`), so the job deactivates, retries, drops and alerts as the table says: `404 UNREGISTERED` and an `INVALID_ARGUMENT` caused by the token deactivate with `token_invalid`, `403 SENDER_ID_MISMATCH` deactivates with `sender_mismatch` and raises a configuration alert, `401 THIRD_PARTY_AUTH_ERROR` is a configuration alert, `429 QUOTA_EXCEEDED` waits at least one minute, and `503 UNAVAILABLE` and `500 INTERNAL` retry with the `Retry-After` of the answer honoured. A Fake-FCM (HTTP with TLS on loopback, serving both the token endpoint and the send endpoint, verifying the assertion and the bearer token and recording every request) proves all of it here and is the provider fake that later client tests (M5-05) read their payload fixture from. Device tokens, the assertion, the access token and the key never reach a log, an error text or telemetry.

## Read first

- `docs/spec/13-push-notifications.md`: 1 "Goals" (point 1: no content at Google; point 5: every push has an ID); 5.2 "Android: FCM data message" (all of it: endpoint, OAuth scope, data message only, priority, `android.ttl` in `"<seconds>s"` with the 28 day limit, no `collapse_key`, overflow); 5.4 "Later" (the port `PushProvider`); 6.2 "FCM" (the payload, the string values, the sync form `k = "sync"` with `badge` and `cleared`); 7.2 "FCM" (the error table); 7.3 "Common" (failure count, `push.max_age`, tokens never in logs); 8 "Badge and sync" (the badge in `data.badge` and the Android sync with priority `normal`); 9 "Delivery measurement" (t1 = the provider accepted with `200`); 10 "Privacy"; 11 "Push keys and white label" (the token endpoint "from the service account", a token only valid for the sender of its Firebase project); 14 "Tests" (rows "Payload allow-list" and "Fake FCM with every response from 7.2"); 15 R-PN-08, R-PN-10; 17 "Configuration values of this chapter" (`push.ttl`, `push.retry_base`, `push.retry_cap`, `push.fcm.project_id`, `push.fcm.service_account_ref`); 18 sources Q8 to Q12 and Q16.
- `docs/spec/02-architecture.md`: 2 "Context (C4 level 1)" (row "Push services": not trusted, only IDs and placeholders); 5 "Layers within a module" (adapters may use `platform` and `kernel`); 6 "Code layout" (test helpers next to the adapter); 12.2 queue `delivery` and 12.3 row `notify.push_send`; 13 "Configuration loading" (items 3 to 5: secret references, start checks, cross-checks); 14.2 "Deadlines" (row "outbound calls": `egress.timeout`); 14.4 "Failure of a dependency" (rows "push services" and "secret provider").
- `docs/spec/19-security.md`: 8 "SSRF ban and outgoing connections" (SEC-NET-1, SEC-NET-2: push services are allowed targets that the consumer names in `Options.Hosts`); SEC-TLS-4; SEC-LOG-4 (push tokens never in logs); SEC-SEC-3; the row "Push credentials (APNs key, FCM service account, WNS client)" of the secrets table (`secret_ref`); SEC-T02 (lint of HTTP clients).
- `docs/spec/16-operations.md`: 4.1 "Basic rules" (point 3: only `worker` needs egress to push services); 5.2 "Secret provider (ADR 0011)"; 12 "Status page" (`health_source` `push.fcm`, built by M7-09, reads the outcomes this ticket returns).
- `docs/operations/configuration.md`: section 2 (`secret_ref`, egress target), the rows `push.ttl`, `push.retry_base`, `push.retry_cap`, `push.fcm.project_id`, `push.fcm.service_account_ref`, `egress.allowed_hosts`, `egress.timeout`, `egress.max_response_bytes`, `secrets.dir`, `secrets.cache_ttl`; section 5 "Secrets" (row `fcm_service_account.json`).
- `docs/observability/signals.md`: 4.4 rows `bothy.push.sent` (the outcomes `accepted`, `rejected_token`, `rejected_other`, `throttled`, `provider_error`) and `bothy.push.send.duration` (both recorded by M3-19 around `Send`); 5.2 span `push.send`; 5.3 attributes (`bothy.push.provider` with the value `fcm`; no device, person or thread IDs on metrics); 7 "Logs". `docs/observability/alerting.md` `BothyPushAcceptanceBudgetBurn` and `docs/observability/slos.md` `bothy-push/provider-acceptance` (a send with `rejected_other` burns the budget; `rejected_token` does not).
- ADR 0031 (push without content; Confirmation: a test checks the serialized payload against an allow list of keys and value types), ADR 0067 (JOSE profile with the standard library; item 2 lists `RS256` for signing assertions; item 5: the signer stays with the caller), ADR 0011 (secret provider), ADR 0008 (hexagonal modules).
- `docs/DECISIONS.md`: D-R-PUSH, D-PN-06 (no FCM `collapse_key`), D-W-100, D-W-108 (M3-19 owns the provider port, M3-20 and M3-21 implement it; (6) M3-19 also builds the badge count and the badge sync of 13 8), section 10 "Rulings of the build milestones" (format of the rows `D-M3-21-<n>`).
- `docs/security/threat-model.md`: TB7, 5.12 "Push", T-PU-01.
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (row "Test doubles": Fake-FCM); 9 "M3 Platform services" ("Test strategy": Fake-FCM; "Red team": content in push payloads, T-PU-01); the rows M3-19 (the port), M3-20 (the APNs sibling, not in this ticket's closure), M3-21 (this ticket, the contract), M3-22 (consumer of this adapter); 12 row M6-13, 13 row M7-09 and the row M5-05 (later consumers: measure points, status page, the instrumented test with a Fake-FCM payload); 19 P-09, P-10, P-11.
- Dependencies: the header names M3-19; its closure brings what this brief uses: M3-17 (`internal/notify/domain`), M3-18, M2-01, M1-71, M1-68, M1-33 (`internal/kernel/jose`), M1-22 (`internal/platform/egress`), M1-14 (`platform/jobs`), M1-13, M1-12 (`pgtest`), M1-09 (`internal/platform/secrets`), M1-08, M1-07 (configuration schema), M1-05, M1-01 and the rest of their closure. Not in the closure and not needed: M1-41 (`RS256` verification in `kernel/jose`; this ticket brings its own signer, Design note 3), M3-20 (`egress.Options.HTTP2`; FCM speaks HTTP/1.1, Design note 1 d).
- Briefs (merged names win, D-W-100): `M3-19-notify-jobs.md` (Design note 11: the steps of `notify.push_send`; Design note 12: `PushProvider`, `Registration`, `Notification` with `Badge`, `HasBadge` and `Cleared`, the constant `KindSync`; Design note 16: the badge number and the badge sync; Design note 13: `domain.SendResult`, `ResultClass` with `accepted`, `disable`, `retry`, `throttled`, `config_error`, `program_error`, `ApplyResult`, `RetryDelay`; Design note 14: `internal/wire/configkeys_push.go`; Design note 15: logs and outcomes; the provider set in `internal/wire`; `fakepush.FakeProvider`), `M1-22-egress-client.md` (`New`, `Config`, `Options`, `Deps`, `HostPolicy`, `RedirectNever`, `ErrBlocked`, `ErrResponseTooLarge`, the Semgrep rules `bothy-go-egress-only` and `bothy-go-tls-verify`), `M1-09-secret-provider.md` (`Ref`, `Value`, `Value.Bytes`, `Version`, `Provider.Resolve`, the `file` driver rules, `bothy config check --secrets`), `M1-33-identity-devices-dpop.md` (`jose.SignCompact`, `jose.Signer`, `jose.Header`), `M1-07-config-schema-loader.md` (`config.Key` with `Min` and `Max`, `KindSecretRef` and the placeholder `<secrets.dir>` in defaults).
- Code on the base branch (all under `services/server/`): `internal/platform/egress` (`client.go`: `Options`, `Deps.RootCAs`, `Client.Do`, `Client.HTTPClient`; HTTP/1.1 with keep-alive; `README.md`), `internal/platform/secrets`, `internal/kernel/jose`, `internal/kernel/clock` (`Clock`, `Fake`), `internal/notify` (`app/ports.go` with `PushProvider`, `app/send.go`, `domain/result.go`, `adapters/fakepush`, `adapters/jobs`), `internal/wire` (`config.go`, `configchecks.go` with `configChecks`, `configchapters.go`, `configkeys_push.go`, `egress.go` with `EgressConfig`, `config_test.go` with `TestConfigSchemaBuilds`), `internal/platform/config` (`Key`, `Min`, `Max`, `KindSecretRef`: an empty default is allowed only for string and list keys), `.semgrep/`, `architecture/modules.yaml`.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory.
9. Limits, windows and thresholds only through named configuration keys listed in `docs/operations/configuration.md`; code comments and docs name the key, never the number.
10. Specification first: a change to behaviour, API, event, right or configuration updates `docs/spec/` or `docs/api/` in the same change. The core contract (`docs/spec/00-glossary.md`, `03-data-model.md`, `04-sync-protocol.md`, `05-actions.md`) is not edited by this ticket.

Product requirements implemented here:

11. Sending: `POST https://fcm.googleapis.com/v1/projects/{project_id}/messages:send` with `project_id` = `push.fcm.project_id` and an OAuth 2.0 access token of the service account with the scope `https://www.googleapis.com/auth/firebase.messaging` (13 5.2).
12. Every connection goes through the egress client (19 SEC-NET-1); the adapter names exactly `fcm.googleapis.com` and the token endpoint host as its allowed targets (SEC-NET-2, 13 11); TLS is always verified (SEC-TLS-4); no redirect is followed.
13. Data messages only: no `notification` block, no `collapse_key`, no `apns` or `webpush` block; the app replaces notifications by a tag equal to the thread ID (13 5.2, R-PN-08, D-PN-06).
14. `android.priority` `high`, `normal` for quiet hints and for the sync form; `android.ttl` = `push.ttl` as whole seconds with the suffix `s`, at most 28 days (13 5.2, 8).
15. Payload exactly as 13 6.2: `message` with `token`, `android` and `data`; `data` holds only `v`, `nid`, `k`, `i`, `t`, `m`, `badge`, `quiet` and, for the sync form, `k = "sync"`, `badge` and `cleared`; every value is a string that holds an ID, a number or a fixed key (13 6.2, ADR 0031).
16. Answers as the table of 13 7.2: `404 UNREGISTERED` deactivates with `token_invalid`; `400 INVALID_ARGUMENT` caused by an invalid token deactivates with `token_invalid`, any other `INVALID_ARGUMENT` is a program error with alert; `403 SENDER_ID_MISMATCH` deactivates with `sender_mismatch` and raises a configuration alert; `401 THIRD_PARTY_AUTH_ERROR` is a configuration alert; `429 QUOTA_EXCEEDED` backs off with a first wait of at least one minute; `503 UNAVAILABLE` honours `Retry-After`, then exponential backoff with jitter; `500 INTERNAL` backs off between `push.retry_base` and `push.retry_cap` (the backoff itself is M3-19's `RetryDelay`).
17. No content and no secret leaves the server or reaches telemetry: Google sees the token, the time, the size and the IDs; logs, spans, metrics and error texts never carry a device token, the assertion, the access token, the key, a name or a text (13 7.3, 13 10, SEC-LOG-4, SEC-SEC-3).
18. Push credentials are instance configuration of the operator: the Firebase project ID and the service account key as a reference to the secret provider (13 11, 19 secrets table row "Push credentials").

## Scope

All paths under `services/server/` unless they start with `docs/`. Where M3-19 or M1-22 already created a file named below, extend it. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0` (`make licenses`).

Create:

- `internal/notify/adapters/fcm/doc.go`: package comment (what the adapter does, the two endpoints, the OAuth flow, the payload rules, that it is built only when FCM is configured, a short wiring example).
- `internal/notify/adapters/fcm/config.go`: `Config`, `Config.Enabled`, `Config.Validate` (Design note 2).
- `internal/notify/adapters/fcm/account.go`: `ServiceAccount`, `ParseServiceAccount` and the unexported `RS256` signer that implements `jose.Signer` (Design note 3).
- `internal/notify/adapters/fcm/token.go`: the unexported access token cache and the pause (Design notes 3 and 4).
- `internal/notify/adapters/fcm/payload.go`: the message builder (Design note 5).
- `internal/notify/adapters/fcm/classify.go`: the answer table and the `Retry-After` parser (Design note 6).
- `internal/notify/adapters/fcm/adapter.go`: `Adapter`, `Deps`, `Endpoints`, `SecretResolver`, `New`, `Adapter.Name`, `Adapter.Send` and a compile-time assertion that `*Adapter` implements the port `PushProvider` of M3-19 (Design notes 1, 4, 7).
- `internal/notify/adapters/fcm/fcmtest/server.go`: Fake-FCM `Server`, `Options`, `Response`, `TokenResponse`, `Request`, `Start` (Design note 8).
- Tests (design of each in "Tests"): `config_test.go`, `account_test.go`, `token_test.go`, `payload_test.go`, `classify_test.go`, `adapter_test.go`, `logs_test.go`, `fcmtest/server_test.go`, and `testdata/payload_message.golden`.
- `internal/wire/push_fcm.go`: builds the adapter from the configuration and adds it to the provider set of M3-19 in the roles that run `notify.push_send` (Design note 9), with `push_fcm_test.go`.

Change:

- `internal/wire/configkeys_push.go` of M3-19: the keys of Design note 2 if absent, and the bounds of `push.ttl`; `internal/wire/configchecks.go`: the cross-check `push_fcm_project_id`; `internal/wire/config_test.go`: `TestConfigSchemaBuilds` lists the keys.
- `cmd/bothy`: `bothy config check --secrets` resolves `push.fcm.service_account_ref` only when FCM is enabled, if the merged command resolves every `*_ref` key unconditionally (Design note 2).
- Docs (Design note 11; all new text in English, like the existing text): `docs/spec/13-push-notifications.md` 5.2, 6.2, 7.2, 11, 17 and 18; `docs/spec/02-architecture.md` 13 item 4 (the new cross-check); `docs/operations/configuration.md` regenerated (`make config-reference`).
- `internal/notify/README.md`: an English section on the adapter `fcm` (configuration, endpoints, token cache, pause, answer table, payload allow list, Fake-FCM); keep the line `May use the ports of these modules: ...` and the section `## Tables` exactly as they are, because M1-06 parses them. `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-21-1` to `D-M3-21-8` (Design note 12).

This ticket adds no migration, no operation, no action, no event, no problem code, no metric, no job kind, no new configuration key and no top-level Makefile target. It registers the existing keys of 13 17 that it reads, the bounds of `push.ttl` and one cross-check.

## Out of scope

- The hint pipeline: the consumer `push`, the job `notify.push_send` with its steps, the rights check before sending, `push.max_age`, the semaphore of `push.max_concurrent_requests`, `domain.ApplyResult`, `domain.RetryDelay` with `push.retry_base`, `push.retry_cap` and the jitter, the metrics `bothy.push.sent`, `bothy.push.send.duration`, `bothy.push.skipped`, `bothy.push.registrations.active` and the span `push.send`: M3-19. This ticket returns a `domain.SendResult` and lets the job do the rest.
- The badge number, the list of cleared threads and when a badge sync is due (13 8): M3-19 (D-W-108 (6)). This ticket writes `Badge` of every `Notification` and the sync form `KindSync` with `Cleared` (Design note 5 a, d).
- APNs (`internal/notify/adapters/apns`, Fake-APNs, `egress.Options.HTTP2`): M3-20. `unifiedpush`, `wns` and a push relay: later (13 5.3, 5.4, 11, R-PN-10).
- `registerPush`, `unregisterPush`, the preview token, the preview endpoint, the deactivation on logout, device revocation and user disable, the canary scenario with a push token (SEC-T10): M3-22.
- The Android push service that shows the placeholder and fetches the preview, and its instrumented test: M5-05 (it may read `testdata/payload_message.golden` of this ticket). The status page and its `health_source` `push.fcm`: M7-09. Measure points of `notify` for the own statistics: M6-13.
- New alert rules: the configuration alert of 13 7.2 is the Error log record plus, where the class is `config_error` or `program_error`, the outcome `rejected_other` that `BothyPushAcceptanceBudgetBurn` already watches (Design note 7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-21-fcm-adapter`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(notify): ...`, `feat(wire): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; Fake-FCM is an in-process TLS server on loopback; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- No real Google service in any test, no Firebase project, no real service account: test keys are RSA keys generated per run with `crypto/rsa`, and the service account JSON is written to a file with mode `0600` in a `t.TempDir()` for the `file` driver of the secret provider; the project ID is `example-chat`, the account e-mail `push@example-chat.example.com`, the key ID contains `EXAMPLE`. Nothing in this ticket costs money.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go tests `t.TempDir()`), removed at the end.
- Never read or print secrets. Never log, print or persist a device token, an assertion, an access token or key material; no test turns off certificate verification (the Semgrep rule `bothy-go-tls-verify` applies to tests too): clients trust the certificate of Fake-FCM through `egress.Deps.RootCAs`. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- Time only through `kernel/clock`; tests move a fake clock, they never sleep for an interval.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). This ticket touches no core table, event or action; the points it settles (Design note 11) are in chapter 13.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-19 (and M3-17, M3-18): follow its names for `PushProvider`, `Registration`, `Notification`, `domain.SendResult`, the result classes, the provider set and the key file, and list each difference.

## Design notes

1. **Place, port and connection** (02 5, 6; 13 5.2, 11; 19 SEC-NET-1, SEC-NET-2; Ruling for docs/DECISIONS.md, `D-M3-21-1`).

    a. The adapter lives in `internal/notify/adapters/fcm` and implements the outgoing port `PushProvider` of M3-19 (`Name() string` returns `fcm`; `Send(ctx, reg Registration, n Notification) (domain.SendResult, error)`). It imports the own module's `app` and `domain`, `kernel` (`clock`, `ids`, `jose`, `errs`), `platform/egress` and `platform/secrets`, and the standard library only: no Firebase SDK, no `golang.org/x/oauth2`, because both would open their own HTTP clients past the egress client. `architecture/modules.yaml` does not change.

    b. Public surface (names may follow the merged code, D-W-100; the fields are binding):

    ```go
    // SecretResolver resolves a secret reference; *secrets.Provider implements it (M1-09).
    type SecretResolver interface {
        Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
    }
    // Endpoints are base URLs for tests only; the zero value means the Google endpoints.
    type Endpoints struct {
        Send  string // replaces https://fcm.googleapis.com
        Token string // replaces https://oauth2.googleapis.com/token
    }
    type Deps struct {
        Egress     egress.Config // egress.* through wire.EgressConfig
        EgressDeps egress.Deps   // resolver, root CAs, logger, metrics
        Secrets    SecretResolver
        Clock      clock.Clock
        Logger     *slog.Logger
        Endpoints  Endpoints // tests only; zero in production
    }
    func New(ctx context.Context, cfg Config, deps Deps) (*Adapter, error)
    func (a *Adapter) Name() string
    func (a *Adapter) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error)
    ```

    c. One egress client per adapter, built in `New` with `egress.Options{Purpose: "push_fcm", Hosts: googleHosts, Redirects: egress.RedirectNever}`; `googleHosts` allows exactly scheme `https`, port `443` and the hosts `fcm.googleapis.com` and `oauth2.googleapis.com` (an operator needs no `egress.allowed_hosts` entry, 19 SEC-NET-2). Time and size limits are `egress.timeout` and `egress.max_response_bytes` (02 14.2; chapter 13 names no own deadline). Send and token requests share the client and its kept-alive connections.

    d. HTTP/1.1 with keep-alive, no HTTP/2: the FCM HTTP v1 API accepts HTTP/1.1 [Q8], chapter 13 demands HTTP/2 only for APNs (5.1, 12 point 5), and the option `egress.Options.HTTP2` of M3-20 is outside this ticket's closure. Do not set it even when it is merged, so the behaviour does not depend on the merge order; a later ticket may switch it on with its own test.

    e. The token endpoint is fixed: 13 11 names "the token endpoint from the service account", and the service account JSON names it in `token_uri`. The adapter never takes a host from a secret file: it posts to `https://oauth2.googleapis.com/token` (or `Endpoints.Token` in tests) and refuses at start a service account whose `token_uri` is present and differs from that URL (Design note 3 a). Reason: a URL from a file would make the secret a second egress configuration and an SSRF path past SEC-NET-2.

    f. `Deps.Endpoints` exists only so that tests reach Fake-FCM: production code in `internal/wire` never sets it. A test that sets it still runs every egress check: the Fake listens on loopback, so the test configuration carries the egress rule `127.0.0.1:<port>;allow_private` (`fcmtest.Server.EgressRule`) and trusts the Fake's certificate through `egress.Deps.RootCAs`.

2. **Configuration and start** (13 11, 17; 02 13 items 3 to 5; 02 14.4 row "secret provider"; Ruling for docs/DECISIONS.md, `D-M3-21-2`).

    a. Keys in `internal/wire/configkeys_push.go` (created by M3-19), each only if absent (M3-20 registers `push.ttl` too), with the chapter constant of 13 and the defaults of 13 17: `push.ttl` (duration, `24h`, `Min` `1s`, `Max` `672h`), `push.fcm.project_id` (string, empty), `push.fcm.service_account_ref` (secret reference). The `Max` of `push.ttl` is the FCM limit of 28 days (13 5.2 [Q11]); if `push.ttl` is already registered without it, add the bound. The `Min` makes `android.ttl` at least one whole second. Both bounds hold in every role, so a larger or smaller value stops the start, naming the key and never the value.

    b. The default of `push.fcm.service_account_ref` becomes `file:<secrets.dir>/fcm_service_account.json`: 13 17 says "empty", but the merged schema allows an empty default only for string and list keys, and `docs/operations/configuration.md` section 5 already names `fcm_service_account.json` as the secret behind this key. Change the row in 13 17 in the same change and regenerate the reference.

    c. `Config{ProjectID, ServiceAccountRef string; TTL, PauseFor time.Duration}`, with `PauseFor` = `push.retry_cap` (Design note 4 c). `Enabled()` is true exactly when `ProjectID` is not empty. The cross-check `push_fcm_project_id` in `configChecks` refuses a non-empty `push.fcm.project_id` that is not only lower-case letters, digits and hyphens starting with a letter, because the value becomes a path segment of the send URL; its message names the key. Add it to 02 13 item 4 ("Cross conditions today").

    d. Not enabled: `internal/wire` builds no adapter and reads no secret; jobs for `fcm` registrations end with the skip reason `no_provider` of M3-19. Enabled: in the roles `worker` and `all` (the roles that run `notify.push_send`), `New` resolves `push.fcm.service_account_ref` and parses it (Design note 3); an error stops the start and names `push.fcm.service_account_ref`, never a value (fail fast). `New` sends no request: the first token request happens at the first send, so a start without network works. The role `api` builds no adapter. Where the merged `bothy config check --secrets` resolves every `*_ref` key, it resolves this one only when FCM is enabled, because the default reference points at a file that an instance without FCM does not have.

3. **Service account and assertion** (13 5.2, 11; ADR 0067; Ruling for docs/DECISIONS.md, `D-M3-21-3`).

    a. `ParseServiceAccount(v secrets.Value, tokenURL string) (ServiceAccount, error)` reads the JSON object with `type`, `client_email`, `private_key_id`, `private_key` and, if present, `token_uri` (other fields are ignored). It requires `type = service_account`, a non-empty `client_email` and `private_key_id`, a `token_uri` that is absent or equal to `tokenURL`, and a `private_key` of exactly one PEM block of type `PRIVATE KEY` in PKCS#8 holding an RSA key (`x509.ParsePKCS8PrivateKey`). Every error text names the field and the reason and never contains a byte of the value. `ServiceAccount` keeps the key unexported and has no `String` or JSON form that could print it.

    b. The unexported signer implements `jose.Signer` of M1-33: `Alg()` returns `RS256`; `Sign` hashes the signing input with SHA-256 and returns `rsa.SignPKCS1v15` (RFC 7518 3.3). Signing an assertion with `RS256` is in the profile of ADR 0067 item 2, and item 5 keeps the private key with the caller; M1-41 (`RS256` verification) is not in the closure and not needed. If the merged `kernel/jose` already exports an `RS256` signer, use it and drop the own one.

    c. One assertion is `jose.SignCompact(signer, header, payload)` with a header of exactly `alg` (`RS256`), `typ` (`JWT`) and `kid` (`private_key_id`), and a payload of exactly `iss` (`client_email`), `scope` (`https://www.googleapis.com/auth/firebase.messaging`), `aud` (the token URL), `iat` (the clock's Unix seconds) and `exp` (`iat` plus one hour, the longest lifetime Google accepts for an assertion [Q17], a named constant whose comment cites the source: a provider fact, not an operator value). If the merged `jose.Header` cannot carry `typ` and `kid` or cannot leave out a further field, add an option to `kernel/jose` (additive, with a test) rather than send a header this note does not name.

    d. The token request is `POST` to the token URL with `content-type: application/x-www-form-urlencoded` and exactly the fields `grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer` and `assertion=<assertion>` (RFC 7523 2.1). A `200` answer must carry `access_token` (non-empty), `token_type` equal to `Bearer` ignoring case and a positive `expires_in`; otherwise the answer is a Go error (transient). Every token request resolves `push.fcm.service_account_ref` again through `Deps.Secrets` (the provider caches for `secrets.cache_ttl`) and parses it, so a rotated key is used from the next token on without a restart (16 5.2).

4. **Access token cache and pause** (13 5.2, 7.2, 11; Ruling for docs/DECISIONS.md, `D-M3-21-4` and `D-M3-21-5`).

    a. State of the adapter: the current access token, the time it was obtained, its `expires_in`, and `pausedUntil`. One mutex guards it, and the token request happens under that lock, so any number of parallel sends wait for one request. A token is reused while `now < obtained + expires_in / 2`; otherwise the first send that needs one requests it. Reason: half the lifetime leaves a wide margin for clock skew and request time without a new configuration key, and the ratio derives from the provider's own answer. A failed request stores nothing; the next send tries again.

    b. `401` from the send endpoint without the FCM error code `THIRD_PARTY_AUTH_ERROR` (status `UNAUTHENTICATED`: the access token expired or was revoked): if the request used the current token, drop it, request a new one and send the same message once more with it; a parallel send that got `401` for the same token reuses the replacement and also retries once. A second `401` for the replacement is a configuration error (c).

    c. Configuration error (Design note 6: the rows marked "pause", and a token request that the token endpoint refuses with a `4xx` answer, or a failed resolve or parse of the secret at that time): set `pausedUntil = now + push.retry_cap` and drop the token; write one Error record (Design note 7); return `config_error`. While `now < pausedUntil`, `Send` makes no request and returns `config_error` without a log record (the job drops the hint and counts `rejected_other`). The first send at or after `pausedUntil` requests a new token, reading the key again, and so probes once: an answer other than a configuration error ends the pause with one Info record; a configuration error starts the next pause. Reason: 13 7.2 asks for an alert and no repetition, but without a pause every hint would ask the token endpoint again with a revoked key; `push.retry_cap` is the longest wait the push path already names, so a rotated key or a fixed project heals the instance within it without a manual step, and hints dropped meanwhile are carried by the unread counts (13 7.3). The pause lives in the process; each worker probes for itself. Answers that concern one registration (`SENDER_ID_MISMATCH`, `THIRD_PARTY_AUTH_ERROR`) never pause the adapter.

    d. Token endpoint answers `429`, `5xx` or no answer: a Go error (transient, M3-19 counts it and retries); no pause, because the credentials are not shown to be wrong.

5. **Payload** (13 5.2, 6.2, 8; ADR 0031; R-PN-08; Ruling for docs/DECISIONS.md, `D-M3-21-6`).

    a. Request body of a hint, built from Go structs with JSON tags in the key order of 13 6.2 (the `data` map is a struct with `omitempty` string fields, not a Go map, so no other key can appear):

    | Key | Value |
    |---|---|
    | `message.token` | the registration's token |
    | `message.android.priority` | `high`; `normal` when `Quiet` |
    | `message.android.ttl` | `push.ttl` in whole seconds plus `s` (`86400s` for `24h`) |
    | `message.data.v` | `1` |
    | `message.data.nid` | the `Notification` ID |
    | `message.data.k` | the kind (`NotificationKind`) |
    | `message.data.i` | the instance ID |
    | `message.data.t` | the thread ID; absent when the hint has none (`security`, `admin`) |
    | `message.data.m` | the message ID; absent when the hint has none |
    | `message.data.badge` | `Badge` of the `Notification` in decimal (M3-19 Design note 12, 13 6.2, 8); always present |
    | `message.data.quiet` | `1` when `Quiet`, else `0` |

    UUIDs in canonical lower-case form; every `data` value is a JSON string (13 6.2 "Values in `data` are strings"). No `notification`, `apns`, `webpush`, `fcm_options`, `collapse_key`, `android.notification` or `android.collapse_key` key exists in the structs (13 5.2, R-PN-08). `Count` and `TargetID` of the `Notification` are not written: 13 6.2 has no key for them, and the app reads both through the preview and its own sync; this matches the APNs payload of 13 6.1.

    b. Headers: `authorization: Bearer <access token>`, `content-type: application/json`; method `POST`, path `/v1/projects/<push.fcm.project_id>/messages:send`.

    c. Checks before any request (no token is requested, the job sees the result at once): `reg.Provider` other than `fcm`, an unknown kind, a zero `nid` or instance ID, or `HasBadge` false give `program_error`; a token that is empty or holds a byte outside printable ASCII without space gives `disable` with `token_invalid` (it can never be valid and would otherwise break the request). `reg.Environment` and `reg.AppID` are not read: FCM has no sandbox, and a token of another app or project answers `SENDER_ID_MISMATCH` (13 11). The serialized `message.data` stays under the FCM limit of 4096 bytes for data messages [Q9]; a message above it gives `program_error` and is never truncated. The limit is one named constant whose comment cites 13 6.2: it is Google's fixed limit, not an operator value.

    d. The badge sync of 13 8 (`n.Kind == app.KindSync`, M3-19 Design notes 12 and 16): `data` is exactly `k = "sync"`, `badge` from `Badge` and, when `Cleared` is not empty, `cleared` (the IDs of `Cleared`, comma-separated, in the order given); `v`, `nid` and `i` are present as in a hint, so the app can match the instance; `t`, `m` and `quiet` are absent; `android.priority` is `normal`. If the IDs would push `data` over the limit of c, `cleared` is left out entirely and the app keeps those notifications until it opens, the same known limit 13 8 states for iOS; never a partial list, so a list is either complete or absent.

6. **Answer table** (13 7.2; Ruling for docs/DECISIONS.md, `D-M3-21-7`). `classify(status, body, header, now)` is a pure function. The body `{"error": {"code": ..., "status": "...", "details": [...]}}` is read within `egress.max_response_bytes`; the FCM error code is `errorCode` of the detail with `@type` `type.googleapis.com/google.firebase.fcm.v1.FcmError`, the token cause is a `fieldViolations` entry with `field` `message.token` in the detail with `@type` `type.googleapis.com/google.rpc.BadRequest` [Q16]. `error.message` is never read into a log or an error text, because Google may echo request values in it. A missing or broken body leaves the status and the `status` field to decide. The class names are those of M3-19 (`domain.ResultClass`).

    | Answer | Class | Fields | Log (Design note 7) |
    |---|---|---|---|
    | `200` | `accepted` | | |
    | `404 UNREGISTERED` | `disable` | `DisableReason` `token_invalid` | |
    | `400 INVALID_ARGUMENT` with a violation on `message.token` | `disable` | `token_invalid` | |
    | `400 INVALID_ARGUMENT` otherwise, any other `400` | `program_error` | | Error |
    | `403 SENDER_ID_MISMATCH` | `disable` | `sender_mismatch` | Error (configuration alert) |
    | `401 THIRD_PARTY_AUTH_ERROR` | `config_error`, no pause | | Error |
    | `401` otherwise (`UNAUTHENTICATED`) | renew and retry once (Design note 4 b); a second `401` is `config_error`, pause | | Error at the pause |
    | `403` otherwise (`PERMISSION_DENIED`) | `config_error`, pause | | Error |
    | `404` otherwise (`NOT_FOUND`: unknown project) | `config_error`, pause | | Error |
    | `429 QUOTA_EXCEEDED`, any other `429` | `throttled` | `RetryAfter` = `Retry-After`, at least one minute | |
    | `503 UNAVAILABLE`, `500 INTERNAL`, any other `5xx` | `retry` | `RetryAfter` = `Retry-After` when present, else zero (the job's backoff `domain.RetryDelay` of M3-19 applies: with its jitter in `[0.5, 1]` the first retry comes between half of `push.retry_base` and `push.retry_base`, every retry at most `push.retry_cap` later) | |
    | any other `4xx` | `program_error` | | Error |
    | any other status (`1xx`, `2xx` other than `200`, `3xx`) | `program_error` | | Error |
    | no answer (DNS, connect, TLS, timeout, egress refusal, oversize body), a token request without answer or with `429` or `5xx` | Go error | M3-19 counts it as transient | |

    `Retry-After` is read as delta seconds or as an HTTP date (RFC 9110 10.2.3) against the clock; a negative, unparsable or past value counts as absent. The minimum wait of one minute after `QUOTA_EXCEEDED` is Google's documented rule [Q16], one named constant whose comment cites 13 7.2. The unlisted rows follow the nearest listed answer of the same status, because Google documents the HTTP status as the class and the error code as detail [Q16]. `RetryDelay` of M3-19 keeps every retry at or after `RetryAfter`, so a `Retry-After` beyond `push.max_age` ends the hint as `expired`.

7. **Alert and logs** (13 7.2, 7.3, 9, 10; SEC-LOG-4; signals.md 5.3, 7; Ruling for docs/DECISIONS.md, `D-M3-21-8`).

    a. "alert" and "configuration alert" in 13 7.2 mean: one Error record (`fcm configuration error`, `fcm program error`, `fcm sender mismatch`) with `bothy.push.provider = fcm` and `error.type` = the FCM error code of the table when it is one, else the `status` field when it is one of `UNAUTHENTICATED`, `PERMISSION_DENIED`, `NOT_FOUND`, else the OAuth `error` of a refused token request when it is one of `invalid_grant`, `invalid_client`, `unauthorized_client`, `invalid_scope`, `invalid_request`, else `other`; plus the outcome `rejected_other` that M3-19 records for `config_error` and `program_error`, which the SLO `bothy-push/provider-acceptance` counts against its budget, so `BothyPushAcceptanceBudgetBurn` fires and the status page of M7-09 sees the error share of `push.fcm`. `SENDER_ID_MISMATCH` is class `disable` with outcome `rejected_token`, which does not burn the budget: the token belongs to another project's app, not to a broken server; its Error record is the configuration alert, one per answer. No new metric, attribute or alert rule.

    b. A pause logs once when it begins and once when a probe ends it, not per dropped hint.

    c. Never in a log record, span, metric or error text: the device token (also no hash prefix; the adapter needs none), the assertion, the access token, the `authorization` header, key material, the client e-mail, the project ID, `nid`, instance, thread, message or person IDs, `error.message` of Google, a name or a text. Errors of the egress client name method, scheme and host only (M1-22); the adapter wraps them with fixed text and never adds the URL, which holds the project ID.

8. **Fake-FCM** (`internal/notify/adapters/fcm/fcmtest`; PLAN 5 row "Test doubles", PLAN 9 "Test strategy"). A package without `testing` imports, so that later tests can use it as well:

    ```go
    type Options struct {
        ProjectID   string
        ClientEmail string
        KeyID       string
        PublicKey   *rsa.PublicKey // verifies the assertion
        Lifetime    time.Duration  // expires_in of issued tokens; 0 means one hour
    }
    type Response struct {
        Status     int    // 0 means 200
        ErrorCode  string // FcmError errorCode; empty: no FcmError detail
        RPCStatus  string // error.status, for example "UNAVAILABLE"
        TokenField bool   // adds a BadRequest violation on message.token
        RetryAfter string // Retry-After header verbatim; empty: absent
    }
    type TokenResponse struct {
        Status int    // 0 means 200 with a new token
        Error  string // OAuth error for a non-200 answer
    }
    type Request struct {
        Proto, Method, Path string
        Header              http.Header // a copy
        Body                []byte
        Bearer              string
    }
    func Start(opts Options) (*Server, error) // HTTP/1.1 with TLS on 127.0.0.1:0
    func (s *Server) URL() string              // base URL for Endpoints.Send
    func (s *Server) TokenURL() string         // URL() + "/token", for Endpoints.Token
    func (s *Server) RootCAs() *x509.CertPool
    func (s *Server) EgressRule() string       // "127.0.0.1:<port>;allow_private"
    func (s *Server) ServiceAccountJSON(key *rsa.PrivateKey) []byte
    func (s *Server) Script(token string, rs ...Response) // send answers in order, then 200
    func (s *Server) ScriptToken(rs ...TokenResponse)     // token answers in order, then 200
    func (s *Server) Requests() []Request                 // send requests
    func (s *Server) TokenRequests() int
    func (s *Server) RevokeTokens()                       // issued tokens answer 401 from now on
    func (s *Server) SetPublicKey(k *rsa.PublicKey)       // after a key rotation
    func (s *Server) Close()
    ```

    It is built on `httptest.NewUnstartedServer` and `StartTLS`. `ServiceAccountJSON` writes a service account with `type`, `client_email`, `private_key_id`, `private_key` (PKCS#8 PEM) and `token_uri` = `TokenURL()`. The token handler accepts only `POST` with the two form fields of Design note 3 d, verifies the assertion (header exactly `alg = RS256`, `typ = JWT`, `kid = KeyID`; payload exactly `iss = ClientEmail`, the FCM scope, `aud = TokenURL()`, `exp > iat`, `exp - iat` at most one hour; signature with `crypto/rsa` `VerifyPKCS1v15`, because `JWS.Verify` of M1-33 covers `ES256` and `EdDSA` only), answers the token script, and otherwise issues a random token with `expires_in` = `Lifetime` in seconds; a bad assertion gets `400` with OAuth `error` `invalid_grant`. The send handler answers `405` to anything but `POST`, `404` (`NOT_FOUND`, no FcmError) to a path other than `/v1/projects/<ProjectID>/messages:send`, `401` (`UNAUTHENTICATED`) to a bearer that it did not issue or revoked, `400 INVALID_ARGUMENT` to a body that is not a JSON object with a `message` object, records the request (with `r.Proto`), and then answers the script of the body's `message.token` with the FCM error JSON of Design note 6. It is safe for concurrent use. A test runs `go list -deps ./cmd/bothy` and fails if `fcmtest` appears (pattern of `identitytest`, M1-33).

9. **Wiring** (02 13 item 5; M3-19 Design note 12). `internal/wire/push_fcm.go` reads the keys of Design note 2 and `push.retry_cap` into `fcm.Config`, and when `Enabled()` it builds the adapter with `wire.EgressConfig`, the process's `egress.Deps` (resolver, logger, the egress metrics), the secret provider, `clock.System()` and the logger, and adds it to the provider set of M3-19 under `Name()` in the roles `worker` and `all`. `Deps.Endpoints` stays zero there. The instance ID that `Notification` carries is set by M3-19's wiring; the adapter does not read `instance`.

10. **Concurrency.** `Adapter` is safe for concurrent use; the job of M3-19 bounds parallel sends with `push.max_concurrent_requests`. `Send` does no I/O besides the egress requests (the send and, when due, one token request) and the secret resolve of a token request, holds no lock during the send request and honours the context deadline; a send that waits for the token lock gives up when its context ends.

11. **Specification** (rule 10; nothing in 00, 03, 04 or 05 changes; all new text in English, like the existing text):

    - `docs/spec/13-push-notifications.md` 5.2: a paragraph: the connection runs through the egress client over HTTP/1.1 with `fcm.googleapis.com` and `oauth2.googleapis.com` as the adapter's targets; the OAuth flow of Design note 3 (assertion header and claims, the JWT bearer grant, the fixed token URL and the `token_uri` check); the token cache with reuse for half of `expires_in`, one request in flight, the renewal after a `401` (Design note 4 a, b); the pause with one probe after `push.retry_cap` (Design note 4 c).
    - 13 6.2: a paragraph: the value table of Design note 5 a (absent keys without thread or message, `badge` always present, `Count` and `TargetID` not carried, no `collapse_key` and no `notification` block anywhere), the checks of 5 c (size checked before sending against the 4096 byte data limit [Q9], never truncated), the sync form of 5 d with `v`, `nid`, `i`, priority `normal` and the all-or-nothing rule for `cleared`.
    - 13 7.2: a paragraph after the table: the rows "otherwise" and "any other" of Design note 6, how the token cause of `INVALID_ARGUMENT` is recognised, the `401` renewal, the `Retry-After` forms, the retry delay without `Retry-After` (jitter in `[0.5, 1]`, so the first retry may come before `push.retry_base`; "backoff between `push.retry_base` and `push.retry_cap`" is read with that jitter, Design note 6), token endpoint failures, and what "alert" means (Design note 7 a).
    - 13 11: after "the token endpoint from the service account" the English rule: the token endpoint is `https://oauth2.googleapis.com/token`; a service account whose `token_uri` names another URL is refused at start.
    - 13 17: default of `push.fcm.service_account_ref` `file:<secrets.dir>/fcm_service_account.json` (Design note 2 b); the meaning of `push.ttl` gains "between 1 second and 28 days (FCM limit)".
    - 13 18: a new source Q17 "Google, Using OAuth 2.0 for Server to Server Applications: https://developers.google.com/identity/protocols/oauth2/service-account" for the assertion claims and the one hour limit; this build has no network, so mark the statements that cite it `[derived]` and say in the report that the source was not retrieved in this ticket.
    - `docs/spec/02-architecture.md` 13 item 4: the cross-check `push_fcm_project_id`.

12. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M3-21-1` egress over HTTP/1.1 with the two Google hosts as the adapter's policy, the fixed token URL, `Deps.Endpoints` for tests only (note 1); `D-M3-21-2` FCM enabled exactly with `push.fcm.project_id`, the cross-check, the default of `push.fcm.service_account_ref`, the bounds of `push.ttl`, the start check in `worker` and `all`, the conditional secret check (note 2); `D-M3-21-3` service account format, `RS256` signer, assertion header and claims, the key read at each token request (note 3); `D-M3-21-4` access token cache with reuse for half its lifetime and the renewal after `401` (note 4 a, b); `D-M3-21-5` pause for `push.retry_cap` with one probe, only for credential errors (note 4 c, d); `D-M3-21-6` payload rules, the pre-checks, the data size limit and the sync form with its all-or-nothing `cleared` (note 5); `D-M3-21-7` the answer table with its unlisted rows, the `Retry-After` rules and the retry delay bounds with jitter (note 6); `D-M3-21-8` alert and logs, `SENDER_ID_MISMATCH` as a logged configuration alert without budget burn (note 7). Rulings 3 to 8 also go into 13 (note 11).

## Tests (write them first)

Test world: Sam has an Android phone (`android`) with an `fcm` registration (`environment = production`, `app_id` `com.example.chat`); Robin has a second Android phone with an `fcm` registration; Alex has no Android device. Configuration in tests: `push.fcm.project_id = example-chat`, `push.fcm.service_account_ref` pointing at a `file` secret in `t.TempDir()` that holds `fcmtest.Server.ServiceAccountJSON` with an RSA key generated per run; durations are fixture values of the test configuration. Every adapter in tests talks to one Fake-FCM through `Deps.Endpoints` (`URL()` and `TokenURL()`), `egress.allowed_hosts` with the Fake's `EgressRule` and `egress.Deps.RootCAs` with its `RootCAs`; the clock is `clock.Fake`. Unless a test says otherwise, `Send` gets a `Notification` of kind `message` with thread, message, `nid`, instance ID and `Badge` 3 with `HasBadge` true.

Unit, `internal/notify/adapters/fcm` and `fcmtest`:

- `TestConfigEnabledAndValidate`: a project ID gives enabled; an empty one gives disabled; `Example-Chat`, `1chat`, `chat/x` and `chat project` fail the cross-check `push_fcm_project_id` with a message naming the key and not the value; `example-chat` passes.
- `TestParseServiceAccount`: the Fake's JSON parses; a missing or wrong `type`, an empty `client_email` or `private_key_id`, a `token_uri` of `https://token.example.com/token`, an EC key, a PKCS#1 `RSA PRIVATE KEY` block, two PEM blocks, broken JSON and an empty value fail; no error text contains a byte of the key, the e-mail or the URL.
- `TestAssertion_HeaderAndClaims` (13 5.2): the assertion that Fake-FCM receives has a header of exactly `alg = RS256`, `typ = JWT`, `kid` = the key ID and a payload of exactly `iss` = the e-mail, `scope` = the FCM scope, `aud` = the token URL, `iat` = the fake clock in Unix seconds and `exp` = `iat` plus one hour; the signature verifies with `rsa.VerifyPKCS1v15`; the form has exactly `grant_type` and `assertion`.
- `TestTokenCache_OnePerHalfLifetime`: 200 parallel sends (`-race`) give exactly one token request and every send request carries that token; the clock moved to one second before half the lifetime: still one; moved to half the lifetime: exactly two; 200 further parallel sends then: still two.
- `TestTokenCache_KeyRotation`: the secret file gets a new key and the Fake the new public key; after `secrets.cache_ttl` and the next renewal the next assertion is signed with the new key and accepted, without a new adapter.
- `TestTokenRenewOn401`: `RevokeTokens` before a send: two send requests and two token requests, the second send with the new token, result `accepted`; revoked again after the renewal within the same send: `config_error` and the adapter pauses; ten parallel sends that all get `401` for the same token create exactly one replacement and each retries once.
- `TestPayloadAllowList` (PLAN row "payload allow-list", 13 14 row "Payload allow-list", ADR 0031 Confirmation): for every kind of `NotificationKind` times `Quiet` times with and without message ID times a `Badge` of 0 and 3, and for the sync form with and without cleared threads: the request body parsed as generic JSON has only the top-level key `message`, `message` only `token`, `android` and `data`, `android` only `priority` and `ttl`, `data` only keys of 13 6.2 (`v`, `nid`, `k`, `i`, `t`, `m`, `badge`, `quiet`, and for the sync form `k`, `badge`, `cleared` beside `v`, `nid`, `i`); every `data` value is a JSON string; `badge` is present in every message and equals `Badge`; `nid`, `i`, `t` and `m` are canonical UUIDs, `v`, `badge` and `quiet` are decimal digits, `k` comes from `NotificationKind` or is `sync`, `cleared` is UUIDs joined by commas, `priority` is `high` or `normal`, `ttl` matches `^[0-9]+s$`; `security` and `admin` have no `t`; no `collapse_key` and no `notification` key appears at any depth; a `reflect` walk of the payload structs finds no string field outside these enums, IDs, numbers and the token, no map and no interface.
- `TestPayloadMatchesChapterExample`: the test reads the JSON block of 13 "6.2 FCM" from `docs/spec/13-push-notifications.md` (walking up to the directory that holds `docs/spec`), builds the message for the same kind, IDs and badge, not quiet, `push.ttl` `24h`, replaces `message.token` of the example by the test token, and compares both as JSON values; `testdata/payload_message.golden` holds the exact bytes and `-update` rewrites it.
- `TestPriorityAndTTL` (13 5.2, 8): not quiet gives `high`, quiet gives `normal` for every kind, the sync form gives `normal`; `quiet` is `1` exactly when quiet; `push.ttl` `24h` gives `86400s`, `90m` gives `5400s`.
- `TestDataSizeLimit`: every combination of `TestPayloadAllowList`, with the badge at the largest value its type allows, keeps `data` under the FCM limit; a hint whose `data` is made larger through an unexported test hook gives `program_error` and the Fake counts no request; a sync form with more cleared threads than fit is sent without `cleared` and with `badge`.
- `TestBadgeAndSyncForm` (13 6.2, 8; M3-19 Design note 12): a hint with `Badge` 7 gives `data.badge = "7"`; a `Notification` with `Kind` `app.KindSync`, `Badge` 2 and `Cleared` [T1, T2] gives `data` of exactly `v`, `nid`, `k = "sync"`, `i`, `badge = "2"` and `cleared` = T1 and T2 joined by a comma in that order, no `t`, `m` or `quiet`, and `android.priority` `normal`; the same with `Cleared` nil gives no `cleared` key.
- `TestHeadersAndPath`: method `POST`, path `/v1/projects/example-chat/messages:send`, `authorization` `Bearer` plus the issued token, `content-type` `application/json`, `Proto` `HTTP/1.1`.
- `TestPrechecksWithoutRequest` (Design note 5 c): provider `apns`, an unknown kind, a zero `nid` and `HasBadge` false give `program_error`; an empty token and a token with a space or a newline give `disable` with `token_invalid`; the Fake counts no send request and no token request; an `environment` of `sandbox` and an `app_id` of `com.example.other` are sent normally.
- `TestAnswerTable` (PLAN row "every response from 13 'FCM' triggers the expected consequence", 13 14 row "Fake FCM with every response from 7.2"): one subtest per row of Design note 6 with the Fake scripted for the token: the returned `SendResult` (class, `DisableReason`, `RetryAfter`), whether a Go error is returned (Fake closed; egress refusal; token endpoint `503`) and the log level; the unlisted rows with `400` without FcmError, `401 UNAUTHENTICATED` twice, `403 PERMISSION_DENIED`, `404 NOT_FOUND`, `409`, `502`, `204` and `301`.
- `TestRetryAfterHonoured` (PLAN row "`Retry-After` respected", 13 7.2): `503 UNAVAILABLE` with `Retry-After: 120` gives `RetryAfter` of two minutes; with an HTTP date 90 seconds after the fake clock gives 90 seconds; with a past date, `-5` or `soon` gives zero; without the header zero; `500 INTERNAL` with `Retry-After: 30` gives 30 seconds; `429 QUOTA_EXCEEDED` without the header gives one minute, with `Retry-After: 10` one minute, with `Retry-After: 300` five minutes; `retry` for `500` and `503`, `throttled` for `429`.
- `TestConfigErrorPauses`: `403 PERMISSION_DENIED` gives `config_error` and one Error record; the next 50 sends give `config_error` with no request at the Fake and no further log record; at `pausedUntil` (fake clock plus `push.retry_cap`) exactly one probe with a new token request goes out; `200` ends the pause with one Info record and the following sends go out. The same for a token request refused with OAuth `error` `invalid_grant` (scripted with `ScriptToken`), and for a secret file deleted before a token request (resolve error: the Error record names `push.fcm.service_account_ref`). `403 SENDER_ID_MISMATCH` and `401 THIRD_PARTY_AUTH_ERROR` for Sam's token do not pause: the next send for Robin's token goes out.
- `TestEgressOnly` (19 SEC-NET-1, SEC-NET-2): without the Fake's rule in `egress.allowed_hosts` a send returns an error with `errors.Is(err, egress.ErrBlocked)` and the Fake counts no request; the adapter's host policy allows `https` on port `443` for exactly `fcm.googleapis.com` and `oauth2.googleapis.com` and refuses `googleapis.com`, `www.googleapis.com`, port `8443` and `http`; an adapter built with zero `Deps.Endpoints` targets exactly `https://fcm.googleapis.com` and `https://oauth2.googleapis.com/token`, and a service account with another `token_uri` makes `New` fail.
- `TestNoSecretsInLogsOrErrors` (13 7.3, SEC-LOG-4, SEC-SEC-3): with a captured `slog` handler, run every row of `TestAnswerTable`, a pause, a probe, a renewal after `401`, a key rotation and a send against a closed Fake, with the Fake's `error.message` set to a canary string; neither the captured records nor any returned error text contain the device token, a prefix of it longer than four characters, the assertion, the access token, a line of the PEM key, the client e-mail, the project ID, `nid`, the instance, thread or message ID or the canary; Error records carry exactly `bothy.push.provider` and `error.type`.
- `fcmtest`: `TestFakeVerifiesAssertion` (an assertion of another key, a wrong `kid`, `iss`, `aud` or `scope`, or a lifetime above one hour gives `invalid_grant`; a valid one a token), `TestFakeRequiresPostPathAndBearer` (`405`, `404`, `401` for an unknown or revoked bearer), `TestFakeScriptOrder` (scripted answers in order, then `200`; token script likewise), `TestFakeNotInBinary` (`go list -deps ./cmd/bothy` does not list `fcmtest`).

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, the job handler of `notify.push_send` of M3-19 called directly after moving the fake clock; registrations are fixture rows written through the `pgtest` connection, because `registerPush` comes with M3-22):

- `TestConfigLoad_PushTTLBounds`: loading the configuration with `push.ttl` `673h` or `500ms` fails with a message naming the key and not the value; `1s`, `24h` and `672h` load. `TestConfigSchemaBuilds` lists every key of Design note 2.
- `TestFCMWiring`: FCM not enabled: the provider set has no `fcm`, no secret is read (a missing service account file is no error), and a job for Sam's phone ends with `no_provider`; enabled: the set has `fcm` in the roles `worker` and `all` and none in `api`, built with zero `Endpoints`, and no request leaves the process at start; enabled with a missing file, a broken JSON or an EC key: the start fails naming `push.fcm.service_account_ref`.
- `TestPushSendThroughFCM` (PLAN row "every response from 13 'FCM' triggers the expected consequence, `Retry-After` respected", end to end through the job): one `notify.push_send` for Sam's phone per answer, the registration row and the jobs checked after the handler: `200` sets `last_success_at` and `failure_count = 0`; `404 UNREGISTERED` sets `disabled_at` and `disabled_reason = 'token_invalid'`; `400 INVALID_ARGUMENT` with the token violation disables with `token_invalid`, without it leaves the row and inserts no retry; `403 SENDER_ID_MISMATCH` disables with `sender_mismatch`; `429 QUOTA_EXCEEDED` leaves `failure_count` and inserts the retry job no earlier than one minute later; `503 UNAVAILABLE` with `Retry-After: 120` raises `failure_count` by one and inserts the retry job no earlier than two minutes later; `500 INTERNAL` raises `failure_count` and inserts the retry job no earlier than half of `push.retry_base` and no later than `push.retry_cap` (the bounds of `domain.RetryDelay` with its jitter in `[0.5, 1]`); `401 THIRD_PARTY_AUTH_ERROR` and `403 PERMISSION_DENIED` leave the row, insert no retry and count `bothy.push.sent` with `rejected_other` (SDK manual reader); a revoked access token then `200` gives one hint delivered with two send requests.
- `TestHintReachesFakeFCM` (PLAN 9 "Red team": content in push payloads): Alex writes a message whose text contains a canary string in a thread Sam follows; the consumer `push` and the job deliver exactly one request to the Fake for Sam's phone with `data.k = message`, `data.t` = the thread, `data.m` = the message, `data.i` = the instance ID and `data.nid` = the job's `NotificationID` and `data.badge` = `BadgeCounter.For` of Sam at the send time; neither body nor headers contain the canary string, a display name, the space or channel name.
- `TestBadgeSyncReachesFakeFCM` (13 8, D-W-108 (6)): Sam has a counted unread thread T and reads it; the jobs `notify.badge_sync` and `notify.push_send` of M3-19 deliver exactly one request to the Fake for Sam's phone with `data.k = sync`, `data.badge` = the lowered number, `data.cleared` = T, no `t`, `m` or `quiet`, and `android.priority` `normal`.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/notify/... ./internal/kernel/jose/... ./internal/wire/... ./cmd/...`
- `make semgrep` (the rules `bothy-go-egress-only` and `bothy-go-tls-verify` stay green without a new `nosemgrep`), `make lint`, `make arch`
- `make config-reference` (no diff afterwards), `make config-doc-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `internal/notify/adapters/fcm` implements `PushProvider` of M3-19 for `provider = fcm` and sends HTTP v1 data messages to `fcm.googleapis.com` with an OAuth access token from `oauth2.googleapis.com`, only through the egress client.
- [ ] Every answer of 13 7.2, and every unlisted answer of Design note 6, gives the expected result class and, through the job of M3-19, the expected change of the registration, retry or drop; `Retry-After` is honoured in both forms and `QUOTA_EXCEEDED` waits at least one minute (`TestAnswerTable`, `TestRetryAfterHonoured`, `TestPushSendThroughFCM`).
- [ ] At most one token request is in flight, a token is reused for half its lifetime, a `401` renews it once, and a credential error pauses the adapter until one probe after `push.retry_cap` (`TestTokenCache_OnePerHalfLifetime`, `TestTokenRenewOn401`, `TestConfigErrorPauses`).
- [ ] The serialized message holds only the keys and string values of 13 6.2 for every kind and the sync form, has no `collapse_key` and no `notification` block, matches the example of 13 6.2 and keeps `data` under the FCM limit; priority and `android.ttl` follow 13 5.2 (`TestPayloadAllowList`, `TestPayloadMatchesChapterExample`, `TestPriorityAndTTL`, `TestDataSizeLimit`).
- [ ] Every hint carries `Badge` of the `Notification` as `data.badge`, and a `Notification` of `Kind` `KindSync` is sent as the sync form `k = "sync"` with `badge`, `cleared` from `Cleared` (complete or absent) and priority `normal` (`TestBadgeAndSyncForm`, `TestBadgeSyncReachesFakeFCM`).
- [ ] No device token, assertion, access token, key, ID or text appears in logs, spans or error texts; a message text never reaches Fake-FCM (`TestNoSecretsInLogsOrErrors`, `TestHintReachesFakeFCM`).
- [ ] Fake-FCM (`fcmtest`) verifies the assertion and the bearer token, serves both endpoints with TLS on loopback, answers by script and is not part of the server binary.
- [ ] 13 5.2, 6.2, 7.2, 11, 17 and 18, 02 13, `configuration.md`, the `notify` README, the CHANGELOG and the rows `D-M3-21-1` to `D-M3-21-8` are updated; `make semgrep`, `make arch` and `make config-doc-check` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `notify` updated in English (the adapter `fcm`: configuration, endpoints, token cache, pause, answer table, payload allow list, Fake-FCM), keeping the line `May use the ports of these modules: ...` and the section `## Tables` in their form for M1-06.
- Rulings of Design note 12 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 3 to 8 also in `docs/spec/13-push-notifications.md`.
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven (at least: behaviour against the real FCM service and the real token endpoint, which no test contacts; the shape of Google's error details, proven only against Fake-FCM; the source Q17, which was not retrieved in this ticket).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
