# Module `privacy`

Status: privacy requests (M7-06). The deletion journal follows with M7-07.
The normative source is [docs/spec/20-privacy.md](../../../../docs/spec/20-privacy.md) sections 4.4 and 5;
where they differ, the chapter applies.

## Purpose

Privacy requests: access and portability (export), erasure (deletion) and rectification, run
through the participants of all modules.

## Code

- `internal/privacy` (`doc.go`, `requests.go`): the port `Requests` with its DTOs (the five REST
  operations).
- `domain/`: kinds and states of `gdpr_requests` with the transitions, the statutory deadline
  in calendar months (`DueAt`, `Extend`, `ReminderDue`), the progress of a request, the
  layout of the package (`Manifest`, `PackageName`, `Readme` in German and English).
- `app/`: the use cases (`Service`): `Create`, `Get`, `List`, `Decide`, `Download`, the jobs
  `RunExport`, `RunErase`, `ExpirePackages`, `Remind` and `Fail`.
- `adapters/postgres`: `Requests` (the store of `gdpr_requests`), `Retention` (the participant of
  `retention.privacy_job`) and `PersonalData` (the participant of privacy itself: the person's
  requests in `access.json`).
- `adapters/archive`: the package under construction in a temporary directory and the ZIP.
- `adapters/jobs`: the four jobs and the queue that inserts `privacy.export` and `privacy.erase`.
- `adapters/httpapi`: the REST handlers.

## Port (root package `internal/privacy`)

- Inbound: `Requests`.
- Emits in-tx events: none.
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: collects all kernel `personaldata.Participant`;
  kernel `retention.Participant` of the class `privacy_job`.

## Dependencies

May use the ports of these modules: `identity`, `files`.

Reason: data subject and active owners (`identity.Directory`); store, open and delete the export
package (`files.ExportPackages`).

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`gdpr_requests`, `deletion_journal_state` (numbering of the deletion journal, R-DM-52).

Only this module writes and reads these tables via SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`gdpr.request.create`, `gdpr.request.read`, `gdpr.request.decide`, `gdpr.export.download`.

## Events and WebSocket ops

- Durable: `gdpr.request_updated`, `gdpr.request_due`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `privacy.export`, `privacy.erase` | approval of a privacy request (`InsertTx`) | `privacy` |
| `privacy.export_expiry` | periodic every `retention.job_interval` and at the start | `maintenance` |
| `privacy.request_reminder` | periodic every `retention.job_interval` and at the start | `maintenance` |
| `retention.privacy_job` | periodic every `retention.job_interval` (M7-05 port) | `maintenance` |

The first goes with every change of a request; the second once per active owner and open
request when the deadline minus the reminder lead has passed (R-AZ-35).

## How a request runs

1. `createGdprRequest` (step-up): the person for themselves, or an owner for anybody. `due_at` is
   the receipt plus one calendar month in UTC (a day missing in the target month moves to its last
   day). One request per person and kind may be open; a second one is `409 conflict` with reason
   `request_open` and the open request in `detail` and `current`. The deletion of the last active
   owner is `409 last_owner`.
2. The export of one's own data is approved by the system (`approved_by` empty) unless
   `privacy.export_requires_approval`; everything else waits for `decideGdprRequest` of an owner
   (step-up), which may extend `due_at` once by one or two months before it expires. An approved
   rectification is completed at once; its `reason` documents the correction.
3. `privacy.export` runs every participant into the package (20 5.2), writes `README.txt` in the
   language of the person and `manifest.json` last, stores the ZIP with purpose `gdpr_export`
   and completes the request with `result_expires_at = now + privacy.export_ttl`.
   `downloadGdprExport` (step-up) serves it until then; `privacy.export_expiry` deletes it
   afterwards and the download answers `410 gone`.
4. `privacy.erase` runs steps 1 to 6 of 20 4.4 through all participants, records the last
   finished step in `progress` and resumes there after a crash; step 7 is the audit entry
   `privacy.erase`. The content mode `privacy.account_deletion_content` is fixed when the job
   starts. After `jobs.MaxAttempts` failed attempts the request is `failed`.

## Participants of the modules (kernel `personaldata`)

Each module with tables of 20 section 2 registers one participant in
`internal/wire/module_privacy.go`; `TestEveryPersonalTableHasAParticipant` holds the tables against
20 section 2. The READMEs of the modules describe their participant.

## Configuration

`privacy.export_ttl`, `privacy.export_requires_approval`, `privacy.account_deletion_content`,
`privacy.request_reminder_lead`, `retention.privacy_job` (20 section 13); the maintenance jobs
use `retention.job_interval` and `retention.batch_size`. The statutory deadline is no
configuration value (R-DP-05).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [20-privacy.md](../../../../docs/spec/20-privacy.md)
