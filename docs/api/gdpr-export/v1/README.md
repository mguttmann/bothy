# Export format of privacy requests, version 1

The export package of a privacy request ([20-privacy.md](../../../spec/20-privacy.md) section 5.2,
[06-api.md](../../../spec/06-api.md) section 10) is a ZIP archive. `manifest.json` names the
version of the format in `schema_version`; this directory describes version 1 with JSON Schema
(draft 2020-12). A change that is not additive raises `schema_version` and adds a directory `v2`.

| File of the archive | Schema |
|---|---|
| `manifest.json` (last entry) | [`manifest.schema.json`](manifest.schema.json) |
| `profile.json` | [`profile.schema.json`](profile.schema.json) |
| `identities.json` | [`identities.schema.json`](identities.schema.json) |
| `devices.json` | [`devices.schema.json`](devices.schema.json) |
| `access.json` | [`access.schema.json`](access.schema.json) |
| `files/files.json` | [`files.schema.json`](files.schema.json) |
| `personal.json` | [`personal.schema.json`](personal.schema.json) |
| `messages.jsonl` | one line: [`lines/messages.schema.json`](lines/messages.schema.json) |
| `reactions.jsonl` | one line: [`lines/reactions.schema.json`](lines/reactions.schema.json) |
| `mentions.jsonl` | one line: [`lines/mentions.schema.json`](lines/mentions.schema.json) |
| `runs.jsonl` | one line: [`lines/runs.schema.json`](lines/runs.schema.json) |
| `interactions.jsonl` | one line: [`lines/interactions.schema.json`](lines/interactions.schema.json) |
| `audit.jsonl` | one line: [`lines/audit.schema.json`](lines/audit.schema.json) |
| `files/<file_id>` | the bytes of an own upload, listed in `files/files.json` |
| `README.txt` | explanation of the files in the language of the person |

Every JSON object file has one member per section; each section is written by the module that
owns its table and described in `sections/<file>.<section>.schema.json`. Every file of the table
is present in every package: an object file without data is `{}` with empty sections, a JSON Lines
file without data is empty.

Rules of the content (20 section 5.2): only data and metadata of the person; no message or other
content of another person; channel and thread names only where the person may still read the
channel, otherwise only IDs; no secret, hash or key. Times are RFC 3339 in UTC, IDs are UUIDs.
