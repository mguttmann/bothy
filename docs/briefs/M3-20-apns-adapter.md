# M3-20: APNs adapter with Fake-APNs

Status: ready | Depends on: M3-19 | Milestone: M3

## Goal

After this ticket the job `notify.push_send` of M3-19 can deliver hints to iOS and macOS devices: `internal/notify/adapters/apns` implements the provider port `PushProvider` of M3-19 for registrations with `provider = apns`. The adapter speaks HTTP/2 over TLS to `api.push.apple.com` or `api.sandbox.push.apple.com` by `push_registrations.environment`, only through the egress client of M1-22, and authenticates with the token method of 13 5.1: an `ES256` JWT with `kid` = `push.apns.key_id` and the claims `iss` = `push.apns.team_id` and `iat`, signed with the `.p8` key that the secret provider of M1-09 returns for `push.apns.private_key_ref`. It creates at most one JWT per environment per `push.apns.jwt_refresh_interval`, however many sends run in parallel, and the server does not start when that key lies outside 20 to 60 minutes. The payload carries only the keys of 13 6.1 (IDs, numbers and fixed keys, never a name or a text) and stays under the Apple limit; the headers follow 13 5.1 (`apns-push-type`, `apns-id` = `nid`, `apns-topic` from `push.apns.topics`, `apns-priority` 10 or 5, `apns-expiration` from `push.ttl`, `apns-collapse-id` = thread ID). Every answer of 13 7.1 maps to exactly one result class of M3-19 (`domain.SendResult`), so the job deactivates, retries, drops and alarms as the table says: `410` deactivates with the `timestamp` rule, `403 ExpiredProviderToken` renews the JWT and retries once, a configuration error pauses that environment, `500` and `503` retry after `push.apns.retry_5xx_delay`. A Fake-APNs (HTTP/2 with TLS on loopback, verifying the JWT and recording every request) proves all of it here and is the provider fake of the push handoff measurement of M3-24. Device tokens, the JWT and the key never reach a log, an error text or telemetry.

## Read first

- `docs/spec/13-push-notifications.md`: 1 "Goals" (point 1: no content at Apple; point 5: every push has an ID); 5.1 "iOS and macOS: APNs with Notification Service Extension" (all of it: connection, token method, headers, NSE conditions `mutable-content` and visible `alert`); 5.4 "Later" (the port `PushProvider`); 6.1 "APNs" (the payload, the allow list of keys, `loc-key` per kind, `category`, `interruption-level`, `sound`, `badge`, `i`, the size limit); 7.1 "APNs" (the error table, the list of answers that must not be repeated); 7.3 "Common" (failure count, `push.max_age`, tokens never in logs); 8 "Badge and sync" (the badge in `aps.badge` and the iOS sync push with only `badge`); 9 "Delivery measurement" (t1 = the provider accepted with `200`); 10 "Privacy"; 11 "Push keys and white label"; 12 "Execution in the server" point 5 (HTTP/2 connections stay open and are shared); 14 "Tests" (rows "Payload allow-list", "Fake APNs (HTTP/2) with every response from 7.1", "JWT cache"); 15 R-PN-10; 17 "Configuration values of this chapter" (`push.ttl`, every `push.apns.*` key, `push.retry_base`, `push.retry_cap`).
- `docs/spec/02-architecture.md`: 2 "Context (C4 level 1)" (row "Push services": not trusted, only IDs and placeholders); 5 "Layers within a module" (adapters may use `platform` and `kernel`); 6 "Code layout" (test helpers in `internal/<module>/adapters/<module>test`, adapter names like `apns`); 12.2 queue `delivery` and 12.3 row `notify.push_send`; 13 "Configuration loading" (items 3 to 5: secret references, start checks, cross-checks); 14.2 "Deadlines" (row "outbound calls": `egress.timeout`); 14.4 "Failure of a dependency" (rows "push services" and "secret provider").
- `docs/spec/19-security.md`: 8 "SSRF ban and outgoing connections" (SEC-NET-1, SEC-NET-2: push services are allowed targets that the consumer names in `Options.Hosts`); SEC-LOG-4 (push tokens never in logs); the row "Push credentials" of the secrets table (`secret_ref`); SEC-T02 (lint of HTTP clients).
- `docs/spec/16-operations.md`: 4.1 "Basic rules" (point 3: only `worker` needs egress to push services); 5.2 "Secret provider (ADR 0011)"; 12 "Status page" (`health_source` `push.apns`, built by M7-09, reads the outcomes this ticket returns).
- `docs/operations/configuration.md`: section 2 (`secret_ref`, egress target), the rows `push.ttl`, `push.apns.team_id`, `push.apns.key_id`, `push.apns.private_key_ref`, `push.apns.topics`, `push.apns.jwt_refresh_interval`, `push.apns.retry_5xx_delay`, `push.apns.time_sensitive_enabled`, `egress.allowed_hosts`, `egress.timeout`, `egress.max_response_bytes`, `secrets.dir`, `secrets.cache_ttl`; section 5 "Secrets" (row `apns_key.p8`).
- `docs/observability/signals.md`: 4.4 rows `bothy.push.sent` (the outcomes `accepted`, `rejected_token`, `rejected_other`, `throttled`, `provider_error`) and `bothy.push.send.duration` (both recorded by M3-19 around `Send`); 5.2 span `push.send`; 5.3 attributes (`bothy.push.provider`; no device, person or thread IDs on metrics); 7 "Logs". `docs/observability/alerting.md` 3.1 `BothyPushAcceptanceBudgetBurn` and `docs/observability/slos.md` `bothy-push/provider-acceptance` (a send with `rejected_other` burns the budget; `rejected_token` does not).
- ADR 0031 (push without content, Confirmation: a test checks the serialized payload against an allow list of keys and value types), ADR 0067 (JOSE profile with the standard library), ADR 0011 (secret provider), ADR 0008 (hexagonal modules).
- `docs/DECISIONS.md`: D-K-X3 (`push.apns.private_key_ref` is the name), D-W-100, D-W-108 (M3-19 owns the provider port, M3-20 and M3-21 implement it; (6) M3-19 also builds the badge count and the badge sync of 13 8), section 10 "Rulings of the build milestones" (format of the rows `D-M3-20-<n>`).
- `docs/security/threat-model.md`: TB7, 5.12 "Push", T-PU-01. `docs/RISKS.md`: RK-39 (placeholders stay when the device cannot reach the server; nothing to build here).
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (row "Test doubles": Fake-APNs); 9 "M3 Platform services" ("Test strategy": Fake-APNs with HTTP/2 and TLS; "Red team": content in push payloads, T-PU-01, T-NW-01); the rows M3-19 (the port), M3-20 (this ticket, the contract), M3-21 (the FCM sibling), M3-22, M3-24 (consumers of this adapter and of Fake-APNs); 12 row M6-13 and 13 row M7-09 (later consumers); 19 P-09, P-10, P-11.
- Dependencies: the header names M3-19; its closure brings what this brief uses: M3-17 (`internal/notify/domain`), M3-18, M2-01, M1-71, M1-68, M1-33 (`internal/kernel/jose`), M1-22 (`internal/platform/egress`), M1-14 (`platform/jobs`), M1-13, M1-12 (`pgtest`), M1-09 (`internal/platform/secrets`), M1-08, M1-07 (configuration schema), M1-05, M1-01 and the rest of their closure.
- Briefs (merged names win, D-W-100): `M3-19-notify-jobs.md` (Design note 11: the steps of `notify.push_send`; Design note 12: `PushProvider`, `Registration`, `Notification` with `Badge`, `HasBadge` and `Cleared`, `KindSync`; Design note 16: `BadgeCounter.For` and the job `notify.badge_sync`; Design note 13: `domain.SendResult`, `ResultClass` with `accepted`, `disable`, `retry`, `throttled`, `config_error`, `program_error`, `ApplyResult`, `RetryDelay`; Design note 14: `internal/wire/configkeys_push.go`; Design note 15: logs and outcomes; the provider set in `internal/wire`; `fakepush.FakeProvider`), `M1-22-egress-client.md` (`New`, `Config`, `Options`, `Deps`, `HostPolicy`, `RedirectNever`, `ErrBlocked`, `ErrResponseTooLarge`, the Semgrep rules `bothy-go-egress-only` and `bothy-go-tls-verify`), `M1-09-secret-provider.md` (`Ref`, `Value`, `Version`, `Provider.Resolve`, the `file` driver rules, `bothy config check --secrets`), `M1-33-identity-devices-dpop.md` (`jose.SignCompact`, `jose.Signer`, `jose.ParseCompactJWS`, `JWS.Verify`, `ES256` signatures as 64 bytes `R || S`), `M1-07-config-schema-loader.md` (`config.Key` with `Min` and `Max`), `M3-02-object-storage-s3.md` (the pattern for an adapter that takes its credentials from the secret provider and its connections from the egress client: `SecretResolver`, start check, conditional resolve in `bothy config check --secrets`).
- Code on the base branch (all under `services/server/`): `internal/platform/egress` (`client.go`: HTTP/2 is off because `DialContext` and `TLSClientConfig` are set and `ForceAttemptHTTP2` is not; `README.md`), `internal/platform/secrets`, `internal/kernel/jose`, `internal/kernel/clock` (`Clock`, `Fake`), `internal/notify` (`app/ports.go` with `PushProvider`, `app/send.go`, `domain/result.go`, `adapters/fakepush`, `adapters/jobs`), `internal/wire` (`config.go` with `configKeyGroups`, `configchecks.go` with `configChecks`, `configchapters.go`, `configkeys_push.go`, `egress.go` with `EgressConfig`, `config_test.go` with `TestConfigSchemaBuilds`), `internal/platform/config` (`Key`, `Min`, `Max`, `KindSecretRef`: an empty default is allowed only for string and list keys), `.semgrep/`, `architecture/modules.yaml`.

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

