# tools/api-checks

Contract checks of the API in the gate (chapter [06](../../docs/spec/06-api.md), section 15, G2
and G3): every copy between 04, 05, 06, [`docs/spec/actions.yaml`](../../docs/spec/actions.yaml),
[`openapi.yaml`](../../docs/api/openapi.yaml), [`asyncapi.yaml`](../../docs/api/asyncapi.yaml) and
[`webhooks.md`](../../docs/api/webhooks.md) is checked against its source. `api_checks.py`
needs only Python and PyYAML and reads the action catalogue through
[`tools/catalog`](../catalog/README.md) (`load_catalog`, `generated_blocks`,
`replace_generated_block`).

## Commands

| Command | Effect |
|---|---|
| `make api-checks` | Gate: both rules, exit 1 with one line per finding (`api_checks.py check`; one rule alone with `--rule api-catalogue-sync` or `--rule api-tables`) |
| `make api-tables` | rewrite the generated blocks in 06 and `webhooks.md` (`api_checks.py generate`); writes nothing while an input is unreadable or malformed, a value cannot become a table cell, a marker is broken, unknown or missing, or one of the files to change cannot be written |
| `uv run --project tools --locked pytest tools/api-checks/tests` | Tests: per sub-check and per block one mutation of a copy of the real inputs, which must be red; the repository itself is green |

Flow of a change: change `openapi.yaml` or `asyncapi.yaml`, run `make api-tables`, and commit
the diff of 06 and `webhooks.md` with it. After a change to
[`docs/spec/actions.yaml`](../../docs/spec/actions.yaml) and `make catalog`, `make api-tables`
belongs to it too, because the column "Receiver and scope" in `webhooks.md` comes from the
catalogue. A hand edit in a generated block fails the gate.

Every finding has the form `<rule>: <sub-check>: <subject>: <detail>`. No call ends in a
traceback. If a sub-check, the generation of a block or the form check of `openapi.yaml` or
`asyncapi.yaml` raises an unexpected error, it becomes a finding of this sub-check, this block or
the sub-check `source` of the file (for a block `input not renderable`, otherwise
`input not checkable`); the others keep reporting. The command line turns any other error into
one line `<rule>: source: input not checkable (...)` (for `generate`: `input not renderable`)
with exit 1. The sub-check `source` reports an input that cannot be read or loaded (invalid YAML,
a value that PyYAML cannot build, such as an unquoted date that does not exist, and duplicate
keys, which PyYAML otherwise overwrites silently) or whose containers do not have the form the
checks walk: a container with the wrong type or with an empty value (`parameters:` without a
list, `responses:` without a mapping, `tags:` without a list) and a path entry that points only
through `$ref` to `components.pathItems` (its operations would otherwise meet no check and no
table; they stand directly under `paths`). For `api-catalogue-sync` the other sub-checks then do
not run. For `api-tables` such a finding in `actions.yaml`, `openapi.yaml` or `asyncapi.yaml`
stops all blocks; if 06 or `webhooks.md` is unreadable, the rule keeps checking the other of the
two files. Recursive YAML aliases do not loop.

## Rule `api-catalogue-sync` (G2)

