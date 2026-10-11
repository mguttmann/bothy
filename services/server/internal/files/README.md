# Module `files`

Status: skeleton (M1-05). The port package exists; interfaces and use cases come with the tickets of the milestone.
The normative source is [docs/spec/02-architecture.md](../../../../docs/spec/02-architecture.md); if this file differs, the chapter applies.

## Purpose

Upload sessions in parts, object storage drivers, scan, metadata removal, thumbnails, attachments, transcripts, file text extracts.

## Code

- `internal/files` (`doc.go`): port as the root package, so far only with a package comment. Interfaces, DTOs and In-Tx events come with the ticket that implements them.
- `domain/`, `app/` and `adapters/<kind>/` are created with the first use case. `tools/archgen` generates their components from `architecture/modules.yaml`; a package outside these layers fails `make arch`.

## Port (root package `internal/files`)

- Inbound: `Uploads` (sessions, parts), `Files` (read, open, preview, file tickets, delete), `Attachments`, `Transcripts`, `TextExtracts`; declares `AttachmentLocator`.
- Emits in-tx events: `FileFinalized` (scan and, if applicable, transcript final).
- Subscribes to in-tx events: none.
- Implements source ports and cross-cutting ports: `identity.AvatarFiles`, kernel `personaldata.Participant`.

## Dependencies

May use the ports of these modules: `identity`, `chat`.

Reason: implements `AvatarFiles`; check the upload channel.

In addition, for all modules: `internal/kernel`; `internal/platform` and `api/` only in `adapters/`.
Checked by go-arch-lint (ADR 0008).

## Tables

`files`, `file_upload_sessions`, `file_upload_parts`, `file_thumbnails`, `message_attachments`, `file_text_extracts`.

Only this module writes and reads these tables by SQL (03-data-model.md, principle 12).

## Actions (05-actions.md)

`file.upload`, `file.read`, `file.delete`, `file.transcribe`, `file.transcript.submit`.

## Events and WebSocket ops

- Durable: `file.updated`.
- Ephemeral: none.
- WebSocket ops (client to server): none.

## Background jobs

| Job | Trigger | Queue |
|---|---|---|
| `files.scan` | `InsertTx` when an upload completes | `files` |
| `files.thumbnail` | after `clean` | `files` |
| `files.extract_text` | after `clean`, supported type | `files` |
| `files.transcribe` | `file.transcribe` with server transcription | `files` |
| `files.cleanup_pending` | periodic | `maintenance` |
| `files.purge_objects` | periodic, deleted files | `maintenance` |

## Notes for the build

- Files are served only with `scan_status` `clean` or `skipped` and only through the server after `file.read` (03, 4.7).
- Scan, thumbnails and transcription run as jobs in the worker; network calls never in a transaction (02, 8.5).
- Object keys contain no names or IDs of channels; no deduplication (R-DM-13).

## Chapters

[02-architecture.md](../../../../docs/spec/02-architecture.md), [12-files.md](../../../../docs/spec/12-files.md)