11. Connection: HTTP/2 over TLS to `api.push.apple.com` for `environment = production` and `api.sandbox.push.apple.com` for `sandbox`, request `POST /3/device/<token>`; connections stay open and are shared between sends (13 5.1, 13 12 point 5).
12. Every connection goes through the egress client (19 SEC-NET-1); the adapter names exactly the two Apple hosts as its allowed targets (SEC-NET-2); TLS is always verified (SEC-TLS-4); no redirect is followed.
13. Token method: JWT with `ES256`, header `alg` and `kid`, claims `iss` (team ID) and `iat`, signed with the `.p8` key from the secret provider; renewed every `push.apns.jwt_refresh_interval`; the server does not start when that value lies outside 20 to 60 minutes (13 5.1, 13 14 row "JWT cache").
14. Headers: `apns-push-type: alert`; `apns-id` = `nid`; `apns-topic` = `push_registrations.app_id`, which must be in `push.apns.topics`; `apns-priority` 10, 5 for quiet hints; `apns-expiration` = now plus `push.ttl`; `apns-collapse-id` = thread ID (13 5.1).
15. Payload exactly as 13 6.1: `aps` with `alert` (`title-loc-key`, `loc-key`), `mutable-content: 1`, `thread-id`, `category`, `sound`, `badge`, `interruption-level`; own keys only in the object `b` next to `aps` (`v`, `nid`, `k`, `i`, `t`, `m`); `loc-key` and `category` per kind; `interruption-level` `active`, `passive` in quiet hours, `time-sensitive` only for `input_needed` with `push.apns.time_sensitive_enabled`, never `critical`; no `sound` for quiet hints; every value is an ID, a number or a fixed key; the serialized payload stays under the Apple limit of 4096 bytes (13 6.1, ADR 0031).
16. Answers as the table of 13 7.1: `200` success; `410 Unregistered` and `410 ExpiredToken` deactivate with `token_invalid` unless the token was registered again after the answer's `timestamp`; `400 BadDeviceToken` deactivates with `token_invalid`; `400 DeviceTokenNotForTopic` with `topic_mismatch`; `403 ExpiredProviderToken` renews the JWT and retries once; the other `403` reasons of the table are configuration errors (alarm, APNs sending paused); `429 TooManyProviderTokenUpdates` is a program error (alarm); `429 TooManyRequests` retries later on the same device; `413 PayloadTooLarge` and other `400` are program errors without retry; `500` and `503` retry after `push.apns.retry_5xx_delay`.
17. No content and no secret leaves the server or reaches telemetry: Apple sees the token, the topic, the time, the size and the IDs; logs, spans, metrics and error texts never carry a device token, the JWT, the key, a name or a text (13 7.3, 13 10, SEC-LOG-4, SEC-SEC-3).
18. Push credentials are instance configuration of the operator: team ID, key ID, the key as a reference to the secret provider, the allowed topics (13 11, 19 secrets table "Push credentials").

## Scope

All paths under `services/server/` unless they start with `docs/`. Where M3-19 or M1-22 already created a file named below, extend it. Every new Go file starts with `// SPDX-License-Identifier: Apache-2.0` (`make licenses`).

Create:

- `internal/notify/adapters/apns/doc.go`: package comment (what the adapter does, the two endpoints, the token method, the payload rules, that it is built only when APNs is configured, a short wiring example).
- `internal/notify/adapters/apns/config.go`: `Config`, `Config.Enabled`, `Config.Validate` (Design note 2).
- `internal/notify/adapters/apns/key.go`: `ParsePrivateKey` and the unexported `ES256` signer that implements `jose.Signer` (Design note 3).
- `internal/notify/adapters/apns/token.go`: the unexported JWT cache per environment (Design notes 3 and 4).
- `internal/notify/adapters/apns/payload.go`: the payload and header builders (Design note 5).
- `internal/notify/adapters/apns/classify.go`: the answer table (Design note 6).
- `internal/notify/adapters/apns/adapter.go`: `Adapter`, `Deps`, `SecretResolver`, `New`, `Adapter.Name`, `Adapter.Send` and a compile-time assertion that `*Adapter` implements the port `PushProvider` of M3-19 (Design notes 1, 4, 7).
- `internal/notify/adapters/apns/apnstest/server.go`: Fake-APNs `Server`, `Options`, `Response`, `Request`, `Start` (Design note 8).
- Tests (design of each in "Tests"): `config_test.go`, `key_test.go`, `token_test.go`, `payload_test.go`, `classify_test.go`, `adapter_test.go`, `logs_test.go`, `apnstest/server_test.go`, and `testdata/payload_message.golden`.
- `internal/wire/push_apns.go`: builds the adapter from the configuration and adds it to the provider set of M3-19 in the roles that run `notify.push_send` (Design note 9), with `push_apns_test.go`.