| Sub-check | Holds |
|---|---|
| `rest-action` | every operation under `paths`: `x-authz-action` is exactly one known action, no system action; `x-rate-limit-class` is the class of this action |
| `webhook-events` | every entry under `webhooks`: the key is an event type of the catalogue, `x-receive-action` equals its receive action, `x-authz-action` equals `webhook_out.deliver`, `x-rate-limit-class` equals `none`, and the schema of the body has exactly this type (`type` with an `enum` of this one value) |
| `ws-ops` | the operations at the channel `gateway` are exactly the ops of the catalogue: a message is named like the op, the operation with many messages is `dispatch`; `receive` fits a direction ending in `_to_server`, `send` fits `server_to_client`; action and class as in the catalogue; every frame carries exactly its op in `payload.properties.op.enum` (`dispatch` for the events), also one with `contentType: application/octet-stream`, because the gateway knows only JSON frames; every message under `messages` of an operation stands there as a `$ref`, as AsyncAPI 3 requires, and one written inline (also with `$ref: null`) is a finding; the check does not look at `reply.messages` for this |
| `channel-ops` | the operations at `terminalStream`, `hostControl` and `hostData` are exactly the `channel_ops` of the catalogue (channel by name, `terminal_stream` is called `terminalStream` in asyncapi.yaml); `receive` fits `device_to_server`, `host_to_server` or `both`, `send` fits `server_to_device`, `server_to_host` or `both`; exactly one operation per channel, op and direction; every frame carries exactly its op in `payload.properties.op.enum`, except only a binary frame (`contentType: application/octet-stream`), which names no `op`; every message under `messages` of an operation stands there as a `$ref`; no other channel has an operation, and every channel reference resolves |
| `events` | the messages with `x-event-kind` (under `components.messages` and written directly in a channel) are exactly the event types of the catalogue (as a set, each type once), each with kind, receive action and `x-audience: self` exactly for the audience `self`; `dispatch` lists exactly these messages, each once; the schema behind `payload.properties.d` has exactly the type of the message; the union `Event` in openapi.yaml names exactly the durable types in `oneOf` and `discriminator.mapping`, each once and with its schema; a message with `x-receive-action` or `x-audience` without `x-event-kind` is a finding |
| `close-codes` | `channels.gateway.x-close-codes` names the same pairs of code and name in the same order as the table in 04 "Close codes"; a terminal channel uses a code below 4100 (below the own range of the terminal channels, which starts at 4100) only with the name from this table and each pair only once; `client_action` is not compared (04 and asyncapi.yaml word the client reaction independently) |
| `idempotency` | every POST and PATCH operation has exactly one of both: a header parameter `Idempotency-Key` (at path or operation level, directly or through `$ref`, name compared case-insensitively) or `x-idempotency-exempt` with a known reason; no other method, no path entry and no webhook carries `x-idempotency-exempt`, also not with an empty value |
| `deprecation` | in both files, every `deprecated: true` has the fields `x-deprecated-since` (SemVer), `x-deprecated-at` and `x-sunset` (each `YYYY-MM-DD`), and `x-sunset` lies at least `api.deprecation_min_notice` after `x-deprecated-at`; none of the three fields stands without `deprecated: true`. Example values (`example`, `examples`) are data and do not count; under `properties` the keys are names |
| `app-scopes` | the enum `AppScope` in openapi.yaml names exactly the scopes that the catalogue assigns to actions (05 section 2.6, without `none`), each once |
| `problem-codes` | `x-problem-codes` at the schema `Problem`: codes unique, `status` three digits, texts not empty and without German umlauts or sharp s, sorted by status, only known keys; every response in `components.responses` has `x-status`, and every reference to it stands under this status (from 500 also under `default`); every code in backticks in the description of a response stands with its status in `x-problem-codes`; every code without `router: true` is named by at least one response |

`api.deprecation_min_notice` stands in
[`docs/operations/configuration.md`](../../docs/operations/configuration.md), section
"Named values of all chapters": it is looked up by column name in every table there whose header
has the columns `Key` and `Default`. If the row is missing or appears more than once, or if the
section is missing or appears more than once, that is a finding, never a default value. A
duration is a number with at most six digits and the unit `ms`, `s`, `m`, `h` or `d`, as in
section 2 of the configuration reference.

## Rule `api-tables` (G3)

The tool owns exactly these blocks. Another block name, a missing block and a broken marker in
the two files are findings of the sub-check `markers`; a stale block is a finding with its name
as the sub-check. A marker is what `action_catalog.generated_blocks` reads
(`<!-- BEGIN GENERATED: <name> -->`). Rows have the form `| a | b |`, and an empty cell becomes
two spaces.

