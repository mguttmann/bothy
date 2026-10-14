# M7-22: Audit log checkpoints in object storage

Status: ready | Depends on: M1-50, M3-02 | Milestone: M7

## Goal

After this ticket `audit.checkpoint_sink` accepts, next to `file://`, an S3-compatible object store with a retention lock, the recommended target for production (19 SEC-AUD-2, `backup-restore.md` 8). A new adapter of the port `CheckpointSink` stores every signed checkpoint as an object that is written once and never overwritten, lists them back in `seq` order, and is used by `audit.chain_verify` and `audit.verify` exactly like the file sink, so a rewritten chain is detected through the checkpoints in object storage (SEC-AUD-3). A conformance suite runs the same cases against `file://` and against the object storage test service: storing, reading in `seq` order, overwriting rejected. `bothy config check` fails for an object storage target whose bucket has no effective retention lock (Object Lock enabled with a default retention in mode Compliance and a period at least `retention.audit`), because a target without the lock is not immutable. Why: the signed checkpoints are the only evidence against a complete rewrite of the hash chain by whoever administers the database; on a file system of the same host they protect little.

## Read first

- `docs/spec/19-security.md`: SEC-AUD-1 to SEC-AUD-3, 12 (backups and audit integrity), 16 "Configuration values" (`audit.checkpoint_interval`, `audit.checkpoint_sink`).
- `docs/operations/backup-restore.md` 8 (all: Compliance mode, separate account, other S3-compatible storage only with proof of the same lock strength, implementation status).
- `docs/spec/03-data-model.md` (core contract, read only): `audit_checkpoints` (`exported_at`, the export index).
- `docs/adr/0021-hash-chained-audit-log-separate-from-telemetry.md`; `docs/spec/12-files.md` (S3 driver rules that apply to every S3 client of the server); `docs/spec/20-privacy.md` 13 (`retention.audit`).
- `docs/operations/configuration.md` (rows `audit.*`, the secret catalogue).
- `docs/DECISIONS.md`: D-W-100 and its extension to M7 (D-W-125); section 10 (format of `D-M7-22-<n>` rows).
- Briefs (merged names win, D-W-100): `M1-50-audit-verify.md` (port `CheckpointSink` with `Put`, `List`, `Configured`; `filesink`; "An object storage sink with retention lock (PLAN M7-22)"), `M3-02-object-storage-s3.md` (S3 client, SeaweedFS test container, egress rules), `M1-22-egress-client.md`, `M1-07-config-schema-loader.md` (`bothy config check`).
- Code on the base branch: `services/server/internal/audit`, `internal/files/adapters` (S3 client), `internal/platform/egress`, `cmd/bothy`.

This ticket touches no table, no action and no event.

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
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; buckets in examples are `audit-checkpoints.example.com` or `bothy-audit`; no company, host or personal names.
9. Periods only through named keys; code comments and docs name the key, never the value.
10. Specification first: the new value form of `audit.checkpoint_sink` and any new key go into 19 16 and the configuration reference first. The core contract is not edited by this ticket. Everything this ticket adds is English.
11. No cost (D-W-125): tests use throwaway local S3-compatible containers only; no cloud account.

Product requirements implemented here:

12. Write once: `Put` never replaces an existing checkpoint object; a second `Put` of the same `seq` with other bytes fails with a named error, and the job stops signing (a broken sink is a loud failure, SEC-AUD-2).
13. `List` returns every checkpoint in `seq` order and verifies nothing itself; verification stays in `audit/domain`.
14. The object store target uses its own credentials and endpoint, separate from the file storage and the backup (runbook 8 item 1), through the egress client of M1-22 and the S3 client conventions of M3-02.
15. `bothy config check` with an object store target reads the bucket's Object Lock configuration and fails unless Object Lock is enabled with a default retention in mode Compliance of at least `retention.audit`; Governance mode fails (runbook 8). The server start runs the same check and refuses to start with a failing target.
16. The file sink stays unchanged and passes the same conformance suite.

## Scope

Server module (paths under `services/server/` unless they start with `docs/`; every new Go file starts with `// SPDX-License-Identifier: Apache-2.0`):

