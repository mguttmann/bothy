# M3-15: File text extraction

Status: ready | Depends on: M3-14 | Milestone: M3

## Goal

After this ticket `search` with type `files` finds an attached document by the words inside it, not only by its name and transcript. When a file of purpose `attachment` with a supported detected type becomes deliverable, the transaction that makes it deliverable creates its row in the new table `file_text_extracts` (`state = 'pending'`, the search language of the instance) and inserts the job `files.extract_text`; the job reads the stored object once outside every transaction, runs the pure Go adapter `internal/files/adapters/extract` behind the outbound port `TextExtractor` under `search.extract.timeout`, and stores at most `search.extract.max_text_bytes` of text with `truncated`, the extractor name and version and `extracted_at`, or ends `failed` with a code without content, or `unsupported`. Supported are plain text (Markdown, CSV, JSON, source code), HTML, OOXML (DOCX, XLSX, PPTX) and ODF (ODT, ODS, ODP). Archives are opened only in memory and only within `search.extract.max_input_bytes`, `search.extract.max_uncompressed_bytes` and `search.extract.max_archive_entries`, so a zip bomb ends `failed` before its parts are inflated, and an XML part with a document type declaration, the carrier of every external entity, ends `failed` without any fetch. A periodic maintenance job re-extracts rows whose extractor version is older than the current one. The generated vector of 03 5 (stems weight C, words weight D) joins `SearchFiles`, so file text ranks below names and transcripts and passes the same rights filter (14 2, 14 4.3). This is 14 "File text extraction" and R-SU-07 without the optional Tika adapter, which PLAN 9 "Out of scope" leaves to V3.

## Read first

- `docs/spec/14-search.md`: 1 "Goals and limits" (not searchable: files that are not deliverable), 2 "What is searchable" (rows `files` with weights A to D), 3.1 "Language per object" ("File text gets the language of the instance"), 3.2 "Expressions" (`file_text_extracts`, `STORED`), 4.5 "Ranking and sorting", 4.6 "Highlighting", 5 "File text extraction" with 5.1 "Flow", 5.2 "Types and tools", 5.3 "Limits and protection", 8 "Index maintenance", 10 "Module and port" (`file_text_extracts` belongs to `files`), 12 "Tests" (row "Office file with a zip bomb, XML with an external entity"), 13 R-SU-07, 15 "Configuration values of this chapter" (`search.extract.*`).
- `docs/spec/12-files.md`: 2 "States of a file" (definitions "servable" and "final"; which changes write `file.updated`), 3.4 "Completion and verification pass" (last paragraph: extraction follows a deliverable file), 3.7 "Purpose of a file", 4.1 "Port" (`Get` with range, `ErrNotFound`), 6.1 "Type" (detected `mime_type`), 10 "Attaching and agent invocation", 12 "Retention, deletion, deduplication", 14 "Observability".
- `docs/spec/03-data-model.md` (core contract, read only): 1 "Principles" items 2, 12 and 13; 4.7 tables `files`, `message_attachments` (with the copies `thread_id` and `channel_id` of R-DM-43) and `file_text_extracts`; 5 "Search vectors" (row `file_text_extracts`); 6.3 "Lock order".
- `docs/spec/02-architecture.md`: 4.2 (row `files`), 4.3 (row `files`: port `TextExtracts`, see Design note 3), 4.4 (row `files` may use `identity`), 5 "Layers within a module", 6 "Code layout" (test helpers `internal/<module>/adapters/...test`), 7.2 "go-arch-lint" (vendors per adapter), 8.5 "What never happens in a transaction", 12.1 "Usage" (job arguments only IDs), 12.2 "Queues" (`files`, `maintenance`), 12.3 "Job catalogue" (row `files.extract_text`), 12.4 "Rules for jobs" (rules 1, 2, 5 and 6), 13 "Configuration loading" item 5.
- `docs/spec/19-security.md`: 7 "Upload hardening" SEC-UP-5 and SEC-UP-7 (Design note 6 amends both), 9 "Logs without secrets and without content" (SEC-LOG-4), 14 "Tests" SEC-T08 and SEC-T10. `docs/spec/20-privacy.md`: row `file_text_extracts` (class `content`, "with the file") and DP-T01.
- ADRs: 0007 (sqlc and goose), 0008 and 0050 (module register, vendor groups), 0010 (object store), 0032 (search in PostgreSQL), 0033 (only for the scan states this ticket builds on).
- `docs/DECISIONS.md`: D-K-G15 (`file_text_extracts` taken into the core contract), D-SU-04 (R-SU-07), D-FI-04, D-W-100, D-W-104 (R-DM-43), section 10 (format of `D-M3-15-<n>`).
- `docs/security/threat-model.md` 5.8 "Files, object storage, virus scanner" (rows D and E) and T-UP-01; `docs/RISKS.md` RK-38 (context only: a vector expression change rewrites a table).
- `docs/observability/signals.md`: 2 "Cardinality", 4.4 (`bothy.job.duration`, `bothy.job.queue` with `bothy.job.kind`), 5.2 "Spans" (`job <bothy.job.kind>`), 5.3 "Attributes in spans and logs" (`bothy.file.id`, `bothy.reason`).
- `docs/operations/configuration.md`: section 4 "14 Search" (`search.extract.max_input_bytes`, `search.extract.max_text_bytes`, `search.extract.max_uncompressed_bytes`, `search.extract.max_archive_entries`, `search.extract.timeout`, `search.extract.tika_url`), `jobs.queue.files.max_workers`, `jobs.queue.maintenance.max_workers`, `database.statement_timeout`, `file.scan.mode`. `docs/operations/upgrade.md` 4 "Rules for migrations (Expand/Contract)".
- `docs/PLAN.md`: 3 "Way of working per ticket"; 5 "Test strategy" (row "Security": Go fuzzing of the parsers); 9 "M3 Platform services" (Goal, Scope, Out of scope "Tika (V3)", Exit criteria 1, 2 and 4, Test strategy "decompression bombs and archive limits", Red team T-UP-01, Risks) and the rows M3-03, M3-04, M3-07, M3-13, M3-14 (before), M3-06, M3-08 (beside), M3-16 (after). No ticket of the plan depends on M3-15.
- Briefs: `docs/briefs/M3-04-clamd-scan.md` (the outcome write of its note 6 and the place its note 13 reserves for `files.extract_text`, `domain.Deliverable`, `adapters/jobs`, `HandlerOptions.Timeout`, `clamdtest`), `docs/briefs/M3-14-search-coordinator.md` (`SearchFiles`, `searchkit.Query`, `searchkit.Page`, its note 6 on files and note 15 on attachment facts), `docs/briefs/M3-13-search-vectors.md` (`identity.Directory.InstanceSearchLanguage`, `kernel/searchlang`, the text-and-cast rule of its note 7), `docs/briefs/M3-03-upload-sessions.md` (migration and grant pattern, `app/events.go`), `docs/briefs/M3-01-object-storage-fs.md` (`ObjectStore`, `ErrNotFound`), `docs/briefs/M3-06-image-metadata-thumbnails.md` (only for the boundary: images become deliverable in `files.thumbnail`), M1 briefs for the interfaces used: M1-07 (`KindInt`, `KindDuration`, `Min`), M1-08 (`tools/configdoc/planned.yaml`), M1-12 (migrations, grants of D4, `pgtest`), M1-13 (`txn.UnitOfWork`), M1-14 (`internal/platform/jobs`: `Args`, `Register`, `HandlerOptions`, `PeriodicJob`, `Inserter.InsertTx`, the argument and kind checks), M1-70 (`tools/lint/sql_visibility.yaml`, category `system_job`), M1-71 (`test/harness`, `BuildMatrixWorld`).
- Dependencies: the header names M3-14 (D-W-106); M3-04 comes through M3-14 and M3-07. M3-04 implies M3-03, M3-01, M1-55 and their chain; M3-14 implies M3-13 and M1-56. This brief also builds on what M3-14 itself needs, which its header names or implies (D-W-106): M3-07 (`message_attachments`, `MessageCreate.attachment_file_ids`), M1-70 (SQL visibility lint) and M1-71 (harness). Design note 0 checks them.
- Code (paths under `services/server/` unless they start with `docs/` or `tools/`): `internal/files` (port, `domain`, `app`, `adapters/postgres` with `queries/search.sql`, `adapters/jobs`, `adapters/fs`, `adapters/sniff`), `internal/identity` (port `Directory`), `internal/kernel/searchlang`, `internal/kernel/txn`, `internal/platform/jobs`, `internal/platform/pg/pgtest`, `internal/wire` (`configkeys_search.go`, `configchapters.go`, `configchecks.go`), `migrations/`, `sqlc.yaml`, `architecture/modules.yaml`, `test/harness`, `tools/lint/sql_visibility.yaml`, `tools/configdoc/planned.yaml`.

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