| File | Block | Source |
|---|---|---|
| 06 | `problems` | `x-problem-codes` in list order |
| 06 | `idempotency_exempt` | operations with `x-idempotency-exempt`, by reason in the order of the table `EXEMPTION_REASONS`, then in file order; the reason as its text |
| 06 | `endpoints` | per tag in the order of `tags` with operations: heading `####` and table, operations in file order (paths and methods as written) |
| 06 | `ws_ops` | operations at the channel `gateway` in file order; the operation with many messages as the fixed row `dispatch` |
| 06 | `events` | messages in the order of `dispatch` |
| `webhooks.md` | `webhook_events` | entries under `webhooks` in file order; receiver and scope from the catalogue |

A value that cannot become a table cell (missing, no text, contains `|` or a line break, or a
backtick in a code span), an operation without a tag or with a tag that `tags` does not name,
missing `tags`, an AsyncAPI operation whose channel reference does not resolve, and an operation
at the gateway whose message does not resolve or stands inline are findings of the block;
otherwise the row would silently drop out of the table. A line break is every place where
`str.splitlines` splits (also U+2028 and U+2029), as in the cell rule of `tools/catalog`:
`tools/catalog` reads the markers line by line too, and such a character would otherwise make the
rest of the line a line of its own, in the worst case a marker.

## What the check does not cover

- The check does not compare text outside the generated blocks of 06 and `webhooks.md` (prose, a
  hand-written second table); that stays with review.
- Whether the texts of the contracts are English, the check checks only for the three texts of
  every entry in `x-problem-codes` (`title`, `description`, `client_action`), and only that they
  are not empty and contain no German umlauts or sharp s. It does not check the language of other
  texts, such as `summary` and `description` of the operations or `client_action` of the close
  codes in asyncapi.yaml.
- Whether a reason in `x-idempotency-exempt` fits in content; the check knows only the allowed
  codes.
- A schema reference that the check follows (`payload.properties.d` of an event message, the
  schema of the body of a webhook, `discriminator.mapping` of the union `Event`) is always
  resolved in `docs/api/openapi.yaml`: a local reference (`#/...`, also in asyncapi.yaml) as well
  as a reference to any file named `openapi.yaml`, in whatever directory or under whatever URL. A
  reference to a file of another name is a finding. The check does not check whether a reference
  really means `docs/api/openapi.yaml`; the lint (G1) checks that it resolves. The check resolves
  no other external `$ref`; for the columns of the tables only the last segment of a reference
  counts.
- The check compares close codes from 4100 on the terminal channels with nothing; their source is
  chapter 24, which is not one of the inputs.
- A comment that only resembles a marker (such as `<!--BEGIN GENERATED: x-->` without spaces) is
  an ordinary comment for the check, as for `tools/catalog`; it does not hide a block of its own,
  because its markers are then missing or do not match.
- The check does not walk operations under `callbacks`; the contract has none.
- A catalogue row with the direction `both` counts as covered with an operation in one of the two
  directions. The catalogue cannot express the direction per op (the `both` row of `host_data`
  contains `hello` and `identify`, which run in one direction only); if, for example,
  `terminal.send.frame` is missing, the check does not see it.
- The check does not know further copies of catalogue values in the schemas (beyond `AppScope`,
  the union `Event` and the `type` and `op` enums); a new copy needs its own sub-check.
- `api.deprecation_min_notice` is looked up only in the section "Named values of all chapters"; the
  check does not see a second row in sections 6 or 7 of the configuration reference (other
  sources).
- `make api-tables` writes every file it changes to a temporary file next to it first and
  replaces the targets only when all are complete. A read-only file, a full disk or a size limit
  stops the run, and no file is changed. Only if the rename itself fails can the first file
  already be new; the message names it (`rewrote only ...`), and the next run of
  `make api-checks` reports the other one as stale. If one of the files is a symbolic link,
  `make api-tables` replaces the link with an ordinary file.
- No server reads the fields `x-problem-codes`, `x-status`, `x-idempotency-exempt` and
  `x-deprecated-at` before M1-17 and M1-18; `catalog-check` (M1-02) holds the lists in
  `docs/api/redocly.yaml`.
