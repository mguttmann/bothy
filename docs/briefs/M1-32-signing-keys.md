# M1-32: Signing keys, JWKS and rotation

Status: ready | Depends on: M1-14, M1-15, M1-20, M1-24, M1-28 | Etappe: M1

## Goal

After this ticket the server owns one Ed25519 signing key per purpose (`access_token`, `invocation`, `webhook_out`, `audit_checkpoint`) with the states `next`, `active`, `retiring` and `retired`. The private part lives only in the Secret-Provider; the database holds the public JWK, the state and a reference. Keys rotate on schedule without invalidating any signature that is still valid, an owner with step-up can rotate on demand or in an emergency that withdraws the old key at once, and the public JWKS is served for clients and agent adapters. Access tokens (M1-34), invocation tokens (M2), outgoing webhooks (M2) and audit checkpoints (M1-50) sign through this ticket's port.

The plan dependencies list M1-14, M1-15 and M1-28; M1-20 (`Idempotency-Key` on `rotateSigningKey`) and M1-24 (audit of `signing_key.rotate`) are needed as well and are merged in earlier waves.

## Read first

- `docs/PLAN.md`: section 3, section 7 (M1), rows M1-09, M1-23, M1-32, M1-34, M1-37, M1-50, M1-73.
- `docs/spec/03-datenmodell.md`: 1 (principle 9), 4.1 `instance` and `server_signing_keys`, 4.12 `audit_checkpoints` (column `kid`), 6.3.
- `docs/spec/05-aktionen.md`: table 4.1 (`instance.discovery.read`), table 4.2 (`signing_key.rotate`), 5 (invocation token header and claims).
- `docs/spec/08-identitaet.md`: 4 (bootstrap step 3), 7.1 and 7.2 (access tokens, verification with `active` and `retiring`).
- `docs/spec/09-agenten.md`: 4.5 (Aufruf-Marke, Schluessel und Rotation).
- `docs/spec/11-erweiterungen.md`: 7 (Ausgehende Webhooks, key purpose `webhook_out`).
- `docs/spec/16-betrieb.md`: 5.1 to 5.4 (Secret-Provider, compose secrets, escrow), 4.2 (health), 14.
- `docs/spec/19-sicherheit.md`: SEC-CORS-2, 9 (SEC-LOG-1 to SEC-LOG-4), 10 (Geheimnisse, SEC-SEC-1 to SEC-SEC-5), 12 (SEC-AUD-2), 16.
- `docs/spec/02-architektur.md`: 4.2 and 4.3 (row `instance`), 5, 12 (jobs), 13, 15.1 and 15.2.
- `docs/spec/20-datenschutz.md`: row `server_signing_keys`.
- `docs/api/openapi.yaml`: operations `getJwks`, `rotateSigningKey`; schemas `Jwks`, `SigningKeyRotate`, `SigningKeyRotation`, `SigningKeyPurpose`.
- `docs/operations/configuration.md` section 5 (Geheimnisse), `docs/operations/disaster-recovery.md` (Fall D), `docs/security/threat-model.md` (T-TK-04), `infra/compose/compose.yaml` (service `server`, volume `secrets`), `services/server/Dockerfile` (M1-23).
- ADRs: 0011, 0012, 0020, 0021, 0023, 0016.
- Merged code: `internal/platform/secrets` and `bothy secrets` (M1-09), `internal/platform/jobs` (M1-14), `internal/instance` with `instance` and `sequence_counters` (M1-15), the HTTP chain and problem writer (M1-18), idempotency (M1-20), `auditing.Recorder` (M1-24), `authorize` (M1-28), lifecycle and health (M1-11), config loader (M1-07) and generator (M1-08). Their names win where they differ from this brief only in naming.
- RFC 7517 (JWK), RFC 7638 (JWK thumbprint), RFC 8037 (OKP keys, `EdDSA`).

## Fixed requirements that apply

