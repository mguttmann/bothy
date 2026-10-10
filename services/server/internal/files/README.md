# Modul `files`

Status: geplant (M0). Dieses Verzeichnis enthält noch keinen Code. Normativ ist
[docs/spec/02-architektur.md](../../../../docs/spec/02-architektur.md); bei Abweichung gilt das Kapitel.

## Zweck

Upload, Objektspeicher-Treiber, Scan, Vorschaubilder, Anhänge, Transkripte.

## Port (Wurzelpaket `internal/files`)

- Eingehend: `Uploads`, `Files` (Lesen, Öffnen, Vorschau, Löschen), `Attachments`, `Transcripts`; deklariert `AttachmentLocator`.
- Gibt In-Tx-Ereignisse aus: keine.
- Abonniert In-Tx-Ereignisse: keine.
- Implementiert Quellen-Ports und Querschnitts-Ports: `identity.AvatarFiles`, kernel `personaldata.Participant`.

## Abhängigkeiten

Darf die Ports dieser Module nutzen: `identity`, `chat`.

Grund: implementiert `AvatarFiles`; Upload-Kanal prüfen.

Zusätzlich für alle Module: `internal/kernel`; `internal/platform` und `api/` nur in `adapters/`.
Geprüft durch go-arch-lint (ADR 0008).

## Tabellen

`files`, `file_thumbnails`, `message_attachments`.

Nur dieses Modul schreibt und liest diese Tabellen per SQL (03-datenmodell.md, Grundsatz 12).

## Aktionen (05-aktionen.md)

`file.upload`, `file.read`, `file.delete`, `file.transcribe`.

## Ereignisse und WebSocket-Ops

- Dauerhaft: `file.updated`.
- Kurzlebig: keine.
- WebSocket-Ops (Client an Server): keine.

## Hintergrundjobs

| Job | Auslöser | Warteschlange |
|---|---|---|
| `files.scan` | `InsertTx` beim Abschluss eines Uploads | `files` |
| `files.thumbnail` | nach `clean` | `files` |
| `files.transcribe` | `file.transcribe` bei Server-Transkription | `files` |
| `files.cleanup_pending` | periodisch | `maintenance` |
| `files.purge_objects` | periodisch, gelöschte Dateien | `maintenance` |

## Hinweise für den Bau

- Auslieferung nur bei `scan_status` `clean` oder `skipped` und nur über den Server nach `file.read` (03, 4.7).
- Scan, Vorschaubilder und Transkription laufen als Jobs im worker; Netzaufrufe nie in einer Transaktion (02, 8.5).
- Objektschlüssel enthalten keine Namen oder IDs von Kanälen; keine Deduplizierung (R-DM-13).

## Kapitel

[02-architektur.md](../../../../docs/spec/02-architektur.md), [12-dateien.md](../../../../docs/spec/12-dateien.md)
