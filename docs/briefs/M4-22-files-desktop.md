# M4-22: Files, images and voice messages on the desktop

Status: ready | Depends on: M4-20, M3-05 | Milestone: M4

## Goal

After this ticket the desktop app sends, shows, downloads and plays files: attaching by the attachment button, drag and drop and paste creates one upload entry per file in the outbox before the message (12 3: session with size, SHA-256 and declared type, parts with `Content-Digest`, completion, resumption from the confirmed parts after an abort), with progress and the states of 12 2 ("being checked", "Upload failed", "blocked"); messages show attachments with name, size and state, images as thumbnails loaded only from the server (`downloadThumbnail`, SEC-OUT-2), opening and saving through `downloadFile` with the `Repr-Digest` check, and media players through short-lived file tickets; voice messages are recorded as AAC-LC in `.m4a` (package `record`, `client.voice.bitrate`, at most `voice_max_duration_ms` from `InstanceLimits`), sent as `kind = voice`, played at speed 1, 1.5 and 2, and show the transcript when `file.updated` reports it, with "Transcribe" where the server allows it; the local file cache keeps bytes with the metadata rows of `local_store` and is cleaned with the database; and the session column gets the "Files" section. `409 file_not_ready` is shown as "being checked" and retried on `file.updated`. Why: files are part of the conversation with agents (screenshots, logs, voice), and every byte a client shows must come from the server's checked copy, never from a URL an agent wrote (T-AI-06).

## Read first

- `docs/PLAN.md`: section 3; section 10 (Red team: T-AI-06; rows M4-15, M4-20, M4-22, M4-34).
- `docs/spec/01-product.md`: 3.4 "Files and voice", 5.1 D1 (session column "Files"), 6.6, 8 (labels of attachments and players), 14.
- `docs/spec/12-files.md`: 1, 2 "States of a file", 3 "Upload flow" (3.1, 3.2, 3.4, 3.6, 3.7; `presigned` mode 3.3 only to know the client follows `mode`), 6.2 (names), 7.1 and 7.3 (images and thumbnails), 8 "Voice messages", 9 "Download" (9.1 to 9.3, tickets, `Repr-Digest`), 10 "Attaching and agent invocation", 15, 18 (configuration values the client meets).
- `docs/spec/15-client-architecture.md`: 6.1 (`files`, `message_attachments`, `file_cache`), 6.3 (cleanup), 7.5 (uploads before messages), 13 "Voice recording", 14 "File upload".
- `docs/spec/04-sync-protocol.md`: 2.2 rule 5 (redacted `file.updated`), 13.2.
- `docs/spec/19-security.md`: SEC-OUT-2, SEC-T13; `docs/security/threat-model.md` T-AI-06.
- `docs/api/openapi.yaml`: `createUpload` (`UploadCreate`, `UploadSession`), `uploadPart`, `completeUpload`, `getUpload`, `abortUpload`, `getFile`, `downloadFile`, `downloadThumbnail`, `createFileTicket`, `getTranscript`, `requestTranscription`, `deleteFile`, `getInstance` (`InstanceLimits`); problem codes `file_not_ready`, `file_blocked`, `payload_too_large`, `unsupported_media_type`, `file_not_attachable`, `voice_format_unsupported`, `voice_too_long`, `transcription_unavailable`.
- `docs/DECISIONS.md`: D-W-100, D-W-114, D-W-115; section 10.
- Briefs: `docs/briefs/M4-20-thread-view.md` (composer, history, session column), `docs/briefs/M4-15-sync-outbox.md` (dependency rule, executors), `docs/briefs/M4-12-local-store.md` (`file_cache`, `FileCacheDao`, cleanup), `docs/briefs/M4-14-sync-catch-up.md` (`FileCacheEraser`), `docs/briefs/M4-07-core-components.md` (`DsMarkdown` attachment resolver), `docs/briefs/M4-17-fake-server.md` (handler registry); for the server side `docs/briefs/M3-03-upload-sessions.md`, `docs/briefs/M3-05-file-download.md`, `docs/briefs/M3-06-image-metadata-thumbnails.md`, `docs/briefs/M3-09-voice-transcript.md`.