1. No MVP. Built from scratch directly in its final state; etappen are only the build order.
2. Strictly modular: clear module and service boundaries, dependencies only through defined interfaces, no cross access.
3. Documentation in code: every public module, class, function and interface has a docstring or doc comment (purpose, parameters, return value, errors, example where useful); comments explain why, not what. API documentation is generated from code (OpenAPI from the routes, dartdoc, Python API docs in the docs site).
4. Documentation in the repo: README per module/service, an ADR for every architecture decision, a CHANGELOG entry per release, docs site kept current. A change without matching documentation is not done.
5. Quality gates (SonarQube, linters, type checks, security scanners, test coverage) block the merge.
6. Test first: red before green, red output kept for the report.
7. No en dash or em dash in any file; White Label.
8. Configuration values only through named keys listed in `docs/operations/configuration.md`; the new keys of this ticket are added to their chapter tables first and the reference is regenerated.
9. The private part of a signing key exists only in the Secret-Provider; `server_signing_keys` holds the public JWK, the state and `private_key_ref` (03; 19 section 10; SEC-SEC-3). A private key never appears in the database, a log, telemetry, an audit entry, an error or a response.
10. States `next`, `active`, `retiring`, `retired`; exactly one `active` key per purpose (partial unique index of 03); a key stays `retiring` until the longest validity of anything signed with it has passed (19 section 10); the JWKS publishes `next`, `active` and `retiring` keys (09 section 4.5).
11. `invocation` and `webhook_out` keys use only `EdDSA` (03 check). This ticket generates Ed25519 keys for every purpose (08 section 7.1 names `EdDSA` for access tokens).
12. Scheduled rotation of `invocation` keys after `signing_key.invocation_rotation_interval`, with the new key published for at least `agent.jwks_max_age` before it signs (09 section 4.5).
13. Emergency rotation withdraws the old key immediately (09 section 4.5 point 3); `signing_key.rotate` is an owner action with step-up and audit (05 table 4.2).
14. JWKS is public (`instance.discovery.read`, `security: []`), carries no private member, and allows any origin without credentials (SEC-CORS-2).

## Scope

### 1. Specification first (first commit)

The current specification cannot work as written: the `file` and `env` drivers of the Secret-Provider are read-only (16 section 5.2) and the server mounts its secrets read-only (compose), yet the server must create new keys for scheduled and emergency rotation. Resolve it in the documents before the code, as follows.

- New ADR `docs/adr/NNNN-signierschluessel-im-beschreibbaren-schluesselspeicher.md` (next free number at merge time; MADR 4, English MADR headings, German text, status accepted). Options: (a) a writable key store directory owned by the server, written only through `Put` of the `file` driver; (b) keys pre-generated by `bothy secrets init` with spares that the server only switches between; (c) private keys encrypted with a key-encryption key and stored in the database. Decision (a). Reasons: (b) cannot rotate automatically without a host action and an attacker who read the secrets directory also holds every spare; (c) contradicts 03 (private part only in the Secret-Provider). Consequences: one writable volume for the server; the server can create but never read other secrets in it; multi-node operation (M7) needs a shared volume or a writable driver (`azkv`, `awssm`). Confirmation: the tests of this ticket by name.
- `docs/spec/16-betrieb.md`: 5.2 table, column Schreiben of `file`: only below `secrets.signing_key_dir`; 5.3: signing keys are no longer created by `secrets-init` but by the server in `secrets.signing_key_dir`; 5.4: `bothy secrets export` includes the key store; 14: new key `secrets.signing_key_dir`, default `/var/lib/bothy/signing-keys`, meaning writable directory for the private signing keys, mode 0700, owned by the service user.
- `docs/spec/19-sicherheit.md`: section 10, row of the private signing keys: created by the server, Ed25519, `kid` = RFC 7638 thumbprint, file `<purpose>-<kid>.pem` in `secrets.signing_key_dir`, private file deleted when the key becomes `retired`; section 16: `signing_key.cache_ttl` (default `30s`, longest time a process accepts a withdrawn key or misses a new one) and `signing_key.rotation_check_interval` (default `1m`, period of `instance.key_rotation`).
- `docs/spec/02-architektur.md` 15.1: new step after step 5, Signierschluessel sicherstellen (19 section 10); 15.2: `/readyz` also fails with the check name `signing_keys` while a purpose has no `active` key.
- `docs/spec/09-agenten.md` 4.5 point 3: emergency rotation is `signing_key.rotate` with `mode = emergency`; a manual scheduled rotation uses `mode = scheduled`.
- `docs/api/openapi.yaml`: `SigningKeyRotate` gets the required member `mode`, enum `[scheduled, emergency]`, with a description of both effects; update the example. Required instead of defaulted, because the two modes have opposite effects and nothing is released yet. Regenerate (`make codegen-check` without drift), `make api-lint`.
- `docs/operations/configuration.md`: regenerate with the M1-08 tool (`make config-doc-check` green); in section 5 replace the row `signing/<zweck>-<kid>.pem` by the key store row (`<purpose>-<kid>.pem` in `secrets.signing_key_dir`, created by the server).
- `docs/operations/disaster-recovery.md` Fall D: `signing_key.rotate` with `mode = emergency`.
- `docs/security/threat-model.md` T-TK-04: countermeasures add key store mode 0700, deletion of retired private keys, emergency rotation.