11. Table `file_text_extracts` exactly as 03 4.7, with the generated column of 03 5 written `STORED`, owned and read only by `files` (03 1 item 12, 14 10, 14 3.2).
12. When a file becomes deliverable (12 2), has a supported detected type and purpose `attachment`, the same transaction inserts its row with `state = 'pending'` and the job `files.extract_text` in queue `files` with `InsertTx`; the job arguments hold only the file id (14 5.1 item 1, 02 12.1, 12.3).
13. The job calls the port `TextExtractor` (`Extract(ctx, reader, mime)` giving text and `truncated`) and ends `done` with at most `search.extract.max_text_bytes` of text, `truncated`, `extractor` (name and version) and `extracted_at`; `failed` with a code without content in `error`; or `unsupported` (14 5.1 items 2 and 3, 03 `file_text_extracts`).
14. Types and tools exactly as 14 5.2 without the row `tika`: text formats `builtin` (valid UTF-8, invalid bytes replaced), `text/html` with `golang.org/x/net/html` (text nodes only, scripts and styles dropped), DOCX, XLSX and PPTX with `archive/zip` and `encoding/xml` (document, shared strings, slides), ODT, ODS and ODP (`content.xml`); pure Go, no cgo, no external process, no network (R-SU-07).
15. Limits (14 5.3): only files up to `search.extract.max_input_bytes`; archives at most `search.extract.max_uncompressed_bytes` inflated and at most `search.extract.max_archive_entries` entries, otherwise `failed`; at most `search.extract.timeout` per file.
16. XML only through `encoding/xml` and never with an external entity loaded; an Office file with a zip bomb and XML with an external entity end `failed`, and nothing is fetched (14 5.3, 14 12).
17. A rise of an extractor's version re-extracts the older rows through a maintenance job (14 5.1 item 4).
18. `search_language` of a row is the language of the instance (14 3.1, 03).
19. File text is content of class `content`: never in logs, spans, metrics, job arguments, events, audit details or problem bodies, and deleted with the file (14 5.3, 20, SEC-LOG-4, 02 12.1).
20. `SearchFiles` matches `file_text_extracts.search_vector` (weight C for stems, D for words) next to name (A) and transcript (B), under the same rights filter, the same deliverable and attached rules and the same candidate limit (14 2, 4.3, 4.5, 10).
21. Parsers of untrusted formats run only in the worker's jobs, never in a request of the API process; the server never writes archive entries to storage and never opens nested archives (SEC-UP-5, SEC-UP-7 as amended in Design note 6).
22. No object store call inside a database transaction (02 8.5); every job run repeats without double effect (02 12.4 rule 1).

## Scope

All paths under `services/server/` unless they start with `docs/` or `tools/`. Where M3-04 or M3-14 already created a file named below, extend it.

Create:

- `migrations/<next>_files_text_extracts.sql` (Design note 1) and its line in `migrations.sha256` through `make migrations-lock`.
- `internal/files/domain/extract.go` (standard library and `kernel-core` only): the states, the closed set of failure codes of Design note 4, `ExtractApplies` (Design note 2), `RefreshDecision` (Design note 9) and `HalveText` (Design note 8). `internal/files/domain/textbuf.go`: the bounded `TextBuilder` of Design note 5.
- `internal/files/app/extract.go`: the outbound port `TextExtractor` and `ExtractResult` (Design note 3), the outbound port `ExtractQueue`, the schedule function of Design note 2, the use case `ExtractText.Run(ctx, fileID ids.ID) error` (Design note 4) and the repository methods it needs. `internal/files/app/extract_refresh.go`: the use case `ExtractRefresh.Run(ctx, after ids.ID) error` (Design note 9).
- `internal/files/adapters/extract/`: `doc.go`, `registry.go` (`Registry` implements `app.TextExtractor`; `Config{MaxTextBytes, MaxUncompressedBytes int64; MaxArchiveEntries int}`), `text.go`, `html.go`, `zipread.go`, `xmltext.go`, `ooxml.go`, `odf.go`, tests, fuzz targets with seed corpora, `testdata/golden/` (Design note 3), and `extracttest/` (fixture builders of Design note 14).
- `internal/files/adapters/jobs/extract.go`: args `ExtractText struct { FileID ids.ID }` with `Kind() = "files.extract_text"` and `ExtractRefresh struct { After ids.ID }` with `Kind() = "files.extract_text_refresh"`, their handlers, and the `ExtractQueue` implementation over `jobs.Inserter`.
- `internal/files/adapters/postgres/queries/text_extracts.sql`: the queries of Design notes 2, 4, 8, 9 and 10; regenerated sqlc code.

Change:

- The outcome write of M3-04 (one function in `internal/files/app`, M3-04 note 13): call the schedule function of Design note 2 for a deliverable outcome inside its unit of work. Do the same in every other place where merged code makes a file deliverable (Design note 2).
- Every merged code path of `files` that sets `files.deleted_at`: delete the file's extract row in the same unit of work (Design note 10).
- `internal/files/adapters/postgres/queries/search.sql` and the `SearchFiles` read operation of M3-14: the file text branch, the combined rank and the headline input of Design note 11.
- `internal/wire`: in `configkeys_search.go` (M3-13, M3-14) the keys of Design note 12 with `chapter14`; the expected keys in `TestConfigSchemaBuilds`; construction of the `extract.Registry` from the configuration; `identity.Directory` for the files app if not yet passed; registration of `files.extract_text` (queue `files`, no River job timeout) and the periodic `files.extract_text_refresh` (queue `maintenance`, `Interval = search.extract.refresh_interval`, `RunOnStart = true`) for the roles `worker` and `all`; regenerate `testdata/txn_registrations.golden` only if a registration changes.
- `internal/platform/jobs`: no change; after M3-04 `HandlerOptions` has the per-kind timeout that M3-04 note 7 describes (zero: River default; negative: none), with its test and README line.
- `architecture/modules.yaml`: a vendor group for `golang.org/x/net/html` (`golang.org/x/net/html`, `golang.org/x/net/html/**`; reuse a group of the same import path if one exists) in the `adapter_vendors` of `files`; regenerate `.go-arch-lint.yml` with `cd services/server && go generate ./tools/archgen`.
- `go.mod`, `go.sum`: `golang.org/x/net` as a direct dependency at a released version (keep the version already in `go.sum` if one is there and it has the `html` package); BSD-3-Clause is on the allowed list (`make licenses`).
- `sqlc.yaml`: the new query file in the `files` entry, if the entry lists files.
- `tools/lint/sql_visibility.yaml` (M1-70): one exception with category `system_job` and a reason for each new read of the jobs (Design note 13); the search query needs none.
- `tools/configdoc/planned.yaml` (M1-08): an exact rule `search.extract.tika_url` with milestone `V3`; remove every rule that becomes stale.
- `docs/spec/14-search.md` (not core contract): 5.1 (trigger only for purpose `attachment`, `unsupported` at run time, the refresh job with its staleness rule and its keys, deletion with `files.deleted_at`), 5.2 (the type list of Design note 3, the parts read per format, adapter names with versions, `tika` built in V3), 5.3 (the archive and XML rules of Design notes 6 and 7, memory bound, the vector limit of Design note 8), 4.6 or 10 (headline input and configuration for files, Design note 11), 15 (rows `search.extract.refresh_interval` and `search.extract.refresh_batch_size`).
- `docs/spec/02-architecture.md`: 4.3 row `files` without the port `TextExtracts` (Design note 3); 12.3 row `files.extract_text` with trigger "after `clean` or `skipped`, purpose `attachment`, supported type" and a new row `files.extract_text_refresh` (`files`, periodic every `search.extract.refresh_interval`, `maintenance`, 14).
- `docs/spec/12-files.md` 3.4 last paragraph: the extraction is the job `files.extract_text` for attachments, with a link to 14 5.
- `docs/spec/19-security.md`: SEC-UP-5 and SEC-UP-7 as Design note 6 words them.
- `docs/operations/configuration.md` regenerated with `make config-reference`.
- `internal/files/README.md` (English, like the other module READMEs): jobs, the outbound port `TextExtractor`, the adapter `extract`, the table, the memory bound, the deletion rule, the residuals; `services/server/CHANGELOG.md` under "Unreleased"; `docs/DECISIONS.md` section 10 rows `D-M3-15-<n>` (Design note 15).

This ticket enables no operation, adds no action, event, audit entry, metric, span name and no top-level Makefile target.

## Out of scope

- The Tika adapter, `search.extract.tika_url` in the schema, PDF, RTF, EPUB and the older Office formats (PLAN 9 "Out of scope": V3).
- The search coordinator, the rights filter, filters, cursor, highlighting segments and `searchThreadTitles` (M3-14); this ticket changes only the SQL of `SearchFiles`.
- Upload, scan, type detection and the outcome states (M3-03, M3-04); images and `files.thumbnail` (M3-06); attachments, `message_attachments` and `FileFinalized` (M3-07); the deletion and purge jobs `files.cleanup_pending` and `files.purge_objects` (M3-08), except the extract row deletion of Design note 10.
- Transcripts (M3-09); avatars and logos (M3-10): both are never extracted.
- The offline search of the client, which holds no file text (14 6, M4-23); the search load test (M3-16).
- Reassigning `search_language` of existing rows after a change of `locale.default` (14 8 "Reassigning the language"): no ticket of the plan owns it (M3-13 reported it).
- Export and erasure of file text for a person (`personaldata.Participant`, M7-06); the threat model update for the milestone (PLAN 4 item 5).
- A metric or span for extraction: `signals.md` names none; the job wrapper of M1-14 and the job metrics of M1-68 cover the job.
- Changes to the core contract.

## Boundaries

- A new migration needs its line in `migrations.sha256`: run `make migrations-lock` after creating
  the file and before `make check`; never edit a merged migration (M1-01, ADR 0007).