This ticket touches no API operation, no table of chapter 03, no action, no event and no configuration key.

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
7. No en dash and no em dash in any file you write; the `text-rules` pre-commit hook rejects them, and the ARB test of M4-06 rejects them in interface texts. Use a hyphen or rephrase.
8. White label: examples and test data use `example.com` and the people Alex, Sam and Robin; no company, host or personal names, no paths under a home directory. Interface texts come from `packages/l10n`, never as literals in widgets.
9. Limits only through named keys or the server: `file_max_upload_bytes`, `message_max_attachments` and `voice_max_duration_ms` from `InstanceLimits` (D-W-115), `client.voice.bitrate` from `AppConfig`, part size and count from the upload session; no literal in code.
10. Specification first: 15 7.5 and 15 14 say a message waits until its files are `complete`, while 12 3.4 allows attaching once the upload session is completed and holds the agent invocation until the files are final (12 10). This ticket follows 12 3.4 (Design note 2) and aligns 15 7.5 and 15 14 in the same change.

Product requirements implemented here:

11. Every file is its own outbox entry before the message that attaches it; the client checks size against `file_max_upload_bytes` and the count against `message_max_attachments` before sending (15 14, 04 13.2).
12. Upload as 12 3: session with name, size, SHA-256 and declared type (`Idempotency-Key`), parts with `Content-Digest`, resume from the parts `getUpload` confirms after an abort, completion; a message attaches the file after completion (12 3.4).
13. States shown as 12 2 names them: "being checked", "Upload failed", "blocked"; `409 file_not_ready` waits for `file.updated` (06 5).
14. Previews only from server files: thumbnails through `downloadThumbnail`, never from a URL in the content; the local preview before sending shows the local file (15 14, SEC-OUT-2).
15. Downloads through the server only; the client checks `Repr-Digest`; media playback through file tickets (12 9).
16. Voice: AAC-LC in MP4 (`.m4a`), mono, `client.voice.bitrate`, at most `voice_max_duration_ms`; microphone permission at the first use; playback at 1, 1.5 and 2; the transcript appears when `file.updated` reports it (15 13, 12 8).
17. The file cache lives with the encrypted database's metadata and is deleted on removal, revocation and cleanup (15 6.3, 04 8.3).

## Scope

Create in `apps/client/lib/`:

- `files/upload_executor.dart` (outbox executor `upload`, Design note 1), `files/upload_planner.dart` (hashing in a background isolate, part plan from the session), `files/attach_controller.dart` (button, drag and drop, paste), `files/attachment_tile.dart` and `files/attachment_mapping.dart` (states of 12 2 to the view), `files/image_preview.dart` (thumbnails from the server, full view), `files/download_service.dart` (download with `Repr-Digest`, save dialog, open), `files/file_cache.dart` (bytes on disk under the instance's cache folder, the `FileCacheEraser` of M4-14), `files/ticket_player.dart` (audio and video through tickets).
- `voice/recorder.dart` (port `VoiceRecorder` with the `record` implementation, Design note 4), `voice/voice_button.dart`, `voice/voice_player.dart` (speeds), `voice/transcript_view.dart`.
- `session_column/files_section.dart`.
- The resolver for internal file references of `DsMarkdown` (M4-07 Design note 2) on the files of the message.
- Fake server handlers through the registry of M4-17 for the upload, download, ticket, transcript and file operations of Read first, with the states of 12 2 and the faults `abort after part n`, `file_not_ready`, `blocked`.
- Tests under `test/files/` and `test/voice/`, goldens under `test/goldens/files/`, integration tests under `integration_test/` (Tests).
- macOS: `NSMicrophoneUsageDescription` in `Info.plist` and the audio input entitlement; Windows: nothing beyond the package.
- ARB keys for every new text in `packages/l10n`.

Change:

- `apps/client/pubspec.yaml` (`record`, `crypto` for SHA-256, `file_selector` or `desktop_drop` only if `make licenses` and `make osv` stay green, `just_audio` or the player the license gate allows, Design note 4), `apps/client/README.md` (files and voice).
- `docs/spec/15-client-architecture.md` 7.5 and 14 (attach after the session is completed, 12 3.4).
- `docs/DECISIONS.md` section 10: rows `D-M4-22-<n>`; root `CHANGELOG.md` under `[Unreleased]`.

## Out of scope

- Images as JPEG from a photo library (mobile, M5); share sheet and Share Target (M5, later on Windows).
- Video posters and transcoding (12 8.3 "later").
- File text extraction and search (M3-15, M4-23).
- Files from agents arrive like any attachment; nothing agent-specific here.

## Boundaries

- Work in a dedicated git worktree on the branch `feat/M4-22-files-desktop`; one writer per worktree.
- Conventional Commits with the package or component as scope (for example `feat(client): ...`, `feat(sync-engine): ...`, `build(gates): ...`, `docs(spec): ...`) and a DCO sign-off on every commit (`git commit -s`); squash into meaningful commits before the merge (`docs/quality/push-discipline.md`).
- Never push to `main`, never tag and never create remote repositories; the only remote steps are the branch push and the pull request of `AGENTS.md`. Never publish a Dart package (`publish_to: none` in every `pubspec.yaml`).
- Do not modify files outside the scope above, except the CHANGELOG, README and `docs/DECISIONS.md` entries named in this brief.
- Follow the host rules supplied by the launcher (ports, machine rules).
- Tests bind only `127.0.0.1` with port 0 and make no network call; fakes replace the server unless a section below names a container. Temporary directories only via `mktemp -d` (Dart `Directory.systemTemp.createTemp`, pytest `tmp_path`), removed at the end. Stop only processes you started, by PID.
- Never read or print secrets. No network calls except package downloads for pinned tools and pub packages and what the gates of `make check` do themselves.
- The Flutter SDK at the version pinned in the root `pubspec.yaml` must be on `PATH` (and Go and Docker where a section names them); without them the gates fail by design (R-QA-02), and the ticket stops with `Verdict: BLOCKED <tool> missing` instead of skipping.
- Never lower a threshold, disable a gate, delete a test, raise `repo.max_file_kb` or extend `quality/licenses/allowed.txt`; never exclude hand-written code from a lint, format, coverage or doc gate.
- The product name appears only at the places of 02 "Code layout" ("Places that carry the product name"); the app shows `branding.product_name` from the instance, never a literal.
- No permission logic in the client (15 1 principle 1): controls follow server responses, snapshots and events, never an own derivation from grants.
- If the core contract (chapters 00, 03, 04, 05) or `docs/api/` lacks something this ticket needs, stop with `Verdict: BLOCKED <reason>` instead of building a deviation (P-11).
- This brief was written before milestone M1 closed (D-W-100, extended to M4 by D-W-114). Where the merged code of an earlier ticket names a package, type, function, file, provider or make target differently from this brief, follow the code and list each difference in the report; where it behaves differently from the specification, stop with `Verdict: BLOCKED <reason>`.

## Design notes

1. **Upload executor.** `upload` entries carry the local path, name, size and the SHA-256 computed in a background isolate when the entry is created (streamed, never the whole file in memory). Steps: `createUpload` with `Idempotency-Key` = entry ID (retries return the same session); for `mode = direct` send the parts that `getUpload` does not list as confirmed, at most two in parallel, each with `Content-Digest`, reading only that part from disk; `presigned`: PUT to the given URLs the same way; then `completeUpload`. The entry is confirmed when `completeUpload` returned `200` (12 3.4), which releases the dependent message; the file row's states then follow `file.updated`. Progress per entry (bytes confirmed / size) is exposed for the tile. `409 part_conflict` or a session in `failed`, `aborted` or `expired` fail the entry with the reason; "Send again" creates a new upload. Ruling for docs/DECISIONS.md: attach after completion, and the parallelism.

2. **Attaching and the message.** The composer shows chips for the files being uploaded; sending creates the message entry with `depends_on` on every upload entry and `attachment_file_ids` filled from the sessions when they exist (the message executor reads the file IDs from its confirmed dependencies). Over the limits: the composer refuses with the reason before creating entries.

3. **Display and download.** Attachment tiles by the states of 12 2; images with a thumbnail from `downloadThumbnail` (cached in the file cache, variant per size); clicking opens the full image from `downloadFile`. A file that answers `409 file_not_ready` shows "being checked" and reloads on `file.updated`; `file_blocked` shows "blocked". Downloads stream to the cache or the chosen path and compare `Repr-Digest` with the received bytes; a mismatch deletes the bytes and shows an error. Audio and video use `createFileTicket` and pass the ticket URL to the player; a ticket URL is never logged and never stored. Redacted files (deleted) show the tombstone fields only.

4. **Voice.** `VoiceRecorder` port: `start()`, `stop() -> RecordedFile`, `cancel()`, amplitude stream; the implementation uses `record` with AAC-LC, mono, `client.voice.bitrate`, into a temporary file; it stops at `voice_max_duration_ms`. Permission is asked at the first use; a denied permission shows how to allow it in the system settings. Sending: upload entry plus a message with `kind = voice` and exactly one attachment. Player with speed 1, 1.5 and 2 through a ticket. Transcript: shown from `file.updated` (`transcript_status`); "Transcribe" calls `requestTranscription` and shows `transcription_unavailable` as a short text. Tests use a fake recorder; the real microphone is checked by hand once and named in the report. Choose the player package by the license gate; record the choice.

5. **Cache.** Bytes under `<application support>/<instance id>/files/<file id>/<variant>`, rows in `file_cache` with `last_accessed_at`; the cleaner of M4-12 returns paths that `file_cache.dart` deletes; `targetsRemoved` and the wipe delete them (M4-14, M4-18).

6. **Rulings for docs/DECISIONS.md** (`D-M4-22-<n>`): attach after completion and parallel parts (note 1); hashing in an isolate and streaming (note 1); ticket handling (note 3); the player and picker packages (note 4); the cache layout (note 5).

## Tests (write them first)

Widget and unit tests (fake API through the handlers, fake recorder, fake clock):

- `test/files/upload_executor_test.dart`: session, parts with digests, completion; an abort after part 2 resumes with the parts `getUpload` did not confirm; `part_conflict` fails the entry; the message waits for its uploads and attaches their file IDs; over-limit files are refused before any entry.
- `test/files/states_test.dart`: tiles for every view of 12 2; `file_not_ready` shows "being checked" and reloads on `file.updated`; `file_blocked` shows "blocked".
- `test/files/preview_only_from_server_test.dart` (SEC-OUT-2): a message whose Markdown contains an image URL and an attachment shows the thumbnail of the attachment from `downloadThumbnail` and loads nothing else (an `HttpOverrides` fails on any other request).
- `test/files/download_test.dart`: `Repr-Digest` match and mismatch; ticket URLs never logged.
- `test/voice/voice_test.dart`: recording stops at the limit; the message has `kind = voice` and one attachment; speeds; transcript appears on `file.updated`; `transcription_unavailable` text.
- `test/goldens/files/*`: attachment tiles in every state, image preview, voice player with transcript; light and dark, two densities, text scaling 1 and 2, `de` and `en`.

Integration tests (`integration_test/`, against `FakeServer`):

- `upload_outbox_test.dart`: attach two files and send offline; on reconnect the uploads run before the message; the message appears with both attachments; `file_not_ready` until the fake reports `clean`.
- `voice_test.dart`: a voice message with the fake recorder is sent and played back, and its transcript appears.

Commands that must pass, from the repository root:

- `cd apps/client && flutter analyze --fatal-infos && flutter test --coverage`
- on macOS and on Windows: `make apps-platform`; elsewhere it prints its SKIP line
- `make codegen-check`, `make arch`, `make lint`, `make fmt-check`, `make coverage`, `make licenses`, `make osv`
- `make check`

## Acceptance criteria

- [ ] Integration tests for upload in the outbox, `file_not_ready` and previews only from server files (SEC-OUT-2) are green.
- [ ] Uploads follow 12 3 with resumption; downloads check `Repr-Digest`; media plays through tickets.
- [ ] Voice messages record AAC-LC within the limit, play at three speeds and show the transcript.
- [ ] The file cache is cleaned with the database and on removal and revocation; 15 7.5 and 14 follow 12 3.4.
- [ ] `make licenses` and `make osv` green with the chosen packages; rulings recorded.
- [ ] `make check` is green.

## Definition of done

- Code and tests as specified; doc comment on every public symbol (purpose, parameters, return value, errors, example where useful); comments explain why and cite 12 or 15.
- `apps/client/README.md` (English) as in Scope.
- 15 7.5 and 14 updated; rulings of Design note 6 in `docs/DECISIONS.md` section 10.
- Root `CHANGELOG.md` entry under 'Unreleased'.
- An ADR only if you take an architecture decision beyond this brief (MADR 4 as in `docs/adr/0000-template-madr-4.md`, headings and text in English, next free number, `make adr-index`).
- `make check` green.
- An independent code review over the diff, then the red team gate (`docs/quality/red-team-gate.md`, verdict `JA` or `NEIN`); on `NEIN` fix with a test per finding and have the delta checked again. Record both verdicts.
- Land the change as `AGENTS.md` ("How work lands on `main`") describes: a pull request from this branch, squash-merged only when review, red team and `make check`, `ci-ok` and the review threads are green; then `git worktree remove`.

## Report

Write to `{{REPORT_PATH}}`:

1. First line exactly `Verdict: DONE` or `Verdict: BLOCKED <reason>`.
2. Commits (SHA and subject), changed files with line counts, test results with counts (the red run before the change and the green run after), the summary of `make check`.
3. Review and red team verdicts and the findings fixed.
4. Where the integration tests ran, the packages chosen for recording, playback and file picking with their licenses, and whether the real microphone was checked by hand on macOS and Windows.
5. What is not proven (for example a platform test that ran only in CI, or a check done by hand).
6. Follow-ups that could not be done in this scope, each with its reason, every difference to the names of this brief (D-W-100, D-W-114), and spec gaps found.

Then write `{{DONE_PATH}}` with `exit=0 sha=<sha>` or `exit=1 <reason>`.