Change:

- `internal/platform/egress` (additive): `Options.HTTP2` with its test and the README line (Design note 1).
- `internal/wire/configkeys_push.go` of M3-19: the keys of Design note 2 if absent; `internal/wire/configchecks.go`: the cross-check `push_apns_complete`; `internal/wire/config_test.go`: `TestConfigSchemaBuilds` lists the keys.
- `cmd/bothy`: `bothy config check --secrets` resolves `push.apns.private_key_ref` only when APNs is enabled, if the merged command resolves every `*_ref` key unconditionally (Design note 2).
- Docs (Design note 11; all new text in English, like the existing text): `docs/spec/13-push-notifications.md` 5.1, 6.1, 7.1, 11 and 17; `docs/spec/02-architecture.md` 13 item 4 (the new cross-check); `docs/operations/configuration.md` regenerated (`make config-reference`); `internal/platform/egress/README.md`.
- `internal/notify/README.md`: an English section on the adapter `apns` (configuration, endpoints, JWT cache, pause, answer table, payload allow list, Fake-APNs for M3-19 to M3-24); keep the line `May use the ports of these modules: ...` and the section `## Tables` exactly as they are, because M1-06 parses them. `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-20-1` to `D-M3-20-8` (Design note 12).

This ticket adds no migration, no operation, no action, no event, no problem code, no metric, no job kind, no new configuration key and no top-level Makefile target. It registers the existing keys of 13 17 that it reads, one cross-check and one egress option.

## Out of scope

- The hint pipeline: the consumer `push`, the job `notify.push_send` with its steps, the rights check before sending, `push.max_age`, the semaphore of `push.max_concurrent_requests`, `domain.ApplyResult`, `domain.RetryDelay` with `push.retry_base` and `push.retry_cap`, the metrics `bothy.push.sent`, `bothy.push.send.duration`, `bothy.push.skipped`, `bothy.push.registrations.active` and the span `push.send`: M3-19. This ticket returns a `domain.SendResult` and lets the job do the rest.
- The badge number and when a badge sync is due (13 8): M3-19 (D-W-108 (6)). This ticket only writes the `Badge` it receives into `aps.badge` and sends a `Notification` with `Kind == KindSync` in the iOS sync form of Design note 5 d.
- FCM (`internal/notify/adapters/fcm`, Fake-FCM): M3-21. `unifiedpush`, `wns` and a push relay: later (13 5.3, 5.4, 11, R-PN-10).
- `registerPush`, `unregisterPush`, the check of `app_id` against `push.apns.topics` at registration, the preview token, the preview endpoint, the deactivation on logout, device revocation and user disable, the canary scenario with a push token (SEC-T10): M3-22.
- The measurement of `perf.push_handoff_p95` against Fake-APNs: M3-24 (this ticket provides `apnstest`). The status page and its `health_source` `push.apns` ("status page 'Push disrupted'" of 13 7.1): M7-09. Measure points of `notify` for the own statistics: M6-13.
- The Notification Service Extension and showing the placeholder: M5-04. Desktop hints over the gateway: M3-23.
- New alert rules: the alarm of 13 7.1 is the Error log record plus the outcome `rejected_other`, which `BothyPushAcceptanceBudgetBurn` already watches (Design note 7).
- Changes to the core contract.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M3-20-apns-adapter`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(notify): ...`, `feat(egress): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; Fake-APNs is an in-process TLS server on loopback; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- No real Apple service in any test, no Apple account, no real key: test keys are generated per run with `crypto/ecdsa` and written to a file with mode `0600` in a `t.TempDir()` for the `file` driver of the secret provider; team and key IDs in tests contain `EXAMPLE`; topics are `com.example.chat` and `com.example.chat.mac`. Nothing in this ticket costs money.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d` (in Go tests `t.TempDir()`), removed at the end.
- Never read or print secrets. Never log, print or persist a device token, a JWT or key material; no test turns off certificate verification (the Semgrep rule `bothy-go-tls-verify` applies to tests too): clients trust the certificate of Fake-APNs through `egress.Deps.RootCAs`. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- Time only through `kernel/clock`; tests move a fake clock, they never sleep for an interval.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11). This ticket touches no core table, event or action; the points it settles (Design note 11) are in chapter 13.
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same holds for the merged code of M3-19 (and M3-17, M3-18): follow its names for `PushProvider`, `Registration`, `Notification`, `domain.SendResult`, the result classes, the provider set and the key file, and list each difference.

## Design notes

1. **Place, port and connection** (02 5, 6; 13 5.1, 12 point 5; 19 SEC-NET-1, SEC-NET-2; Ruling for docs/DECISIONS.md, `D-M3-20-1`).

    a. The adapter lives in `internal/notify/adapters/apns` and implements the outgoing port `PushProvider` of M3-19 (`Name() string` returns `apns`; `Send(ctx, reg Registration, n Notification) (domain.SendResult, error)`). It imports the own module's `app` and `domain`, `kernel` (`clock`, `ids`, `jose`, `errs`), `platform/egress` and `platform/secrets`, and the standard library only: no vendor library, no `golang.org/x/net/http2`, because `net/http` speaks HTTP/2 itself. `architecture/modules.yaml` does not change.

    b. Public surface (names may follow the merged code, D-W-100; the fields are binding):

    ```go
    // SecretResolver resolves a secret reference; *secrets.Provider implements it (M1-09).
    type SecretResolver interface {
        Resolve(ctx context.Context, raw string) (secrets.Value, secrets.Version, error)
    }
    type Deps struct {
        Egress     egress.Config // egress.* through wire.EgressConfig
        EgressDeps egress.Deps   // resolver, root CAs, logger, metrics
        Secrets    SecretResolver
        Clock      clock.Clock
        Logger     *slog.Logger
        Endpoints  map[string]string // tests only: environment -> base URL; nil in production
    }
    func New(ctx context.Context, cfg Config, deps Deps) (*Adapter, error)
    func (a *Adapter) Name() string
    func (a *Adapter) Send(ctx context.Context, reg app.Registration, n app.Notification) (domain.SendResult, error)
    ```

    c. One egress client per adapter, built in `New` with `egress.Options{Purpose: "push_apns", Hosts: appleHosts, Redirects: egress.RedirectNever, HTTP2: true}`; `appleHosts` allows exactly scheme `https`, port `443` and the hosts `api.push.apple.com` and `api.sandbox.push.apple.com` (an operator needs no `egress.allowed_hosts` entry, 19 SEC-NET-2). Time and size limits are `egress.timeout` and `egress.max_response_bytes` (02 14.2; chapter 13 names no own deadline). Both environments share the client; `net/http` keeps one multiplexed HTTP/2 connection per host open between sends (13 12 point 5).

    d. `egress.Options.HTTP2` (additive in `internal/platform/egress`): when true the transport sets `ForceAttemptHTTP2`, so a TLS connection negotiates `h2` by ALPN; plain `http` (only with an `allow_private` rule) stays HTTP/1.1, there is no cleartext HTTP/2. The merged client says HTTP/2 stays off because `DialContext` and `TLSClientConfig` are set; replace that comment, keep every check, and name the option in the egress README. Reason: APNs accepts only HTTP/2 (13 5.1), and the egress package is the one place that builds transports.

    e. `Deps.Endpoints` exists only so that tests reach Fake-APNs: production code in `internal/wire` never sets it, and without it the base URLs are `https://api.push.apple.com` and `https://api.sandbox.push.apple.com`. A test that sets it still runs every egress check: the Fake listens on loopback, so the test configuration carries the egress rule `127.0.0.1:<port>;allow_private` (`apnstest.Server.EgressRule`) and trusts the Fake's certificate through `egress.Deps.RootCAs`.