- Work in a dedicated git worktree on the branch `feat/M3-15-file-text-extraction`; one writer per worktree.
- Conventional Commits with the module as scope (for example `feat(files): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`.
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` and port 0; PostgreSQL runs only as a throwaway container on a random port through `pgtest` (P-09, P-10). Without a Docker-compatible runtime the integration tests fail; they never skip.
- Stop only processes you started, by PID. Temporary directories only via `mktemp -d`, removed at the end.
- Never read or print secrets. No network calls except package downloads for pinned tools and Go modules.
- Never lower a threshold, disable a gate or delete a test.
- If the core contract lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100). Where the merged code of an M1 or M2 ticket names a package, type, function, file or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.
- The same rule applies to the merged code of M3-01, M3-03, M3-04, M3-07, M3-13 and M3-14, whose briefs were written together with this one: follow their names (outcome write, repositories, `SearchFiles`, job adapter package, test helpers) and list each difference.
- No binary fixture in the repository: every Office, ODF, zip bomb and XML test file is built at test time from Go code (Design note 14); fuzz seed corpora are the text files `go test` writes.
- The only network listener in the tests besides PostgreSQL is the loopback HTTP server of the XXE test, started and stopped by that test; it proves that nothing calls it. No Tika container, no real third-party service. Files reach their scan state through `file.scan.mode = off` or the fake clamd of M3-04 (`clamdtest`).
- No SQL of `files` names a table of another module (03 1 item 12, 02 7.3); the instance language comes only through `identity.Directory`.
- The extraction never reads a file outside its job and never reads more than `search.extract.max_input_bytes` of an object; no extracted text, no part name of an archive and no XML content in any log line, span attribute, metric attribute, job argument or error message.

## Design notes

0. Start check. `make check` green on the fresh worktree, else `Verdict: BLOCKED base branch red`. Merged and present, else `Verdict: BLOCKED dependency <id> missing`: M3-04 (job `files.scan` and its outcome write), M3-14 (`SearchFiles` in `internal/files` and the operation `search` enabled), M3-13 (`identity.Directory.InstanceSearchLanguage`), M3-07 (table `message_attachments`, `MessageCreate.attachment_file_ids` enabled), M1-70 (`tools/lint/sql_visibility.py`), M1-71 (`test/harness`).

1. Migration `<next>_files_text_extracts.sql` (numbering and slug by the rules of `make migrations-lint`), one transaction; the table is new and empty, so its indexes are built with it (as M3-03 did for `files`); if the merged lint demands `CONCURRENTLY` for every index, move the two indexes into a second migration with `-- +goose NO TRANSACTION` as M3-13 did. Columns, defaults and the CHECK verbatim from 03 4.7, the expression verbatim from 03 5; constraint and index names follow the merged convention (`grep -n 'CREATE INDEX\|CONSTRAINT' migrations/*.sql`). Add no column, CHECK or index that 03 does not name.

    ```sql
    -- +goose Up
    CREATE TABLE file_text_extracts (
      file_id uuid NOT NULL,
      state text NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending','done','failed','unsupported')),
      extractor text,
      text_content text,
      truncated boolean NOT NULL DEFAULT false,
      search_language regconfig NOT NULL DEFAULT 'simple',
      extracted_at timestamptz,
      error text,
      search_vector tsvector NOT NULL GENERATED ALWAYS AS (
        setweight(to_tsvector(search_language, coalesce(text_content, '')), 'C') ||
        setweight(to_tsvector('simple'::regconfig, coalesce(text_content, '')), 'D')
      ) STORED,
      PRIMARY KEY (file_id),
      FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
    );
    CREATE INDEX file_text_extracts_search_vector_idx ON file_text_extracts USING gin (search_vector);
    CREATE INDEX file_text_extracts_pending_idx ON file_text_extracts (state) WHERE state = 'pending';
    GRANT SELECT, INSERT, UPDATE, DELETE ON file_text_extracts TO bothy_app;
    -- +goose Down
    DROP TABLE file_text_extracts;
    ```

    The grant follows M1-12 D4 (no default privileges). `search_language` goes to SQL as text with a cast (`sqlc.arg(search_language)::text::regconfig`) and no query selects `search_vector` (M3-13 note 7), so pgx needs no codec for `regconfig` or `tsvector`. Retention class `content` goes into the module README.

2. Trigger (14 5.1 item 1; Ruling for docs/DECISIONS.md for purpose, row creation and language).
    - `domain.ExtractApplies(purpose string, deliverable, supported bool) bool` is true exactly for `purpose = 'attachment'`, a deliverable file (`domain.Deliverable` of M3-04, 12 2: `upload_state = 'complete'`, `scan_status IN ('clean','skipped')`, `deleted_at IS NULL`) and `TextExtractor.Version(mime_type)` reporting support. Only attachments are searchable (14 2, M3-14 note 6); the text of an avatar, a logo or a GDPR export package would be a copy of content no search can reach.
    - The schedule function in `internal/files/app` (`scheduleTextExtract(ctx, fileID, purpose, mimeType)`, name free) runs inside the caller's unit of work: when `ExtractApplies` holds, it reads `identity.Directory.InstanceSearchLanguage(ctx)` (M3-13; the same unit), runs `INSERT INTO file_text_extracts (file_id, search_language) VALUES (...) ON CONFLICT (file_id) DO NOTHING RETURNING file_id`, and only when a row was inserted calls `ExtractQueue.EnqueueExtract(ctx, fileID)` (`InsertTx` of `files.extract_text`). An unsupported type gets no row (14 5.1 item 1); `unsupported` is written only by the job (note 4b).
    - Callers: the outcome write of M3-04 for `clean` and `skipped`, after the state update, in the same unit of work (M3-04 note 13). If the merged code makes a file deliverable anywhere else (M3-06 sets `upload_state = 'complete'` for image types at the end of `files.thumbnail`), call it there too; it is a no-op for unsupported types and stays correct when a later extractor version supports more types. `infected`, `error`, `failed` and `pending` never schedule.
    - The language is taken once, when the row is created; re-extraction keeps it (14 3.1: a later change of `locale.default` affects only new rows, as for messages).
    - Lock order (03 6.3): the outcome write holds the `files` row; the insert locks only the new extract row (and takes the foreign key's share lock on the `files` row it already holds).

3. Port, adapters, types and versions (14 5.1 item 2, 5.2; Ruling for docs/DECISIONS.md for identifiers, the type list and the golden rule).

    ```go
    // TextExtractor is the outbound port of 14 5.1; internal/files/adapters/extract implements it.
    type TextExtractor interface {
        // Version names the adapter and version that extracts mimeType, for example
        // "builtin-ooxml/1"; ok is false when no adapter supports the type.
        Version(mimeType string) (version string, ok bool)
        // Extract reads r and returns at most search.extract.max_text_bytes of text. Content
        // problems return *domain.ExtractError with a code of note 4; errors of r come back
        // wrapped and unchanged; a done ctx returns ctx.Err().
        Extract(ctx context.Context, r io.Reader, mimeType string) (ExtractResult, error)
    }
    type ExtractResult struct { Text string; Truncated bool; Version string }
    ```

    - Adapters and types (the detected `mime_type` without parameters, M3-04 note 4f): `builtin-text` for every `text/*` type except `text/html` and `text/rtf`, plus `application/json` (14 5.2: `text/plain`, `text/markdown`, `text/csv`, `application/json`, source code; source code is what the detector reports under `text/`, for example `text/x-python`; `text/rtf` is in the `tika` row); `builtin-html` for `text/html`; `builtin-ooxml` for `application/vnd.openxmlformats-officedocument.wordprocessingml.document`, `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` and `application/vnd.openxmlformats-officedocument.presentationml.presentation`; `builtin-odf` for `application/vnd.oasis.opendocument.text`, `application/vnd.oasis.opendocument.spreadsheet` and `application/vnd.oasis.opendocument.presentation`. Every other type is unsupported. Verify each OOXML and ODF name against what the merged detector (`adapters/sniff`, `github.com/gabriel-vasile/mimetype`) returns for the fixtures of note 14 and use the detector's names.
    - The value stored in `extractor` is `<adapter>/<n>` with an integer `n` per adapter, a constant in code (03: "name and version of the extractor").
    - Golden rule: `testdata/golden/<adapter>.txt` holds the version and the SHA-256 of the adapter's output over a fixed fixture set of note 14. `TestExtractorGolden` fails when the output hash differs from the golden; its `-update` flag refuses to write a new hash under an unchanged version and asks to raise the version. So every change of an adapter's output raises its version, and the refresh job of note 9 re-extracts what that adapter extracted before.
    - No inbound port `TextExtracts` is built: no other module reads file text (search reads through `SearchFiles`, 14 10), and a port without a consumer would be dead code (M1-05: interfaces enter with their implementing ticket). Remove it from 02 4.3 row `files` and from the module README. Ruling for docs/DECISIONS.md.

4. One run of `files.extract_text` (`ExtractText.Run`; Ruling for docs/DECISIONS.md for the codes and the error rule). The closed set of codes in `error` (03: "code without contents"; constants in `domain`): `input_too_large`, `archive_invalid`, `archive_entries_exceeded`, `archive_size_exceeded`, `xml_doctype`, `xml_invalid`, `timeout`, `object_missing`.

    a. Read without a transaction the row and the file facts (`purpose`, `mime_type`, `size_bytes`, `object_key`, `upload_state`, `scan_status`, `deleted_at`). No row, or a row that is not `pending`: return `nil` (a duplicate job, 02 12.4 rule 1). File deleted or not deliverable: delete the row in a short unit of work (note 10) and return `nil`.

    b. `TextExtractor.Version(mime_type)` without support: write `unsupported` (`extractor`, `text_content` and `error` NULL, `truncated` false, `extracted_at` now).

    c. `size_bytes` above `search.extract.max_input_bytes`: write `failed` with `input_too_large` and the version, without reading the object.

    d. Outside every unit of work (02 8.5): a context with the deadline `search.extract.timeout` derived from the job context; `ObjectStore.Get(ctx, object_key, nil)`; `Extract` over the body wrapped in a reader limited to `search.extract.max_input_bytes` plus one byte (more bytes give `input_too_large`); close the body. Mapping: `*domain.ExtractError` gives `failed` with its code; the extraction deadline gives `failed` with `timeout`; `ErrNotFound` from the object store gives `failed` with `object_missing` and one Error log (a deliverable file must have its object); a cancelled job context (shutdown) returns `ctx.Err()` (M1-14); any other error of the object store or the reader is transient: return it to River, the row stays `pending`, River's default retry applies (02 12.4 rules 4 and 5), and note 9 re-queues a row whose job River discarded.

    e. Write the result (note 8). Time comes from `kernel/clock` (`extracted_at`), never SQL `now()`.

    Register `files.extract_text` in queue `files` with no River job timeout (`HandlerOptions.Timeout` negative): the use case enforces `search.extract.timeout` itself on read and parse, and River's default job timeout could cut the result write (M3-04 note 7); database work stays bounded by `database.statement_timeout`.

5. Plain text and HTML (14 5.2 rows 1 and 2).
    - `domain.TextBuilder` (pure) collects text up to `search.extract.max_text_bytes` bytes: invalid UTF-8 becomes U+FFFD, NUL and every other C0 control character except tab, line feed and carriage return become a space (PostgreSQL `text` rejects NUL), separators are written once (no run of separators), a cut never splits a rune, `truncated` is set when anything was dropped, and `Full()` tells the adapter to stop reading.
    - `builtin-text` streams the reader into the builder and stops at `Full()`: a UTF-8 byte order mark is removed; a UTF-16 byte order mark (little or big endian) switches to UTF-16 decoding with `unicode/utf16`; everything else is read as UTF-8. No charset guessing.
    - `builtin-html` uses the tokenizer of `golang.org/x/net/html`: only text tokens; everything inside `script` and `style` is dropped; comments and doctype tokens are dropped; entities are decoded by the tokenizer; a start or end tag of a block element (`address`, `article`, `aside`, `blockquote`, `br`, `dd`, `div`, `dl`, `dt`, `fieldset`, `figcaption`, `figure`, `footer`, `form`, `h1` to `h6`, `header`, `hr`, `li`, `main`, `nav`, `ol`, `p`, `pre`, `section`, `table`, `tbody`, `td`, `tfoot`, `th`, `thead`, `title`, `tr`, `ul`) writes a line feed, inline elements write nothing, so `Rech<b>nung</b>` stays one word. No charset conversion: the bytes are read as UTF-8 through the builder. The tokenizer holds at most the input, which `search.extract.max_input_bytes` bounds.

6. Archives (14 5.3; Ruling for docs/DECISIONS.md for every rule here). `archive/zip` needs random access, and ranged reads through the object store would cost one request per small read, so the OOXML and ODF adapters read the whole object into memory (at most `search.extract.max_input_bytes`, enforced by note 4c and 4d) and open it with `zip.NewReader` over a `bytes.Reader`.
    - Any error of `zip.NewReader`, including the insecure path error, gives `archive_invalid`.
    - More entries in the central directory than `search.extract.max_archive_entries` gives `archive_entries_exceeded` before any entry is opened.
    - Only the fixed parts of note 7 are read, matched by exact name; a part name that occurs twice gives `archive_invalid` (an ambiguous archive may be read differently by other tools). Embedded objects and nested archives are never opened, and nothing is written to the file system.
    - Before inflating, the sum of the declared uncompressed sizes of the parts this format reads is compared with `search.extract.max_uncompressed_bytes`; above it gives `archive_size_exceeded` without inflating a byte. While reading, a counter shared by all parts of one file counts the bytes actually inflated and gives `archive_size_exceeded` when it passes the limit; a part that inflates beyond its declared size fails in `archive/zip` and gives `archive_invalid`, so a lying header never inflates more than it declared.
    - An entry with the encryption flag or a compression method other than store and deflate gives `archive_invalid`.
    - Peak memory of one extraction is about `search.extract.max_input_bytes` plus the run guard of note 7; the README names it together with the parallelism `jobs.queue.files.max_workers`.
    - 19 changes (English, same change): SEC-UP-5 names the text extraction of 14 5 (linked) beside the image previews as pure Go parsers that run only in the worker's jobs with a time limit, never in a request of the API process; SEC-UP-7 reads "Archives are never unpacked to storage in the server and never opened nested. The only exception: the text extraction reads single XML parts from OOXML and ODF containers in the memory of the worker, within the limits `search.extract.*`." followed by a link to 14 5.3. Reason: 14 5.2 prescribes `archive/zip` for Office files, which contradicts the unconditional SEC-UP-7 today.

7. XML and the parts per format (14 5.2, 5.3; Ruling for docs/DECISIONS.md).
    - Decoder: `encoding/xml` with `Strict = true`, `Entity` nil (only the predefined entities) and `CharsetReader` nil, so a part declaring an encoding other than UTF-8 gives `xml_invalid`. The first `xml.Directive` token that is a document type declaration gives `xml_doctype` at once: OOXML and ODF parts never need one, and it carries every external entity, parameter entity and internal entity expansion. An undefined entity reference or any other syntax error gives `xml_invalid`. `encoding/xml` has no way to fetch anything; the adapter package imports no network package (test below).
    - Run guard: a reader between the part and the decoder fails the part when more than `search.extract.max_text_bytes` bytes pass without a `<`, because `encoding/xml` buffers a whole text, comment or attribute token before returning it. When the guard trips, the part ends, the text collected so far is kept and `truncated` is set: that run alone exceeds the text limit. So a part never holds more than about the text limit in one token.
    - The context is checked at every token; a done context ends with `ctx.Err()`.
    - Elements match by local name only (both the transitional and the strict OOXML namespaces work).
    - DOCX: `word/document.xml` (missing gives `archive_invalid`); text of `t` elements; `tab` writes a tab, `br` and `cr` a line feed, the end of `p` a line feed. `delText` and `instrText` are not `t`, so deleted revisions and field codes stay out.
    - XLSX: `xl/sharedStrings.xml` (missing gives `done` with empty text: a workbook may hold only numbers); text of `t` elements outside `rPh`; a line feed after each `si`.
    - PPTX: every part named `ppt/slides/slide<N>.xml` with a decimal `N`, in ascending numeric order (`slide2` before `slide10`); text of `t` elements; a line feed after each `p` and between slides.
    - ODF: `content.xml` (missing gives `archive_invalid`); inside `body`, all character data; `s` writes a space, `tab` a tab, `line-break` and the end of `p` and `h` a line feed; the subtrees `annotation` and `tracked-changes` are skipped (comments and deleted revisions).
    - Fixed part names instead of resolving package relationships keep the parser surface small; a document whose producer uses other part names gives `archive_invalid` or empty text, never a wrong text. Record the reason.

8. Writing the result (one unit of work per write).
    - `done`: `UPDATE file_text_extracts e SET state = 'done', text_content = $text, truncated = $t, extractor = $v, extracted_at = $now, error = NULL FROM files f WHERE e.file_id = $id AND e.state = 'pending' AND f.id = e.file_id AND f.deleted_at IS NULL`; the empty string is stored as `''`, not NULL. `failed`: the same with `text_content = NULL`, `truncated = false`, `error = $code`, `extractor = $v`. `unsupported`: `extractor`, `text_content` and `error` NULL. No row updated (deleted file, row gone, or another run was first) is no error.
    - Vector limit (Ruling for docs/DECISIONS.md): PostgreSQL rejects a `tsvector` whose lexemes exceed its size limit with SQLSTATE `54000`, which a text of `search.extract.max_text_bytes` with many distinct tokens (identifiers, hashes in CSV or JSON) can reach. The postgres adapter maps `54000` on this update to `app.ErrVectorTooLarge`; the use case then cuts the text to half its bytes at a rune boundary (`domain.HalveText`), sets `truncated` and writes again in a new unit of work, until the write succeeds (an empty text always does). Never retry the same text.
    - No `file.updated`: 12 2 names the changes that write it, and extraction is not among them; `File` has no extraction field. No audit entry, no in-transaction event.
    - Logs: one Warn for `failed` with `bothy.file.id` and `bothy.reason` equal to the code, one Error for `object_missing`, nothing for `done` and `unsupported` beyond the job wrapper of M1-14. Never the text, a part name, a file name or XML content.

9. Re-extraction (`files.extract_text_refresh`, 14 5.1 item 4; Ruling for docs/DECISIONS.md for the job, both keys, the staleness rule, keeping text during a refresh and recovering discarded jobs).
    - Periodic in queue `maintenance` with `Interval = search.extract.refresh_interval` and `RunOnStart = true` (a new binary brings new versions; River enqueues periodic jobs on the leader only). Args `ExtractRefresh{After ids.ID}`; the periodic run starts at the zero id.
    - One run, in one unit of work: read the next `search.extract.refresh_batch_size` deliverable files with `purpose = 'attachment'` and `id > After`, ordered by `id`, each with its extract row if any (`state`, `extractor`, `error`; a `LEFT JOIN`), and decide each with the pure `domain.RefreshDecision(row, current string, supported bool, maxInputBytes int64)`:

        | Row | Decision |
        |---|---|
        | none, type supported | create the row as note 2 does and enqueue `files.extract_text` |
        | none, type unsupported | nothing |
        | `pending` | enqueue `files.extract_text` again (recovers a job River discarded after its attempts; a duplicate finds the row no longer `pending`) |
        | `done`, `failed` or `unsupported`, type supported, `extractor` differs from the current version (NULL differs from every version) | reset to `pending` and enqueue |
        | `failed` with `input_too_large`, type supported, `size_bytes` now within `search.extract.max_input_bytes` | reset to `pending` and enqueue |
        | any other | nothing |

    - Reset is a compare-and-set: `UPDATE ... SET state = 'pending' WHERE file_id = $id AND state = $seen_state AND extractor IS NOT DISTINCT FROM $seen_extractor`. It keeps `text_content`, `truncated`, `extractor` and `extracted_at`, so the file stays findable by its old text until the job writes the new result (note 11 reads the vector of any state).
    - In the same unit, delete up to `search.extract.refresh_batch_size` extract rows whose file has `deleted_at` set (catch-up for note 10).
    - When the batch was full, insert the next `ExtractRefresh{After: <last file id>}` with `InsertTx` in the same unit; otherwise the walk ends.

10. Deletion (14 5.3, 20, DP-T01; Ruling for docs/DECISIONS.md). The text goes when the file goes, not only when the row is purged: every merged code path of `files` that sets `files.deleted_at` (message deletion with attachments through M3-07, `deleteFile` and the cleanup of M3-08 if merged) deletes the extract row in the same unit of work, after the `files` row (03 6.3), through one repository function. The message deletion of M3-07 is always such a path (M3-07 note 7 c keeps the `deleted_at` write in one function of `files/app` for this). The job (note 4a) and the walk (note 9) remove rows of deleted files as a catch-up; `ON DELETE CASCADE` removes the row with the `files` row.

11. `SearchFiles` (14 2, 4.5, 4.6, 10; extends M3-14 note 6; Ruling for docs/DECISIONS.md for the union, the rank and the headline). The file text joins the candidates through a second branch with its own GIN-indexed predicate, so neither branch needs an `OR` across tables:

    ```sql
    WITH by_file AS (
      SELECT f.id, f.created_at
      FROM files f JOIN message_attachments a ON a.file_id = f.id
      WHERE f.search_vector @@ @tsq::tsquery
        AND a.channel_id = ANY(@channel_ids::uuid[])
        -- plus the deliverable, attached, filter and reference-time conditions of M3-14
      ORDER BY f.created_at DESC, f.id DESC
      LIMIT @candidate_limit
    ), by_text AS (
      SELECT f.id, f.created_at
      FROM file_text_extracts e
      JOIN files f ON f.id = e.file_id
      JOIN message_attachments a ON a.file_id = f.id
      WHERE e.search_vector @@ @tsq::tsquery
        AND a.channel_id = ANY(@channel_ids::uuid[])
        -- plus the same conditions
      ORDER BY f.created_at DESC, f.id DESC
      LIMIT @candidate_limit
    ), cand AS (
      SELECT id, created_at FROM by_file
      UNION
      SELECT id, created_at FROM by_text
      ORDER BY created_at DESC, id DESC
      LIMIT @candidate_limit
    )
    ```

    - `@candidate_limit` is `search.max_candidates` plus one, exactly as M3-14 computes `candidates_capped`; the limit applies to the union, so a file found by name and by text counts once (14 4.5 item 1). Use the attachment columns the merged M3-14 query uses (R-DM-43).
    - Score: `ts_rank(f.search_vector || coalesce(e.search_vector, ''::tsvector), tsq, search.rank_normalization)` over a `LEFT JOIN` of the extract row, then the recency division of M3-14; the weights of 03 5 make a name hit (A) rank above a transcript hit (B) above a text hit (C, D).
    - Headline input: `original_name`, the transcript and `text_content`, joined by line feeds, cut to `search.headline_input_max_chars`; configuration `coalesce(e.search_language, 'simple')` instead of `simple`, so a stem match in the document text is highlighted. Segments, markers and options stay as M3-14 builds them.
    - The extract row's state is not filtered: `failed` and `unsupported` have an empty vector, a `pending` refresh keeps its old text (note 9).
    - The query keeps `channel_id = ANY(...)` in every branch, so the SQL lint needs no exception for it.

12. Configuration (register in `internal/wire/configkeys_search.go` with `chapter14`; kinds of the M1-07 schema, `Min` 1 for sizes and counts and a positive minimum for durations): `search.extract.max_input_bytes`, `search.extract.max_text_bytes`, `search.extract.max_uncompressed_bytes`, `search.extract.max_archive_entries` (`KindInt`), `search.extract.timeout` (`KindDuration`), and two new keys this ticket adds to 14 15 in the same change: `search.extract.refresh_interval` (`KindDuration`, default `24h`, "interval of the maintenance job that re-extracts rows of older extractor versions") and `search.extract.refresh_batch_size` (`KindInt`, default `500`, "files per run of the maintenance job"). No cross-check: no combination of these values is unsafe. `search.extract.tika_url` stays out of the schema (V3); a configuration that sets it is rejected as an unknown key by the loader of M1-07, and `tools/configdoc/planned.yaml` shows it as planned for V3. Regenerate `docs/operations/configuration.md`; `make config-doc-check` compares schema and chapter.

13. Observability and lint. No new signal: the job wrapper of M1-14 gives the span `job files.extract_text` and `job files.extract_text_refresh` with `bothy.job.kind` and `bothy.outcome`, and M1-68 records `bothy.job.duration` per kind. Failed extractions are not a separate metric because `signals.md` names none (Ruling for docs/DECISIONS.md). The reads of the job and the walk over `files` and `file_text_extracts` have no principal and no channel set: list each under `exceptions` in `tools/lint/sql_visibility.yaml` with category `system_job` and a reason; writes are not checked by the lint.

14. Fixtures and seams.
    - `internal/files/adapters/extract/extracttest` builds every test file with `archive/zip` and string templates at test time: DOCX, XLSX, PPTX, ODT, ODS and ODP with given texts, written in the entry order Office and LibreOffice use (`[Content_Types].xml`, `_rels/.rels`, then the parts; for ODF a stored `mimetype` entry first), so that the detector of M3-04 recognises them; the zip bomb variants of note 6 (declared size over the limit, a lying header written with `zip.Writer.CreateRaw`, too many entries, encrypted flag, unknown method, duplicate part name); and the XML variants of note 7 (external `SYSTEM` entity with an `http` URL and with a `file` URL, parameter entity, internal entity expansion, plain document type declaration, undefined entity, a non-UTF-8 encoding declaration). Every test that relies on detection asserts the detected type first, so a fixture the detector misreads fails loudly instead of testing `unsupported`.
    - The version-bump test constructs the `files` use cases directly with the real repository (`pgtest`), the file system driver in a `mktemp -d` root and a wrapper around the real `extract.Registry` that reports `builtin-ooxml/2` and supports one more type (`text/rtf` through the text adapter); production code has no option for this.
    - Integration tests through `internal/wire` use the real registry with the configuration keys set low where a test needs a limit to trip.

15. Rulings for `docs/DECISIONS.md` section 10 (one row each, `D-M3-15-<n>`, with reason): trigger only for deliverable attachments of supported types, row and job in the transaction that makes the file deliverable, language taken once (note 2); `unsupported` written only by the job (notes 2 and 4); the type list, `<adapter>/<n>` identifiers and the golden rule that forces a version rise (note 3); no inbound port `TextExtracts` (note 3); the closed code set, `object_missing` and the transient error rule, no River job timeout (note 4); text normalisation, UTF-16 by byte order mark only, HTML block elements and no charset conversion (note 5); the whole object in memory for archives, declared sizes of the read parts against the inflation limit, duplicate names, encryption and methods, and the wording of SEC-UP-5 and SEC-UP-7 (note 6); rejection of every document type declaration, UTF-8 only, the run guard, local-name matching and the fixed parts per format (note 7); halving on the vector limit, no event and no audit (note 8); the refresh job, its two keys, the decision table, keeping text during a refresh and the catch-up deletion (note 9); deleting the text when `deleted_at` is set (note 10); the union of two branches, the combined rank and the headline input and configuration (note 11); no extraction metric (note 13).

## Tests (write them first)

Unit:

- `internal/files/domain`: `TestExtractApplies` (table over purpose `attachment`, `avatar`, `branding`, `gdpr_export` times `clean`, `skipped`, `infected`, `error`, `pending`, `failed` and deleted, times supported: true only for a deliverable attachment of a supported type); `TestRefreshDecision` (one row per line of the table in note 9, plus `done` with the current version and `failed` with another code under the current version, both nothing); `TestTextBuilder` (cut exactly at the limit with a multi-byte rune at the boundary, `truncated` set only when something was dropped, invalid UTF-8 to U+FFFD, NUL and C0 controls to a space with tab and line feed kept, separators never doubled, `Full()`); `TestHalveText` (rune boundary, empty input stays empty, repeated halving reaches the empty string).
- `internal/files/adapters/extract`: `TestVersions` (each listed type maps to its adapter; `text/html`, `text/rtf`, `image/png`, `application/zip`, `application/pdf` and `application/octet-stream` are mapped or unsupported exactly as note 3 says; identifiers have the form `<adapter>/<n>`); `TestExtractorGolden` (note 3, including the refusal of `-update` under an unchanged version); `TestBuiltinText` (plain UTF-8, UTF-8 with byte order mark, UTF-16 LE and BE with byte order mark, invalid bytes, NUL, CSV, JSON, `text/x-python`, truncation at `search.extract.max_text_bytes` with `truncated` and no read beyond what the builder needed); `TestBuiltinHTML` (script and style text absent, entities decoded, block elements separate words, `Rech<b>nung</b>` gives `Rechnung`, comments absent, broken markup still gives text); `TestDOCX` (paragraphs, tabs and breaks, `delText` and `instrText` absent, strict namespace, missing `word/document.xml` gives `archive_invalid`); `TestXLSX` (shared strings in order, `rPh` absent, no shared strings part gives empty text); `TestPPTX` (`slide2` before `slide10`, text of all slides); `TestODF` (ODT, ODS and ODP; `s`, `tab`, `line-break`; annotations and tracked deletions absent; missing `content.xml` gives `archive_invalid`); `TestZipLimits` (too many entries gives `archive_entries_exceeded` and no entry opened; declared size of the read parts over `search.extract.max_uncompressed_bytes` gives `archive_size_exceeded` with zero bytes inflated; a lying header gives `archive_invalid` after at most the declared size plus one byte inflated; the shared inflation counter trips across two parts; encrypted entry, unknown method, duplicate part name and non-zip bytes give `archive_invalid`); `TestXMLDoctypeRejected` (each XML variant of note 14: `xml_doctype` for every document type declaration, `xml_invalid` for the undefined entity and the non-UTF-8 declaration, in DOCX and in ODT); `TestXMLRunGuard` (a text run longer than `search.extract.max_text_bytes` ends the part with the earlier text kept and `truncated` set; a counting reader shows the decoder consumed at most the limit plus one read buffer); `TestExtractContextDeadline` (a deadline already passed and a stalling reader both end with the context error, for every adapter); `TestExtractImportsNoNetwork` (`go list -deps ./internal/files/adapters/extract` contains neither `net`, `net/http` nor `os/exec`).
- Fuzz (seed corpora run in every `go test`): `FuzzExtractText`, `FuzzExtractHTML`, `FuzzExtractOOXML`, `FuzzExtractODF`: never panic; the result is either text of at most `search.extract.max_text_bytes` bytes, valid UTF-8 and without NUL, or an `*domain.ExtractError` with a code of note 4.
- `internal/files/app` with fakes (object store counting reads, scripted extractor, fake clock, fake repository, fake `identity.Directory`): `TestScheduleTextExtract` (row with the instance language and one job for a supported attachment; nothing for an unsupported type or another purpose; a second call inserts no second job); `TestExtractTextRun` (not `pending`: no effect; file deleted: row removed and no object read; unsupported at run time: `unsupported` with NULL `extractor`; size over `search.extract.max_input_bytes`: `failed` `input_too_large` without a read; success: `done` with version and clock time; an extractor code: `failed` with that code and NULL text; `ErrNotFound`: `failed` `object_missing`; another store error: error returned and row still `pending`; extraction deadline: `failed` `timeout`; cancelled job context: `ctx.Err()`); `TestExtractTextVectorLimit` (repository answers `ErrVectorTooLarge` twice: two halvings at rune boundaries, `truncated` true, three writes); `TestExtractRefreshRun` (each decision of the table, the compare-and-set skips a row changed in between, the next job is inserted only after a full batch, deleted files lose their rows).

Integration (PostgreSQL 18 via `pgtest`, real modules through `internal/wire`, HTTP through the real chain and the M1-71 harness with `BuildMatrixWorld` as the M3-14 tests use it; the file system driver in a `mktemp -d` root; `file.scan.mode = off` unless a test names the fake clamd; uploads with `createUpload`, `uploadPart` and `completeUpload` and attachment with `createMessage` and `attachment_file_ids`, all by Alex in Support/Allgemein (G-1 gives `upload`); Sam reads only Marketing; jobs run through the real `platform/jobs` runtime in role `all` or by calling the handlers directly, never by sleeping for a window):

- `TestMigrationFileTextExtracts`: catalog check of columns, types, defaults, the CHECK, the primary key, the foreign key with `ON DELETE CASCADE`, the GIN index and the partial index; the generated column is stored (`attgenerated = 's'`) with the expression of 03 5; `bothy_app` has exactly `SELECT, INSERT, UPDATE, DELETE`; `make migrations-lint` is green.
- `TestDOCXFoundByContentWords` (plan row, 14 2): a DOCX named `notes.docx` whose document holds "Die Rechnung fur den Lieferantenvertrag ist fertig." with U+00FC (u with diaeresis) as the second letter of "fur" is uploaded, attached and extracted with `locale.default` German; the row is `done` with `builtin-ooxml/1`, `search_language` `german` and `truncated` false. Alex's `search` with type `files` and `q=Lieferantenvertrag` returns the file with a matching segment; `q=Rechnungen` finds it through the stem (weight C); a word in neither name, transcript nor document gives no hit; Sam gets no hit for any of these queries. A variant with the fake clamd of M3-04 answering `stream: OK` proves the trigger for `clean`.
- `TestOfficeAndTextFormatsFound`: XLSX, PPTX, ODT, ODS, ODP, a plain text file, a CSV file and an HTML file are each found by a word only in their content; the HTML file is not found by a word only inside its `script` element.
- `TestZipBombFailedWithoutInflating` (plan row, 14 12, SEC-T08 file text part): with `search.extract.max_uncompressed_bytes` and `search.extract.max_archive_entries` set low, the bomb variants of note 14 end `failed` with `archive_size_exceeded`, `archive_invalid` and `archive_entries_exceeded`, `text_content` NULL, `extractor` set; the counting seam of the adapter shows that no part was inflated beyond its limit; the file stays deliverable and is still found by its name.
- `TestXXEFailedWithoutFetching` (plan row, 14 12): a DOCX and an ODT whose XML part declares an external entity pointing at a loopback HTTP server started by the test, and at a `file` URL of a file in a `mktemp -d` directory holding a canary value, both used in the text; each row is `failed` with `xml_doctype`; the HTTP server counted zero requests; the canary appears in no column of `file_text_extracts`, in no captured log line and in no span.
- `TestNewExtractorVersionReextracts` (plan row, 14 5.1 item 4; construction of note 14): a DOCX and a text file are `done` under version 1 and an RTF file has no row; with the wrapper reporting `builtin-ooxml/2` and supporting `text/rtf`, and `search.extract.refresh_batch_size` set to 1, the walk chains three runs; the DOCX row is `pending` with its old text and still found during the refresh, then `done` with `builtin-ooxml/2` and a later `extracted_at`; the text row keeps its `extracted_at`; the RTF file gets a row and is extracted; a second walk changes nothing.
- `TestRefreshRecoversDiscardedJob`: a `pending` row without a queued job is extracted after one walk.
- `TestExtractOnlyForDeliverableAttachments`: with the fake clamd answering a `FOUND` and an `ERROR` line, and with a checksum mismatch, no row and no `files.extract_text` job exist; an unsupported type (`image/png`) gets no row; a file of another purpose built with the files repository directly gets no row.
- `TestExtractLimits`: a text file over `search.extract.max_input_bytes` ends `failed` `input_too_large` without an object read; a text over `search.extract.max_text_bytes` ends `done` with `truncated` and exactly the limit or less; a stalling object store fake under `search.extract.timeout` ends `failed` `timeout`.
- `TestExtractVectorLimit`: a text file of `search.extract.max_text_bytes` bytes of distinct hex tokens ends `done` with `truncated` true and a non-empty vector, without an endless retry.
- `TestExtractRowRemovedWithFile` (DP-T01 file text part, 14 5.3): deleting the message that carries an extracted DOCX (`deleteMessage`, through the merged soft-delete path) removes the extract row in the same transaction and `search` finds nothing; the path that sets `files.deleted_at` there is the one of M3-07 (note 10). Deleting the `files` row as the owner role removes the extract row by cascade; the walk removes a row whose file has `deleted_at` set.
- `TestExtractNoContentInTelemetry` (SEC-LOG-4, SEC-T10 for this path): a document whose text and file name carry canary values, one run ending `done` and one ending `failed`; captured logs, spans and metric attributes contain neither canary; the failure log has only `bothy.file.id` and `bothy.reason`; the `river_job` arguments contain only the file id.
- `TestSearchFilesCombinesVectors` (14 2, 4.5): of two files of the same age, the one matching by name ranks above the one matching only by text; a file matching by name and by text appears once; with `search.max_candidates` set low, files found only by text count toward `candidates_capped` like files found by name; the M3-14 tests of `search` stay green.
- `TestExtractJobsRegistered`: `files.extract_text` in queue `files` and `files.extract_text_refresh` periodic in queue `maintenance` with `search.extract.refresh_interval` and `RunOnStart`, both only in roles `worker` and `all`; both kinds pass the argument and kind checks of M1-14.
- `TestConfigSchemaBuilds` lists the seven keys with the chapter defaults; a configuration with `search.extract.tika_url` is rejected as an unknown key.
- The SQL visibility lint of M1-70 (part of `make check`) passes with the new `system_job` exceptions and no exception for the search query.

Commands that must pass, from the repository root:

- `cd services/server && go test -race -count=1 ./internal/files/... ./internal/search/... ./internal/wire/... ./internal/platform/jobs/... ./migrations/... ./test/...`
- a short local run of each fuzz target (`cd services/server && go test -run '^$' -fuzz FuzzExtractOOXML ./internal/files/adapters/extract/`, likewise the other three, each stopped after a short `-fuzztime` of your choice); report duration and result and commit any crashing input as a seed after the fix
- `make migrations-lock`, `make migrations-lint`, `make arch`, `make codegen-check`, `make register-check`, `make config-reference` (no diff afterwards), `make config-doc-check`, `make licenses`
- `make docs`, `make linkcheck`
- `make check`

## Acceptance criteria

- [ ] `file_text_extracts` exists exactly as 03 4.7 and 03 5 define it, with the grant for `bothy_app`; `make migrations-lint` is green.
- [ ] A DOCX attached to a message is found by `search` with type `files` through words that appear only inside the document, by its readers only (plan row).
- [ ] XLSX, PPTX, ODT, ODS, ODP, text and HTML files are extracted as 14 5.2 says; script and style content is not.
- [ ] A zip bomb and an XML part with an external entity end `failed` without inflating beyond the limits and without any fetch (plan row, 14 12, SEC-T08 file text part).
- [ ] A new extractor version re-extracts the older rows, keeps them findable meanwhile and leaves current rows untouched (plan row).
- [ ] `search.extract.max_input_bytes`, `search.extract.max_text_bytes` with `truncated`, `search.extract.max_uncompressed_bytes`, `search.extract.max_archive_entries` and `search.extract.timeout` hold; a text at the vector limit still ends `done`.
- [ ] Only deliverable attachments of supported types get a row and a job, in the transaction that makes them deliverable; the language is the instance's.
- [ ] File text appears in no log, span, metric, job argument or event, and it is deleted when the file is deleted.
- [ ] File text ranks below name and transcript and passes the same rights filter and candidate limit as M3-14.
- [ ] Chapters 14, 02, 12 and 19, `configuration.md`, `planned.yaml`, the `files` README, the CHANGELOG and the rows `D-M3-15-<n>` are updated.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why.
- README of `files` updated (jobs `files.extract_text` and `files.extract_text_refresh`, the port `TextExtractor`, the adapter `extract` with its types and versions and the golden rule, the table and its retention class, the memory bound of note 6, the deletion rule of note 10, the test requirements); the README of `platform/jobs` stays as M3-04 left it (`HandlerOptions.Timeout`).
- Rulings of note 15 recorded in `docs/DECISIONS.md` (one row each with reason).
- `services/server/CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`), which also checks T-UP-01 for the parsers (zip bombs, external entities, memory); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. What is not proven.
5. Follow-ups that could not be done in this scope, each with its reason, and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