- `internal/audit/adapters/s3sink`: the adapter (object key `checkpoints/<seq zero-padded>.jws` or the form recorded in the ruling; write once through a conditional create where the service supports it, otherwise an existence check plus the lock, Design note 1), `List` with pagination, the lock check.
- SDK location: `services/server/architecture/modules.yaml` gives the module `audit` `adapter_vendors: [awssdk]` (go-arch-lint grants a vendor to all adapters of a module; regenerate `.go-arch-lint.yml` with `go generate ./tools/archgen`, as M3-02 did, ruling M3-02/12), and the depguard rule `aws-sdk-only-in-s3-driver` in `services/server/.golangci.yml` and `TestS3SDKOnlyInAdapter` in `internal/files/adapters/s3/store_test.go` allow the AWS SDK in exactly one more package, `internal/audit/adapters/s3sink`; the sink holds its own Go interface with only `PutObject` (with `If-None-Match: *`), `GetObject`, `ListObjectVersions` and `GetObjectLockConfiguration`; the core S3 rules of `docs/spec/12-files.md` 4.3 stay unchanged for the file driver, and 12 4.3 names the second package and its operations. The egress rules of the file driver apply here too: own endpoint, no default configuration loading and no credential chain of the SDK, no own `http.Client`.
- `internal/audit/app`: sink selection by scheme; the conformance suite as a shared test package run against both sinks.
- Configuration: value form `s3://<bucket>[/<prefix>]` of `audit.checkpoint_sink`; new keys for endpoint, region, path-style and the credential references (for example `audit.checkpoint_s3_endpoint`, `audit.checkpoint_s3_region`, `audit.checkpoint_s3_access_key_ref`, `audit.checkpoint_s3_secret_key_ref`), secret catalogue rows (supplied, only for `server` and `migrate` as the job runs there); `bothy config check` as in item 15.
- Docs: `docs/spec/19-security.md` 16 (value forms, new keys), `docs/spec/12-files.md` 4.3 (the second package that may import the SDK), the Confirmation sections of ADR 0010 and ADR 0048 (the same second package), `docs/operations/backup-restore.md` 8 (implementation status now built, setup with the new keys), `docs/operations/configuration.md`, the README of `audit`, `services/server/CHANGELOG.md` under `[Unreleased]`, `docs/DECISIONS.md` section 10 rows `D-M7-22-<n>`.

## Out of scope

- Creating the bucket or setting its lock (the owner does it in a separate account, runbook 8); a test against a real cloud bucket.
- Other storage kinds (Azure Blob, GCS) and the console warning banner.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M7-22-audit-checkpoints-s3`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(audit): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag, never publish an image or a release, never create remote repositories or cloud resources; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules). Tests bind only `127.0.0.1` and port 0; PostgreSQL and the S3 test service only as throwaway containers on random ports (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes and containers you started, by PID or container ID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets; tests generate throwaway credentials.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M7 by D-W-125). Where the merged code of an earlier ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Test service with a lock** (Ruling for docs/DECISIONS.md). Check in the documentation of the pinned SeaweedFS version whether it supports bucket Object Lock with a default retention in Compliance mode and conditional writes (`If-None-Match: *`). If it does, the conformance suite and the lock check run against it. If it does not, pin a second free S3-compatible test service by digest that documents both (record the source and the license; it is a test container, not shipped), and keep SeaweedFS for the negative case "no lock: `bothy config check` fails". If no free pinned service supports a Compliance lock, stop with `Verdict: BLOCKED no free S3 test service with a compliance lock` and report what you checked.

2. **Write once** (SEC-AUD-2). Only the job `audit.chain_verify` writes checkpoints, and a checkpoint is signed once, so a retry or an overlapping run writes the same bytes under the same `seq`; identical bytes under an existing key count as stored, different bytes fail. Prefer the conditional create; where the service ignores the header, the adapter checks the key first and relies on the bucket lock (with versioning an overwrite only adds a version, the locked first version stays and `List` reads the oldest version per key). A test writes a different body under an existing `seq` and expects the error and an unchanged first object.

3. **Conformance suite** (owner rule 2). One table of cases (`Put` then `List` order, `Put` duplicate rejected, `List` with more objects than one page, `Configured`), run against `filesink` and `s3sink`; `audit.verify` with a rewritten chain in the database fails in both setups.

4. **Rulings for `docs/DECISIONS.md`** (`D-M7-22-<n>`): the test service and its proof (note 1), write once (note 2), key form, the new keys.

## Tests (write them first)

- `audit/adapters/s3sink`: the conformance suite (note 3) against the test service; `TestLockCheck` (Compliance with enough period passes; Governance, too short a period and no lock fail).
- `audit/adapters/filesink`: the same conformance suite.
- Integration (`pgtest` plus the test service): `TestVerifyDetectsRewriteViaS3Checkpoints` (sign checkpoints into the object store, rewrite the chain in the database including recomputed hashes, `audit.verify` fails at the first rewritten checkpoint).
- `cmd/bothy`: `TestConfigCheck_CheckpointSinkWithoutLock` (fails and names the key) and with a lock (passes).

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/audit/... ./cmd/... ./internal/platform/...`
- `make arch`, `make config-doc-check`, `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] The conformance suite passes against `file://` and the object storage test service: storing, reading in `seq` order, overwriting rejected.
- [ ] `audit.verify` detects a rewritten chain through the checkpoints in object storage.
- [ ] A target without an effective retention lock makes `bothy config check` and the start fail.
- [ ] 19, the runbook, the configuration reference, the README, the CHANGELOG and the rulings are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `audit` updated in English.
- Rulings of Design note 4 recorded in `docs/DECISIONS.md` section 10.
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
4. The test service used, its version and the documented lock and conditional write support with sources.
5. What is not proven (a real cloud bucket in Compliance mode).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