2. **Configuration and start** (13 11, 17; 02 13 items 3 to 5; 02 14.4 row "secret provider"; Ruling for docs/DECISIONS.md, `D-M3-20-2`).

    a. Keys in `internal/wire/configkeys_push.go` (created by M3-19), each only if absent (M3-21 adds `push.ttl` too), with the chapter constant of 13 and the defaults of 13 17: `push.ttl` (duration, `24h`, positive), `push.apns.team_id` (string, empty), `push.apns.key_id` (string, empty), `push.apns.private_key_ref` (secret reference), `push.apns.topics` (string list, empty), `push.apns.jwt_refresh_interval` (duration, `40m`, `Min` `20m`, `Max` `60m`), `push.apns.retry_5xx_delay` (duration, `15m`, positive), `push.apns.time_sensitive_enabled` (bool, `false`). With the bounds on `push.apns.jwt_refresh_interval` the loader refuses a value outside 20 to 60 minutes in every role, naming the key and never the value: the server does not start (13 5.1).

    b. The default of `push.apns.private_key_ref` becomes `file:<secrets.dir>/apns_key.p8`: 13 17 says "empty", but the merged schema allows an empty default only for string and list keys, and `docs/operations/configuration.md` section 5 already names `apns_key.p8` as the secret behind this key. Change the row in 13 17 in the same change and regenerate the reference.

    c. `Config{TeamID, KeyID, PrivateKeyRef string; Topics []string; JWTRefreshInterval, Retry5xxDelay, TTL time.Duration; TimeSensitiveEnabled bool}`. `Enabled()` is true exactly when `TeamID`, `KeyID` and `Topics` are all non-empty. The cross-check `push_apns_complete` in `configChecks` refuses a configuration in which some but not all three are set, and a topic list with an empty or a repeated entry; its message names the keys. Add it to 02 13 item 4 ("Cross conditions today").

    d. Not enabled: `internal/wire` builds no adapter and reads no secret; jobs for `apns` registrations end with the skip reason `no_provider` of M3-19. Enabled: in the roles `worker` and `all` (the roles that run `notify.push_send`), `New` resolves `push.apns.private_key_ref` and parses the key (Design note 3); an error stops the start and names `push.apns.private_key_ref`, never a value (fail fast). The role `api` builds no adapter. Where the merged `bothy config check --secrets` resolves every `*_ref` key, it resolves this one only when APNs is enabled (the pattern of M3-02), because the default reference points at a file that an instance without APNs does not have.