### 2. Secret-Provider (`internal/platform/secrets`)

- Add `Put(ctx, ref string, value Secret) error` and `Delete(ctx, ref string) error`. Only the `file` driver supports them, and only for references below `secrets.signing_key_dir`; every other reference returns `ErrReadOnly`; `env` never writes.
- `Put` creates the file with mode 0600 in the same directory under a temporary name, writes, fsyncs, links it to the final name without overwriting (an existing name is an error), removes the temporary name, and fsyncs the directory.
- Startup check (roles `api`, `worker`, `all`): `secrets.signing_key_dir` exists, is a directory owned by the process user with mode 0700, and is writable; otherwise the start aborts with a message naming the key and the expected mode (same style as SEC-SEC-1).
- `bothy secrets export` (M1-09) includes the files of `secrets.signing_key_dir` in the age-encrypted bundle.
- Add `secrets.signing_key_dir` to the configuration schema (M1-07).

### 3. Port (root package `internal/instance`, file `keys.go`)

```go
// Purpose of a server signing key (03 server_signing_keys.purpose).
type Purpose string // access_token, invocation, webhook_out, audit_checkpoint

// KeyState of a server signing key: next, active, retiring, retired.
type KeyState string

// Signer signs with one fixed key; KeyID and Alg go into the JWS header before signing.
type Signer interface {
    KeyID() string
    Alg() string // always EdDSA
    Sign(ctx context.Context, signingInput []byte) ([]byte, error)
}

// Keys is the port of the signing keys.
type Keys interface {
    Signer(ctx context.Context, p Purpose) (Signer, error)                   // active key, read fresh
    VerificationKey(ctx context.Context, p Purpose, kid string) (PublicKey, error) // next, active or retiring
    PublicKey(ctx context.Context, kid string) (PublicKey, error)             // any state, for checkpoint verification
    JWKS(ctx context.Context) (JWKSet, error)                               // published set
    Rotate(ctx context.Context, a actor.Actor, cmd RotateCommand) (Rotation, error)
    EnsureActive(ctx context.Context) error
}
```

`PublicKey{KID, Purpose, Alg, State, Key ed25519.PublicKey, NotAfter}`, `RotateCommand{Purpose, Mode, Reason}`, `Rotation{Purpose, NewKID, PreviousKID, PreviousNotAfter}`. JOSE assembly (header, base64url, compact form) stays with the token issuers (M1-34 and later); this port signs raw signing input with the standard library only. Callers in M1-34, M1-37, M1-50 and M2 use this port; nobody else reads `server_signing_keys`.

### 4. Domain (`internal/instance/domain/signingkey.go`)

Pure functions over a list of keys and a time `t`, table tested:

| Transition | Condition | Effect |
|---|---|---|
| create `next` (scheduled) | purpose has a rotation interval (only `invocation`: `signing_key.invocation_rotation_interval`), no `next` exists, `active.created_at + interval <= t` | new key `next` |
| promote | a `next` exists and `next.created_at + agent.jwks_max_age <= t` | old `active` to `retiring` with `not_after = t`; `next` to `active` |
| retire | `retiring` key with `not_after + window(purpose) <= t` | `retired` |
| emergency | `Rotate` with `mode = emergency` | every `next`, `active`, `retiring` key of the purpose to `retired` (`not_after = t` where unset); a fresh key `active` |
| manual scheduled | `Rotate` with `mode = scheduled` | if a `next` exists, return it; else create `next`; the job promotes it |

Window per purpose (longest validity of a signature after the key stopped signing):

| Purpose | Window |
|---|---|
| `access_token` | `auth.access_token_ttl` + `auth.clock_leeway` |
| `invocation` | `agent.invocation_token_ttl` + `auth.clock_leeway` |
| `webhook_out` | `webhook.delivery_timeout` + `auth.clock_leeway` (every delivery attempt signs anew) |
| `audit_checkpoint` | zero; old checkpoints are verified through `PublicKey` in any state |

### 5. Application (`internal/instance/app/keys.go`) and outbound port