3. **Key and JWT** (13 5.1, ADR 0067; Ruling for docs/DECISIONS.md, `D-M3-20-3`).

    a. `ParsePrivateKey(v secrets.Value) (*ecdsa.PrivateKey, error)`: exactly one PEM block of type `PRIVATE KEY`, PKCS#8 (`x509.ParsePKCS8PrivateKey`), an ECDSA key on P-256; everything else is an error whose text names the reason and never contains key bytes.

    b. The unexported signer implements `jose.Signer` of M1-33: `Alg()` returns `ES256`; `Sign` hashes the signing input with SHA-256 and returns `R || S` as exactly 64 bytes (each half left-padded to 32 bytes, RFC 7518 3.4).

    c. One JWT is `jose.SignCompact(signer, header, payload)` with a header of exactly `alg` (`ES256`) and `kid` (`push.apns.key_id`) and a payload of exactly `iss` (`push.apns.team_id`) and `iat` (the clock's Unix seconds). If the merged `jose.Header` cannot leave out a further field, add an option to `kernel/jose` that leaves it out (additive, with a test) rather than send a header 13 5.1 does not name.

    d. Every JWT creation resolves `push.apns.private_key_ref` again through `Deps.Secrets` (the provider caches for `secrets.cache_ttl`) and parses it, so a rotated key is used from the next JWT on without a restart (16 5.2). A resolve or parse error at that time is a configuration error of the environment (Design note 4 c) with one Error record that names the key.

4. **JWT cache and pause per environment** (13 5.1, 7.1, 11, 14 row "JWT cache"; Ruling for docs/DECISIONS.md, `D-M3-20-4` and `D-M3-20-5`).

    a. State per environment (`production`, `sandbox`): the current JWT with its creation time, the time of the last forced replacement and `pausedUntil`. One mutex per environment guards it, and creation happens under that lock, so any number of parallel sends wait for one creation. A JWT is reused while `now < created + push.apns.jwt_refresh_interval`; otherwise the first send that needs one creates it. So each environment creates at most one JWT per `push.apns.jwt_refresh_interval`. The two environments have separate state, because one key set serves both (13 11, Design note 11) and a key that Apple restricts to one environment must not stop the other.

    b. `403 ExpiredProviderToken` (13 7.1 "create a new JWT, retry once"): if the request used the current JWT and no forced replacement happened in this environment within the last `push.apns.jwt_refresh_interval`, create a replacement now (it starts a new interval) and send the same request once more with it; a parallel send that got the same answer for the same JWT reuses the replacement and also retries once. If a forced replacement already happened within the interval, or the retry answers `ExpiredProviderToken` again, the answer is a configuration error (c): a fresh token does not help, so the clock of the server or the key is wrong. This is the only case of two JWTs in one interval; write it into 13 5.1.

    c. Configuration error (Design note 6: the `403` reasons of the table, a failed key resolve or parse): set `pausedUntil = now + push.apns.jwt_refresh_interval` for the environment and drop its JWT; write one Error record (Design note 7); return `config_error`. While `now < pausedUntil`, `Send` makes no request and returns `config_error` without a log record (the job drops the hint and counts `rejected_other`). The first send at or after `pausedUntil` creates a new JWT, reading the key again, and so probes once: an answer other than a configuration error ends the pause with one Info record; a configuration error starts the next pause. Reason: 13 7.1 says "APNs sending paused" without an end; a probe per interval lets a rotated key or a restart heal the instance without a manual step, keeps the token updates within Apple's limit (the new JWT comes at least one interval after the dropped one), and hints dropped meanwhile are carried by the unread counts (13 7.3). The pause lives in the process; each worker probes for itself.

5. **Payload and headers** (13 5.1, 6.1, 8; ADR 0031; Ruling for docs/DECISIONS.md, `D-M3-20-6`).

    a. Payload of a hint, built from Go structs with JSON tags in the key order of 13 6.1 (no map, so no other key can appear):

    | Key | Value |
    |---|---|
    | `aps.alert.title-loc-key` | `PUSH_TITLE` |
    | `aps.alert.loc-key` | `PUSH_BODY_` plus the kind in upper case (`PUSH_BODY_MESSAGE` to `PUSH_BODY_ADMIN`) |
    | `aps.mutable-content` | `1` |
    | `aps.thread-id`, `b.t` | the thread ID; both absent when the hint has none (`security`, `admin`) |
    | `aps.category` | the kind in upper case (`MESSAGE`, `MENTION`, `DM`, `RUN_DONE`, `RUN_FAILED`, `INPUT_NEEDED`, `SECURITY`, `ADMIN`) |
    | `aps.sound` | `default`; absent when `Quiet` |
    | `aps.badge` | `Badge` of the `Notification` (M3-19 Design note 12, 13 8), in every hint; written also when it is `0` (no `omitempty`), because `0` clears the badge of the app icon |
    | `aps.interruption-level` | `passive` when `Quiet`; else `time-sensitive` for `input_needed` with `push.apns.time_sensitive_enabled`; else `active` |
    | `b.v` | `1` |
    | `b.nid` | the `Notification` ID |
    | `b.k` | the kind (`NotificationKind`) |
    | `b.i` | the instance ID |
    | `b.m` | the message ID; absent when the hint has none |

    UUIDs in canonical lower-case form. Quiet wins over `time-sensitive`, because 13 3.4 makes a hint in quiet hours quiet and only `notify.input_needed_breaks_quiet_hours` (decided by M3-17) clears `Quiet`. `Count` and `TargetID` of the `Notification` are not written: 13 6.1 has no key for them (`count` belongs to `notification.created`, 13 6.3), and the app reads both through the preview and its own sync.

    b. Headers, and no others: `authorization: bearer <JWT>`, `apns-push-type: alert`, `apns-id` = `nid`, `apns-topic` = `reg.AppID`, `apns-priority` `10` or `5` when `Quiet`, `apns-expiration` = Unix seconds of now plus `push.ttl`, `apns-collapse-id` = thread ID (36 bytes, under Apple's 64) and absent without thread, `content-type: application/json`. Path `/3/device/<token>`.

    c. Checks before any request (no JWT is used, the job sees the result at once): `reg.Provider` other than `apns`, an `Environment` other than `production` or `sandbox`, an unknown kind (neither a `NotificationKind` nor `KindSync`), a zero `nid` or instance ID, and `HasBadge` false (M3-19 sets it on every hint and every sync) give `program_error`; a token that is empty or not only hex digits gives `disable` with `token_invalid` (it can never be valid and would otherwise land in the URL path); an `AppID` that is not in `push.apns.topics` gives `disable` with `topic_mismatch` (this configuration can never serve the registration; Apple would answer `DeviceTokenNotForTopic`); a serialized payload above the Apple limit gives `program_error` and is never truncated. The limit is one named constant whose comment cites 13 6.1: it is Apple's fixed limit, not an operator value.

    d. The badge sync of 13 8 is a `Notification` with `Kind == KindSync` (M3-19 Design note 12: `ID`, `InstanceID`, `Badge`, `HasBadge` and `Cleared` set, `ThreadID`, `MessageID`, `TargetID`, `Quiet` and `Count` zero): payload exactly `{"aps":{"badge":<n>}}` with `<n>` = `Badge`, headers as b with `apns-id` = `ID`, `apns-priority: 5` and without `apns-collapse-id`. No `alert`, so the NSE does not run and nothing is shown (13 8). `Cleared` is not sent: APNs has no key for it, and on iOS shown hints stay until the app opens (13 8); FCM sends it (M3-21).

6. **Answer table** (13 7.1; Ruling for docs/DECISIONS.md, `D-M3-20-7`). `classify(status, reason, timestamp)` is a pure function; the answer body `{"reason": "...", "timestamp": <ms>}` is read within `egress.max_response_bytes`, and a missing or broken body leaves the status alone to decide. The class names are those of M3-19 (`domain.ResultClass`); M3-19 turns them into registration changes, retries and outcomes.

    | Answer | Class | Fields | Log (Design note 7) |
    |---|---|---|---|
    | `200` | `accepted` | | |
    | `410 Unregistered`, `410 ExpiredToken`, any other `410` | `disable` | `DisableReason` `token_invalid`; `InvalidSince` = `timestamp` when present | |
    | `400 BadDeviceToken` | `disable` | `token_invalid` | Warn |
    | `400 DeviceTokenNotForTopic` | `disable` | `topic_mismatch` | |
    | `403 ExpiredProviderToken` | renew and retry once (Design note 4 b) | | |
    | `403 InvalidProviderToken`, `MissingProviderToken`, `BadEnvironmentKeyIdInToken`, `UnrelatedKeyIdInToken`, `Forbidden`, any other `403` | `config_error`, pause | | Error |
    | `429 TooManyProviderTokenUpdates` | `program_error` | | Error |
    | `429 TooManyRequests`, any other `429` | `throttled` | `RetryAfter` zero: the job's backoff applies | |
    | `413 PayloadTooLarge`, any other `400`, `404`, `405`, any other `4xx` | `program_error` | | Error |
    | `500`, `503`, any other `5xx` | `retry` | `RetryAfter` = `push.apns.retry_5xx_delay` | |
    | any other status (`1xx`, `2xx` other than `200`, `3xx`) | `program_error` | | Error |
    | no answer (DNS, connect, TLS, HTTP/2, timeout, egress refusal, oversize body) | Go error | M3-19 counts it as transient | |

    The unlisted rows follow the nearest listed answer of the same status, because Apple documents the status as the class and the reason as detail [Q15]. `410` keeps the `timestamp` rule of 13 7.1: M3-19 leaves a registration created after `InvalidSince` active.

7. **Alarm and logs** (13 7.1, 7.3, 9, 10; SEC-LOG-4; signals.md 5.3, 7; Ruling for docs/DECISIONS.md, `D-M3-20-8`).

    a. "alert" in 13 7.1 means: one Error record (`apns configuration error`, `apns program error`) with `bothy.push.provider = apns` and `error.type` = the Apple reason when it is one of the reasons of the table, else `other`; and the outcome `rejected_other` that M3-19 records for `config_error` and `program_error`, which the SLO `bothy-push/provider-acceptance` counts against its budget, so `BothyPushAcceptanceBudgetBurn` fires and the status page of M7-09 sees the error share of `push.apns`. No new metric, attribute or alert rule.

    b. A pause logs once when it begins and once when a probe ends it, not per dropped hint.

    c. `400 BadDeviceToken`: 13 7.1 asks for an additional configuration alarm when the environment does not match the registration, but one answer cannot tell that from a removed app. The adapter writes one Warn record per answer with the fixed message `apns bad device token in production` or `apns bad device token in sandbox` (signals.md has no environment attribute) and no alarm; a mismatch shows as many such records. Write this into 13 7.1.

    d. Never in a log record, span, metric or error text: the device token (also no hash prefix; the adapter needs none), the JWT, the `authorization` header, key material, the topic, `nid`, instance, thread, message or person IDs, a name or a text. Errors of the egress client name method, scheme and host only (M1-22); the adapter wraps them with fixed text and never adds the URL.

8. **Fake-APNs** (`internal/notify/adapters/apns/apnstest`; PLAN 5 row "Test doubles", PLAN 9 "Test strategy"). A package without `testing` imports, so that the load test of M3-24 can use it as well:

    ```go
    type Options struct {
        TeamID, KeyID string
        PublicKey     *ecdsa.PublicKey // verifies the bearer JWT
    }
    type Response struct {
        Status    int       // 0 means 200
        Reason    string    // body {"reason": ...}; empty: no body
        Timestamp time.Time // body "timestamp" in ms for 410; zero: absent
    }
    type Request struct {
        Proto, Method, Token string
        Header               http.Header // a copy
        Body                 []byte
        JWT                  string
        KeyID, Issuer        string
        IssuedAt             time.Time
    }
    func Start(opts Options) (*Server, error) // HTTP/2 with TLS on 127.0.0.1:0
    func (s *Server) URL() string
    func (s *Server) RootCAs() *x509.CertPool
    func (s *Server) EgressRule() string                 // "127.0.0.1:<port>;allow_private"
    func (s *Server) Script(token string, rs ...Response) // answers in order, then 200
    func (s *Server) Requests() []Request
    func (s *Server) Connections() int                   // TLS connections accepted so far
    func (s *Server) SetPublicKey(k *ecdsa.PublicKey)    // after a key rotation
    func (s *Server) Close()
    ```

    It is built on `httptest.NewUnstartedServer` with `EnableHTTP2` and `StartTLS`. The handler answers `405` (`MethodNotAllowed`) to anything but `POST`, `404` (`BadPath`) to a path other than `/3/device/<token>`, and records the request (with `r.Proto`, so tests can assert HTTP/2). It then checks the bearer JWT like APNs does: missing gives `403 MissingProviderToken`; a token that does not verify with `jose.ParseCompactJWS` (only `ES256`) and `PublicKey`, or a wrong `kid` or `iss`, gives `403 InvalidProviderToken`. Only then it answers the script of the token. Every answer carries `apns-id`. It is safe for concurrent use. A test runs `go list -deps ./cmd/bothy` and fails if `apnstest` appears (pattern of `identitytest`, M1-33).

9. **Wiring** (02 13 item 5; M3-19 Design note 12). `internal/wire/push_apns.go` reads the keys of Design note 2 into `apns.Config`, and when `Enabled()` it builds the adapter with `wire.EgressConfig`, the process's `egress.Deps` (resolver, logger, the egress metrics), the secret provider, `clock.System()` and the logger, and adds it to the provider set of M3-19 under `Name()` in the roles `worker` and `all`. `Deps.Endpoints` stays nil there. The instance ID that `Notification` carries is set by M3-19's wiring; the adapter does not read `instance`.

10. **Concurrency.** `Adapter` is safe for concurrent use; the job of M3-19 bounds parallel sends with `push.max_concurrent_requests`; HTTP/2 multiplexes them over the shared connection. `Send` does no I/O besides the egress request and the secret resolve of a JWT creation, holds no lock during the request and honours the context deadline.

11. **Specification** (rule 10; nothing in 00, 03, 04 or 05 changes; all new text in English, like the existing text):

    - `docs/spec/13-push-notifications.md` 5.1: a paragraph: the connection runs through the egress client with HTTP/2 and the two Apple hosts as the adapter's targets; one JWT cache per environment, at most one JWT per `push.apns.jwt_refresh_interval`, the key read again at each creation, the one forced replacement after `403 ExpiredProviderToken` (Design note 4 b); the pause per environment with one probe after `push.apns.jwt_refresh_interval` (Design note 4 c).
    - 13 6.1: a paragraph: the value table of Design note 5 a (absent keys without thread or message, `Count` and `TargetID` not carried, quiet wins over `time-sensitive`), the checks of 5 c (unknown topic gives `topic_mismatch` without a request, size checked before sending, never truncated), the sync form of 5 d.
    - 13 7.1: a paragraph after the table: the rows "any other" of Design note 6, a repeated `ExpiredProviderToken` is a configuration error, transport errors are transient, what "alert" means (Design note 7 a), and the `BadDeviceToken` rule of 7 c.
    - 13 11: replace "the configuration keeps sandbox and production separate" by the English rule: one key set (`push.apns.team_id`, `push.apns.key_id`, `push.apns.private_key_ref`) serves both environments; the JWT cache and the pause are kept per environment; a key restricted to one environment answers `BadEnvironmentKeyIdInToken` in the other and pauses only that one. Reason: 13 17 and `configuration.md` have one key set, and Apple keys work for both environments unless restricted [Q3].
    - 13 17: default of `push.apns.private_key_ref` `file:<secrets.dir>/apns_key.p8` (Design note 2 b).
    - `docs/spec/02-architecture.md` 13 item 4: the cross-check `push_apns_complete`.
    - `internal/platform/egress/README.md`: `Options.HTTP2`.

12. **Rulings for `docs/DECISIONS.md`** (section 10, one row each with date, decision, reason and place): `D-M3-20-1` egress with `Options.HTTP2`, the Apple hosts as the adapter's policy, `Deps.Endpoints` for tests only (note 1); `D-M3-20-2` APNs enabled exactly with team ID, key ID and topics, the cross-check, the default of `push.apns.private_key_ref`, the start check in `worker` and `all`, the conditional secret check (note 2); `D-M3-20-3` key format, signer, header of exactly `alg` and `kid`, the key read at each JWT (note 3); `D-M3-20-4` JWT cache per environment and the forced replacement (note 4 a, b); `D-M3-20-5` pause per environment with a probe after one interval (note 4 c); `D-M3-20-6` payload and header rules, the pre-checks and the sync form (note 5); `D-M3-20-7` the answer table with its unlisted rows (note 6); `D-M3-20-8` alarm and logs, the `BadDeviceToken` rule and one key set for both environments (notes 7, 11). Rulings 4 to 8 also go into 13 (note 11).

## Tests (write them first)

Test world: Alex has an iPhone (`ios`) and a Mac (`macos`), each with an `apns` registration: the phone `production`, topic `com.example.chat`; the Mac `sandbox`, topic `com.example.chat.mac`. Sam has no Apple device. Configuration in tests: `push.apns.team_id = EXAMPLETEAM`, `push.apns.key_id = EXAMPLEKEY`, both topics, `push.apns.private_key_ref` pointing at a `file` secret in `t.TempDir()` with a P-256 key generated per run; durations are fixture values of the test configuration. Every adapter in tests talks to Fake-APNs (one `apnstest.Server` per environment) through `Deps.Endpoints`, `egress.allowed_hosts` with the Fake's `EgressRule` and `egress.Deps.RootCAs` with its `RootCAs`; the clock is `clock.Fake`. Unless a test says otherwise, `Send` gets a `Notification` of kind `message` with thread, message, `nid` and instance ID.

Unit, `internal/notify/adapters/apns` and `apnstest`:

- `TestConfigEnabledAndValidate`: all three of team ID, key ID and topics set gives enabled; none gives disabled; each partial combination fails the cross-check `push_apns_complete` with a message naming the three keys; an empty or repeated topic fails.
- `TestParsePrivateKey`: a generated P-256 key in PKCS#8 PEM parses; a P-384 key, an RSA key, a SEC 1 `EC PRIVATE KEY` block, two PEM blocks, garbage and an empty value fail; no error text contains a byte of the input.
- `TestJWT_HeaderAndClaims` (13 5.1): the JWT that Fake-APNs receives verifies with the public key through `jose.ParseCompactJWS` and `JWS.Verify`; its header has exactly `alg = ES256` and `kid = EXAMPLEKEY`, its payload exactly `iss = EXAMPLETEAM` and `iat` = the fake clock in Unix seconds; the signature is 64 bytes.
- `TestJWTCache_AtMostOnePerInterval` (PLAN row "at most one JWT per `push.apns.jwt_refresh_interval`", 13 14 row "JWT cache"): 200 parallel sends in `production` (`-race`) give exactly one JWT creation (counting signer calls) and one distinct JWT at the Fake; the clock moved to one second before the interval end: still one; moved to the interval end: exactly two; 200 further parallel sends then: still two; `sandbox` counts its own JWTs and is not renewed by `production`.
- `TestJWTCache_KeyRotation`: the secret file gets a new key and the Fake the new public key; after `secrets.cache_ttl` and the next interval the next JWT is signed with the new key and accepted, without a new adapter.
- `TestPayloadAllowList` (PLAN row "payload allow-list", 13 14, ADR 0031 Confirmation): for every kind of `NotificationKind` times `Quiet` times `push.apns.time_sensitive_enabled` times with and without message ID times `Badge` `0` and `3`, and for the sync form (`KindSync`): the request body parsed as generic JSON has only the top-level keys `aps` and `b` (the sync form only `aps`), `aps.badge` is present and equals `Badge` in every case, including `0`, `aps` only keys of 13 6.1, `aps.alert` only `title-loc-key` and `loc-key`, `b` only `v`, `nid`, `k`, `i`, `t`, `m`; `nid`, `i`, `t`, `m` and `thread-id` are canonical UUIDs, `v`, `mutable-content` and `badge` are numbers, `loc-key`, `title-loc-key`, `category`, `sound`, `interruption-level` and `k` come from their fixed sets; `security` and `admin` have no `thread-id` and no `t`; a `reflect` walk of the payload structs finds no string field outside these enums and IDs, no map and no interface.
- `TestPayloadSizeUnderAppleLimit` (PLAN row "size below the Apple limit"): every combination of `TestPayloadAllowList`, with the badge at the largest value its type allows, serializes to fewer bytes than the Apple limit of 13 6.1; a payload made larger through an unexported test hook gives `program_error` and the Fake counts no request.
- `TestPayloadMatchesChapterExample`: the test reads the JSON block of 13 "6.1 APNs" from `docs/spec/13-push-notifications.md` (walking up to the directory that holds `docs/spec`), builds the payload for the same kind, IDs and badge, not quiet, and compares both as JSON values; `testdata/payload_message.golden` holds the exact bytes and `-update` rewrites it.
- `TestInterruptionLevel`: not quiet gives `active`; quiet gives `passive` for every kind; `input_needed` not quiet with `push.apns.time_sensitive_enabled` gives `time-sensitive`, without it `active`; `input_needed` quiet with the switch gives `passive`; no combination gives `critical`; quiet hints have no `sound`.
- `TestHeaders` (13 5.1): `apns-push-type` `alert`; `apns-id` = `nid`; `apns-topic` = the registration's `app_id`; `apns-priority` `10`, quiet `5`, sync form `5`; `apns-expiration` = fake clock plus `push.ttl` in Unix seconds; `apns-collapse-id` = thread ID, absent for `security`, `admin` and the sync form; the sync form has `apns-id` = its `ID` and `apns-push-type` `alert`; `authorization` `bearer` plus the JWT; `content-type` `application/json`; no other header; path `/3/device/<token>`; method `POST`.
- `TestBadgeSyncForm` (13 8, Design note 5 d): a `Notification` with `Kind` `KindSync`, `Badge` `2`, `HasBadge` true and two thread IDs in `Cleared` sends a body of exactly the bytes `{"aps":{"badge":2}}` with `apns-priority` `5` and no `apns-collapse-id`; `Badge` `0` sends exactly `{"aps":{"badge":0}}`; neither body nor headers contain a `Cleared` ID; the answer table applies to it like to a hint (`410` gives `disable`).
- `TestPrechecksWithoutRequest` (Design note 5 c): provider `fcm`, environment `staging`, an unknown kind, a zero `nid` and a hint or a sync with `HasBadge` false give `program_error`; a token with a non-hex character gives `disable` with `token_invalid`; `app_id` `com.example.other` gives `disable` with `topic_mismatch`; the Fake counts no request and no JWT is created.
- `TestAnswerTable` (PLAN row "every response from 13 'APNs' triggers the expected consequence", 13 14 row "Fake APNs (HTTP/2) with every response from 7.1"): one subtest per row of Design note 6 with the Fake scripted for the token: the returned `SendResult` (class, `DisableReason`, `InvalidSince` equal to the scripted `timestamp` for `410`, `RetryAfter` equal to `push.apns.retry_5xx_delay` for `500` and `503`, zero for `429`), whether a Go error is returned (Fake closed; egress refusal) and the log level; the unlisted rows with `410 Other`, `403 Other`, `429 Other`, `404 BadPath`, `502`, `204` and `301`.
- `TestExpiredProviderToken_RenewOnce`: script `403 ExpiredProviderToken` then `200`: two requests, the second with a new JWT, result `accepted`; script it twice: `config_error` and the environment pauses; ten parallel sends that all get `ExpiredProviderToken` first create exactly one replacement JWT and each retries once; a second `ExpiredProviderToken` within the same interval after a replacement gives `config_error` without a new JWT.
- `TestConfigErrorPausesEnvironment`: `403 InvalidProviderToken` in `production` gives `config_error` and one Error record; the next 50 sends in `production` give `config_error` with no request at the Fake and no further log record; `sandbox` sends normally; at `pausedUntil` exactly one probe request with a new JWT goes out; `200` ends the pause with one Info record and the following sends go out; a probe answered `403 BadEnvironmentKeyIdInToken` starts the next pause. The same for a secret file deleted before a JWT creation (resolve error): `config_error`, pause, the Error record names `push.apns.private_key_ref`.
- `TestHTTP2AndSharedConnection` (13 5.1, 13 12 point 5): every request at the Fake has `Proto` `HTTP/2.0`; 100 sends to one environment, 50 of them in parallel, open exactly one TLS connection at its Fake (`Connections()`).
- `TestEgressOnly` (19 SEC-NET-1, SEC-NET-2): without the Fake's rule in `egress.allowed_hosts` a send returns an error with `errors.Is(err, egress.ErrBlocked)` and the Fake counts no request; the adapter's host policy allows `https` on port `443` for exactly `api.push.apple.com` and `api.sandbox.push.apple.com` and refuses `push.apple.com`, port `2197` and `http`; an adapter built without `Deps.Endpoints` targets exactly those two base URLs.
- `TestNoSecretsInLogsOrErrors` (13 7.3, SEC-LOG-4, SEC-SEC-3): with a captured `slog` handler, run every row of `TestAnswerTable`, a pause, a probe, a key rotation and a send against a closed Fake; neither the captured records nor any returned error text contain the token, a prefix of it longer than four characters, the JWT, a line of the PEM key, the topic, `nid`, the instance, thread or message ID; Error records carry exactly `bothy.push.provider` and `error.type`.
- `apnstest`: `TestFakeVerifiesJWT` (missing header gives `403 MissingProviderToken`, a JWT of another key, a wrong `kid` or a wrong `iss` give `403 InvalidProviderToken`, a valid one the scripted answer), `TestFakeRequiresPostAndPath` (`405`, `404`), `TestFakeScriptOrder` (scripted answers in order, then `200`), `TestFakeNotInBinary` (`go list -deps ./cmd/bothy` does not list `apnstest`).

Unit, `internal/platform/egress`:

- `TestHTTP2Option`: against an `httptest` TLS server with `EnableHTTP2` behind an `allow_private` rule and its root CAs, a client with `HTTP2: true` gets `resp.ProtoMajor == 2`, without it `1`; a plain `http` target with `HTTP2: true` stays HTTP/1.1; every existing egress test passes unchanged.

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, the job handler of `notify.push_send` of M3-19 called directly after moving the fake clock; registrations are fixture rows written through the `pgtest` connection, because `registerPush` comes with M3-22):

- `TestConfigLoad_JWTRefreshIntervalBounds` (13 14 row "JWT cache": "start error outside 20 to 60 minutes"): loading the configuration with `push.apns.jwt_refresh_interval` `19m59s` or `60m1s` fails with a message naming the key and not the value, so `bothy serve` does not start; `20m`, `40m` and `60m` load. `TestConfigSchemaBuilds` lists every key of Design note 2.
- `TestAPNsWiring`: APNs not enabled: the provider set has no `apns`, no secret is read (a missing key file is no error), and a job for Alex's phone ends with `no_provider`; enabled: the set has `apns` in the roles `worker` and `all` and none in `api`, built without `Endpoints`; enabled with a missing key file or a P-384 key: the start fails naming `push.apns.private_key_ref`.
- `TestPushSendThroughAPNs` (PLAN row "every response from 13 'APNs' triggers the expected consequence", end to end through the job): one `notify.push_send` for Alex's phone per answer, the registration row and the jobs checked after the handler: `200` sets `last_success_at` and `failure_count = 0`; `410 Unregistered` with a `timestamp` after `created_at` sets `disabled_at` and `disabled_reason = 'token_invalid'`, with a `timestamp` before `created_at` leaves the row active; `400 BadDeviceToken` disables with `token_invalid`; `400 DeviceTokenNotForTopic` with `topic_mismatch`; `429 TooManyRequests` leaves `failure_count` and inserts the retry job; `500` raises `failure_count` by one and inserts the retry job no earlier than `push.apns.retry_5xx_delay`; `403 InvalidProviderToken` and `413 PayloadTooLarge` leave the row, insert no retry and count `bothy.push.sent` with `rejected_other` (SDK manual reader); `403 ExpiredProviderToken` then `200` gives one hint delivered with two requests.
- `TestHintReachesFakeAPNs` (PLAN 9 "Red team": content in push payloads): the Owner writes a message whose text contains a canary string in a thread Alex follows; the consumer `push` and the job deliver exactly one request to the Fake for Alex's phone with `b.k = message`, `b.t` = the thread, `b.m` = the message, `b.i` = the instance ID, `b.nid` = the job's `NotificationID` and `aps.badge` = `BadgeCounter.For` of M3-19 for Alex at send time; neither body nor headers contain the canary string, a display name, the space or channel name.
- `TestBadgeSyncThroughAPNs` (13 8): after that hint Alex reads the thread on the Mac through `putReadState`; the consumer `push` and the jobs `notify.badge_sync` and `notify.push_send` of M3-19 deliver exactly one request to the Fake of Alex's phone with the body `{"aps":{"badge":<n>}}`, `<n>` = `BadgeCounter.For` for Alex after the read, and `apns-priority` `5`; the Fake of the Mac (`macos`, 13 8: mobile devices only) counts no sync request.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/notify/... ./internal/platform/egress/... ./internal/kernel/jose/... ./internal/wire/... ./cmd/...`
- `make semgrep` (the rules `bothy-go-egress-only` and `bothy-go-tls-verify` stay green without a new `nosemgrep`), `make lint`, `make arch`
- `make config-reference` (no diff afterwards), `make config-doc-check`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `internal/notify/adapters/apns` implements `PushProvider` of M3-19 for `provider = apns` and speaks HTTP/2 over TLS to the two Apple hosts by `environment`, only through the egress client with `Options.HTTP2`; connections are shared.
- [ ] Every answer of 13 7.1, and every unlisted answer of Design note 6, gives the expected result class and, through the job of M3-19, the expected change of the registration, retry or drop; `ExpiredProviderToken` renews the JWT and retries once; a configuration error pauses only its environment until one probe after `push.apns.jwt_refresh_interval` (`TestAnswerTable`, `TestPushSendThroughAPNs`).
- [ ] At most one JWT per environment per `push.apns.jwt_refresh_interval` under parallel load, the only exception being the one replacement after `ExpiredProviderToken`; the configuration does not load with a value outside 20 to 60 minutes (`TestJWTCache_AtMostOnePerInterval`, `TestConfigLoad_JWTRefreshIntervalBounds`).
- [ ] The serialized payload holds only the keys and value types of 13 6.1 for every kind and the sync form, matches the example of 13 6.1 and stays under the Apple limit; headers follow 13 5.1 (`TestPayloadAllowList`, `TestPayloadSizeUnderAppleLimit`, `TestPayloadMatchesChapterExample`, `TestHeaders`).
- [ ] Every hint carries `aps.badge` = `Badge` of the `Notification`, also `0`; a `Notification` with `Kind == KindSync` is sent as exactly `{"aps":{"badge":<n>}}` with `apns-priority` `5`, without `Cleared`, and reaches only mobile registrations through the jobs of M3-19 (`TestPayloadAllowList`, `TestBadgeSyncForm`, `TestHintReachesFakeAPNs`, `TestBadgeSyncThroughAPNs`).
- [ ] No device token, JWT, key, ID or text appears in logs, spans or error texts; a message text never reaches Fake-APNs (`TestNoSecretsInLogsOrErrors`, `TestHintReachesFakeAPNs`).
- [ ] Fake-APNs (`apnstest`) verifies the JWT, requires HTTP/2 with TLS on loopback, answers by script and is not part of the server binary.
- [ ] 13 5.1, 6.1, 7.1, 11 and 17, 02 13, `configuration.md`, the egress and `notify` READMEs, the CHANGELOG and the rows `D-M3-20-1` to `D-M3-20-8` are updated; `make semgrep`, `make arch` and `make config-doc-check` are green.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `notify` updated in English (the adapter `apns`: configuration, endpoints, JWT cache, pause, answer table, payload allow list, Fake-APNs), keeping the line `May use the ports of these modules: ...` and the section `## Tables` in their form for M1-06; README of `internal/platform/egress` names `Options.HTTP2`.
- Rulings of Design note 12 recorded in `docs/DECISIONS.md` (one row each with reason); rulings 4 to 8 also in `docs/spec/13-push-notifications.md`.
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
4. What is not proven (at least: behaviour against the real APNs service, which no test contacts; the answer `BadEnvironmentKeyIdInToken` of a restricted key, proven only against Fake-APNs).
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