- Outbound port `KeyStore` in `internal/instance/app`: `Generate(ctx, purpose) (kid string, pub ed25519.PublicKey, ref string, err error)`, `Load(ctx, ref) (ed25519.PrivateKey, error)`, `Remove(ctx, ref) error`. Adapter `internal/instance/adapters/keystore`: Ed25519 from `crypto/rand`; PEM `PRIVATE KEY` (PKCS#8, `x509.MarshalPKCS8PrivateKey`); `kid` = RFC 7638 thumbprint (SHA-256 over `{crv, kty, x}` in that order, base64url without padding); `ref` = `file:<secrets.signing_key_dir>/<purpose>-<kid>.pem`; writes through `secrets.Put`, reads through `secrets.Get` (cached `secrets.cache_ttl`).
- Every transaction that changes keys of a purpose first takes `pg_advisory_xact_lock(hashtext('bothy.server_signing_keys'), hashtext(<purpose>))`; it serializes creation, promotion and rotation, also when no row exists yet.
- New key: generate and store the file first, then insert the row in the transaction; if the transaction fails, remove the file (best effort, logged with `kid` only). After a crash between both steps an unreferenced file may remain; it is never used and is documented in the README.
- After the commit of any transition to `retired`, remove the private file (best effort, logged with `kid` only). `private_key_ref` stays as a record.
- `EnsureActive`: for each of the four purposes without an `active` key, create one directly as `active`. Called at start in every role (02 15.1) and later by `bothy setup bootstrap-owner` (M1-37).
- `Signer`: reads the `active` row of the purpose fresh on each call (one indexed row), loads the private key, returns a `Signer` for that key.
- `VerificationKey`: per-process cache of all `next`, `active` and `retiring` keys, refreshed after `signing_key.cache_ttl`; an unknown `kid` triggers at most one reload per `signing_key.cache_ttl` (singleflight), otherwise `not found`. A `retired` key is never returned.
- `Rotate`: `authorize(actor, signing_key.rotate, instance)`, then the transition, then the allow audit entry in the transaction (`details = {purpose, mode, new_kid, previous_kid, reason}`). `PreviousNotAfter` = `next.created_at + agent.jwks_max_age` for `scheduled`, `t` for `emergency`.
- Job `instance.key_rotation` (queue `maintenance`, period `signing_key.rotation_check_interval`, roles `worker` and `all`, no arguments): applies the due transitions of section 4 per purpose, each in its own transaction, logs each transition with purpose, `kid`, from and to state.

### 6. Persistence

Migration `services/server/migrations/<next number>_server_signing_keys.sql` (goose): table `server_signing_keys` exactly as 03, with every check (purpose, alg, state, `EdDSA` only for `invocation` and `webhook_out`), the partial unique index `(purpose) WHERE state = 'active'` and the index `(purpose, state)`. sqlc queries in `internal/instance/adapters/postgres/queries/keys.sql`.

### 7. HTTP (`internal/instance/adapters/httpapi`)

- `getJwks`: keys of the purposes `access_token` and `invocation` in the states `next`, `active`, `retiring`, ordered by purpose, state (`active`, `next`, `retiring`) and `kid`; each JWK exactly `{kty: OKP, crv: Ed25519, x, kid, alg: EdDSA, use: sig}`; content type `application/jwk-set+json`; `Cache-Control: public, max-age=<agent.jwks_max_age in seconds>`; `Access-Control-Allow-Origin: *` without credentials (SEC-CORS-2; use the mechanism of M1-18 or M1-21 for public routes if it exists, else set it in the handler). `webhook_out` keys are published by `getWebhookKeys` (M2); `audit_checkpoint` keys are not published.
- `rotateSigningKey`: maps `SigningKeyRotate` to `RotateCommand`, returns `200 SigningKeyRotation`. `audit_checkpoint` is not in `SigningKeyPurpose`, so the schema rejects it with `400`.
- Move `getJwks` and `rotateSigningKey` into the built section of `services/server/api/operations.yaml` (P-04).

### 8. Start, health, image, compose

- `internal/wire` and `internal/platform/lifecycle`: start step `EnsureActive` after loading the instance (02 15.1 as updated); `/readyz` check `signing_keys`.
- `services/server/Dockerfile`: create `/var/lib/bothy/signing-keys` owned by the unprivileged runtime user with mode 0700, so a named volume inherits owner and mode.
- `infra/compose/compose.yaml`: named volume `signing-keys`, mounted read-write at `/var/lib/bothy/signing-keys` in service `server` only. This keeps the installation compose consistent with the specification; it does not make it startable (P-06). Validate with `docker compose -f infra/compose/compose.yaml config --quiet`.

### 9. Configuration and documentation

- Schema entries (M1-07): `secrets.signing_key_dir`, `signing_key.cache_ttl`, `signing_key.rotation_check_interval`; wire `signing_key.invocation_rotation_interval`, `agent.jwks_max_age`, `auth.access_token_ttl`, `auth.clock_leeway`, `agent.invocation_token_ttl`, `webhook.delivery_timeout` where not yet wired. Cross check at load: `agent.jwks_max_age` smaller than `signing_key.invocation_rotation_interval`, else start error.
- `services/server/internal/instance/README.md` (keys section: states, windows, files, cache, job, failure modes, orphan files), `docs/DECISIONS.md`, `services/server/CHANGELOG.md` under Unreleased.

## Out of scope

- Issuing and verifying access tokens: M1-34. Invocation tokens: M2-06. Webhook signatures and `getWebhookKeys`: M2. Audit checkpoints: M1-50. Bootstrap command: M1-37 (it calls `EnsureActive`).
- Revoking all sessions after an emergency rotation of `access_token` keys: an owner step in the runbook (Fall D), not automatic.
- Writable drivers `azkv` and `awssm`: V3-12. Multi-node key store: M7-17.
- Startable installation compose and the smoke overlay: M7 and M1-73 (the overlay mounts `signing-keys`).
- Rotation intervals for purposes other than `invocation`: none exist in the specification (see Design notes).

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M1-32-signing-keys`, created from the base branch named by the launcher. One writer per worktree.
- Conventional Commits with the module as scope (for example `feat(instance): signing keys with rotation and JWKS`) and a DCO sign-off on every commit (`git commit -s`). Squash into meaningful commits before the merge.
- Never push, tag, open pull requests or create remote repositories.
- Do not modify files outside the scope of this brief, except `docs/DECISIONS.md`, `services/server/CHANGELOG.md` and the generated files named here.
- Follow the host rules supplied by the launcher. Tests bind only `127.0.0.1` and port 0 (P-10); PostgreSQL through `pgtest` (P-09), never skipped. Key store directories in tests only via `mktemp -d` with mode 0700, removed at the end.
- Stop only processes you started, by PID.
- Never read or print secrets; test keys are generated in the test and never written outside the temporary directory. No network calls except package downloads for pinned tools.
- No gate may be weakened, no threshold lowered, no test deleted or skipped.

## Design notes

- Ruling (in the ADR): writable key store, see Scope 1.
- Ruling: `kid` is the RFC 7638 thumbprint; anyone can recompute it from the public JWK, it is unique per key and safe as a file name. The examples in openapi.yaml are only examples.
- Ruling: `mode` is required in `SigningKeyRotate`.
- Ruling: emergency rotation retires every unretired key of the purpose, because a compromise of the key store would expose `next` and `retiring` keys too.
- Only `invocation` rotates automatically, because only `signing_key.invocation_rotation_interval` exists; the other purposes rotate on owner request. The job is written generically over a purpose table so that further intervals need only a configuration entry.
- Job-driven transitions write no audit entry: 05 has no system action for them, and recording `signing_key.rotate` without an owner would misstate the catalogue. They are logged. This is reported as a gap of the core contract.
- Lock order: the advisory lock is the domain step of 03 section 6.3; `audit_seq` follows in phase 3.3. Key transactions never take `permission_version`.
- No network call and no slow work inside a transaction; key generation and the file write happen before the transaction starts.
- Logs name purpose and `kid` only (SEC-LOG-1, SEC-LOG-4); private keys use the redacting secret type of M1-09 in memory.
- Problem codes: `400 validation_failed`, `401 step_up_required`, `403 forbidden` (`owner_only`, `step_up_not_possible`), `409 conflict` for a concurrent rotation that lost the race only if it cannot be serialized (the advisory lock normally prevents it), `422 idempotency_key_reuse`, `503 unavailable` when the key store is unusable at runtime.

## Tests (write them first)

Controllable clock (`kernel/clock`), PostgreSQL 18 through `pgtest`, key store in a `mktemp -d` directory with mode 0700, test authenticator for HTTP (from M1-18, or the `actortest` package of M1-31 if merged, else add it as in M1-31).

1. `TestSigningKeyTransitions_*` (domain, table): every row of the transition and window tables, boundaries exactly at the due time, no transition when not due, at most one `next` per purpose.
2. `TestThumbprint_RFC7638`: the thumbprint of the Ed25519 example key of RFC 8037 appendix A equals the value computed by hand in the test from the RFC 7638 rule.
3. `TestEnsureActive_*`: an empty database gets exactly one `active` key per purpose; a second call changes nothing; two concurrent calls from two `Keys` instances create exactly one `active` key per purpose; the private files exist with mode 0600.
4. `TestScheduledRotation_EverySignatureStaysVerifiable`: step the clock by `signing_key.rotation_check_interval` through at least two complete `invocation` rotations and one owner-requested `access_token` rotation, running the job at each step; at every step sign a sample with `Signer` and record `kid`, signing time and the purpose window; assert that every recorded sample still inside its validity verifies through `VerificationKey` and that no `retired` key verifies.
5. `TestScheduledRotation_NextPublishedBeforeUse`: a `next` key appears in `JWKS` at least `agent.jwks_max_age` before any signature uses it.
6. `TestEmergencyRotation_*`: after `mode = emergency` the old `kid` fails `VerificationKey` at once in the same process and, in a second `Keys` instance with a warm cache, at the latest after `signing_key.cache_ttl`; `next` and `retiring` keys of the purpose are `retired` too; their private files are deleted; a fresh `active` key signs.
7. `TestRotateSigningKey_HTTP`: owner with fresh step-up gets `200` with `new_kid`, `previous_kid`, `previous_not_after`; old step-up `401 step_up_required`; human without owner role `403`; owner with a personal token `403 step_up_not_possible`; anonymous `401`; purpose `audit_checkpoint` and missing `mode` `400 validation_failed`; repeat with the same `Idempotency-Key` returns the stored response; audit entry with purpose, mode, kids and reason.
8. `TestGetJwks_HTTP`: anonymous `200`, content type `application/jwk-set+json`, `Cache-Control` with `max-age` from `agent.jwks_max_age`, `Access-Control-Allow-Origin: *`; contains exactly the `next`, `active` and `retiring` keys of `access_token` and `invocation`; every JWK has exactly the members `kty`, `crv`, `x`, `kid`, `alg`, `use` and never `d`; the body validates against `Jwks`.
9. `TestSigningKeyCanary`: generate keys, run every transition and both HTTP operations, then search for the PEM body, the base64url private seed and its hex form in: `row_to_json` of every row of `server_signing_keys` and `audit_events`, all captured log records, every HTTP response body and header, and every error string returned in the test; none may match.
10. `TestSecretsPut_*` (platform): `Put` outside `secrets.signing_key_dir` is `ErrReadOnly`; `env:` refs are `ErrReadOnly`; an existing name is not overwritten; the file has mode 0600; `Delete` outside the directory is refused; the startup check fails for a missing directory, mode 0755 and a foreign owner where the test can create one.
11. `TestSecretsExport_IncludesKeyStore`: the exported bundle decrypted with the test identity contains the key files.
12. `TestStartup_SigningKeys`: start fails when the key store is not writable; `/readyz` reports `signing_keys` while a purpose lacks an `active` key and is ready after `EnsureActive`.
13. `TestKeyRotationJob_Registration`: queue `maintenance`, period `signing_key.rotation_check_interval`, roles `worker` and `all`, no arguments.
14. `TestConfig_JwksMaxAgeBelowRotationInterval`: a configuration with `agent.jwks_max_age` not below `signing_key.invocation_rotation_interval` aborts with a message naming both keys.

Commands that must pass:

- `cd services/server && go test -race ./internal/instance/... ./internal/platform/secrets/... ./internal/platform/config/... ./cmd/...`
- `make migrations-lint`, `make codegen-check`, `make arch`, `make api-lint`, `make config-doc-check`, `make docs`, `make diagrams`
- `docker compose -f infra/compose/compose.yaml config --quiet`, `make image`, `make trivy`
- `make check`

## Acceptance criteria

- [ ] Specification, ADR, configuration reference, runbook and threat model updated in the first commit; `make docs` and `make config-doc-check` green.
- [ ] Scheduled rotation: in every state every still valid signature verifies (test 4); a `next` key is published at least `agent.jwks_max_age` before use (test 5).
- [ ] Emergency rotation withdraws the old key at once in the same process and within `signing_key.cache_ttl` in others (test 6).
- [ ] `rotateSigningKey` only for owners in a session with fresh step-up, audited (test 7).
- [ ] JWKS contains only public parts and only the documented purposes and states (test 8).
- [ ] Canary: no private key in database, log, audit, response or error (test 9).
- [ ] Exactly one `active` key per purpose after start, also with concurrent starts (test 3).
- [ ] Image builds with the key store directory; compose validates.
- [ ] README, ADR, DECISIONS and CHANGELOG updated; doc comments on every exported symbol.
- [ ] `make check` green.

## Definition of done

- Code, tests and documentation as listed in Scope; the module README is current (ports, tables, actions, jobs, files).
- A doc comment on every exported symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- The ADR of Scope 1 is written and indexed (`make adr-index`); further rulings go to `docs/DECISIONS.md`.
- CHANGELOG entry under Unreleased in `services/server/CHANGELOG.md`.
- `make check` green on the final commit.
- Independent review of the diff (`make review` plus a code review by a fresh reviewer) and the red team of `docs/quality/red-team-gate.md` over the diff, with attention to PLAN section 7 Red Team point 6 (secrets in logs, telemetry, audit and errors); verdict recorded in the report. On `NEIN`: fix, add a test per finding, re-check the delta.
- Merge locally into the base branch only when the review is clean, the red team says `JA` and `make check` is green; then `git worktree remove`.

## Report

Write `{{REPORT_PATH}}`:

- first line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`;
- commits (sha and subject), files changed with line counts, test results with counts (zero skipped), the red output of the first run of each new test file, what is not proven (for example multi-node key stores), follow-ups that could not be done in scope with the reason.

Then write `{{DONE_PATH}}` with `exit=0 sha=<merge sha>` or `exit=1 <reason>`.
