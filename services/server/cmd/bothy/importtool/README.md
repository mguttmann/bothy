# importtool: `bothy import discord`

The import tool takes the history of a Discord server over into Bothy
([docs/spec/22-import-migration.md](../../../../../docs/spec/22-import-migration.md), ADR 0043). It runs
outside the server: it reads Discord through the read-only client of [`discord`](discord/README.md)
and writes into Bothy only through the public import interface with the session of
`bothy import login` ([`bothyapi`](bothyapi/README.md)). The server never calls Discord.

## Commands

| Command | Effect |
|---|---|
| `bothy import discord --guild ID --map FILE` | dry run: preflight, plan and check report; only `GET` against Discord and Bothy |
| `... --apply` | imports into the `running` source of this server that the owner created and started in the web console |
| `... --apply --follow` | read mirror: a round every `import.follow.poll_interval` (section "Read mirror"); without `--apply` one reading round per interval that reports what it would copy |
| `... --apply --once` | exactly one round of the read mirror, for scheduled calls; without `--apply` one dry round |
| `bothy import discord init-map --guild ID --out FILE` | writes a skeleton of the mapping file (only this file; it never overwrites one) |
| `bothy import discord status --guild ID` | prints the source, its state, the ownership list (Discord channel ID and lead) and the counters; no names |

Options of a run:

| Option | Meaning |
|---|---|
| `--channel ID` (repeatable) | only these mapped channels, with their threads and structure |
| `--only-thread ID` (repeatable) | exactly these threads of mapped channels, with the structure of their channels |
| `--report text\|json` | form of the report on standard output (default `text`) |
| `--takeover` | with `--apply`: take the run lease of another run over (22 7.4) |
| `--discord-token-file PATH` | bot token from a file (mode `0600`); else `BOTHY_IMPORT_DISCORD_TOKEN`; exactly one of both |
| `--config FILE` | YAML file with nested `import.*` keys (22 19) |
| `--session-file PATH` | session file of `bothy import login` instead of the default path |

An unknown or non-matching ID in `--channel` or `--only-thread` ends the run before any write,
also for the valid IDs of the same call. `--takeover` without `--apply`, another `--report` value,
an ID that is not a Discord ID, `--follow` with `--once` and `--only-thread` with `--follow` end
with exit `2` without any call (22 10.2 rule 3).

## Exit codes (22 10.3)

| Code | Meaning |
|---|---|
| `0` | done, or a dry run without errors |
| `1` | aborted: network, rate limit above the limit, server error, or the owner paused the source; the state stays and a new run resumes |
| `2` | invocation, configuration, mapping file or preflight faulty; nothing written |
| `3` | another run holds the lease of the source; use `--takeover`; with `--once` the normal case for scheduled calls |

## The dry run writes nothing

Without `--apply` the tool sends only `GET` to Discord and Bothy. The only other call is
`refreshToken` when the access token of the session expired, and the only file it may write is the
session file with the rotated refresh token (22 10.2 rule 5). It downloads no attachment and
creates no source. `TestDryRunOnlyGET` holds this with fakes that count the methods and empty
working and temporary directories.

## Flow of a run

1. Flags, then the configuration and the token, then the mapping file (strict YAML; every error
   names its line), all before any call.
2. Preflight (22 4.4): a bot token; the server, every mapped category and channel readable; the
   intent probe (`MESSAGE_CONTENT`); an active owner session; every mapped person, agent and planned
   operator active; no new space or channel name taken by an object the import did not create.
3. With `--apply`: the one open source of this server must be `running`, and its stored mapping
   must equal the file apart from the `lead` values; the leads always come from the stored mapping.
   The run takes the lease in the cursor and renews it every `import.follow.poll_interval`,
   independent of the batches.
4. Structure batches (spaces, channels, forum tags), then per channel the main thread and the
   threads by ascending ID. Before each thread batch the tool posts the user items of its authors.
   Attachments go through the CDN fetch into memory and `createImportFile` with their upload;
   one that is too large or fails gives the message a line with file name, size, reason and the
   jump link. The cursor of a thread advances only after a successful batch.
5. The report (22 11) goes to standard output; its totals are stored in `stats.tool`.

A crash between a batch and its cursor never produces a duplicate: the next run sends the batch
again, and the server answers every posted origin key with its target.

## Read mirror (22 12)

During parallel operation the mirror keeps Bothy in step with Discord, in this direction only; it
never writes to Discord. A round:

1. reads the source (state, the leads of the ownership list, which the owner changes in the web
   console, so they are read again every round), then `GET /guilds/{id}/channels` and the active
   threads;
2. reads only channels whose stored `lead` is `discord` and whose `last_message_id` (or that of
   one of their active threads) is newer than the cursor; a channel with `lead = bothy` that shows
   new messages gets the note "written in Discord after the switch-over" and nothing is copied;
3. posts new threads and the changes of known ones: a thread item is sent when Discord shows the
   name or the done state differently from the fingerprint the cursor keeps of the last posted
   item (`mirrored`, a state letter and a hash of the name, never the name); a thread the cursor
   knows as open that is missing from the active list is confirmed with `GET /channels/{id}`; the
   server renames only while `title_source = auto`; a new thread of a text channel waits until its
   starter message in the main thread is copied, so the server can link it;
4. copies per thread the settled beginning of the new messages (settle rule below) through the
   same batches, attachments and cursor writes as a full run.

Settle rule (22 12.3, `Settle` in `settle.go`): a message is busy when it, or for type 21 its
starter message, carries a reaction of `import.follow.busy_reactions` with a count above zero; it
is unsettled when the later of its time and its edit time lies less than
`import.follow.settle_after` in the past. The copy ends before the first busy or unsettled message
of a type that is taken over; skipped types never stop it. When one of the
`import.follow.busy_lookback` messages before the first new one is busy, the whole thread waits.
The report counts the messages held back (`messages_held`). Edits and deletions after the copy are
not kept in sync (22 12.4); a later full run counts them as "changed at the source".

A failed round (network, `5xx`, `401`, a rate wait above the limit) never ends `--follow`: the next
round waits `min(import.follow.max_backoff, import.follow.poll_interval * 2^failures)`. The waits
are slept in pieces of at most `import.follow.poll_interval` with a lease renewal after each, so a
long backoff never frees the lease of a living mirror. An error of one thread (a `403` or `404` of
Discord, a `4xx` of Bothy for its batch) is a note, and the round goes on with the next thread. A
paused source makes the round a no-op with a note; a completed source ends the mirror with exit
`0`, a failed one with exit `1`; SIGINT and SIGTERM finish the current batch, clear the lease and
exit `0`. The cursor writes of the mirror carry no counters, so `stats.tool` keeps the totals of
the last full run. `--follow` prints the report of a round only when it copied, held or noted
something; a dry mirror and `--once` print every round.

A scheduled mirror, for example from cron every minute; exit `3` means another mirror still runs:

```sh
bothy import discord --guild 13000001 --map mapping.yaml --apply --once --report json
```

The settle values come from the predecessor and were never measured against the real Discord
(22 19).

## Configuration (22 19)

Values come from the defaults of 22 19, the file of `--config`, and the environment
`BOTHY_IMPORT__...` (highest). Only variables with that prefix are read, so a server variable on the
same machine never disturbs the tool; the server file (`BOTHY_CONFIG_FILE`) is never read. Example:

```yaml
import:
  batch_max_items: 50
  discord:
    closed_tags: [done]
  follow:
    poll_interval: 30s
```

`import.attachment_max_bytes` without a value means `file.max_upload_bytes` of the server; a value
above that limit is lowered to it and the report says so.

## Mapping file (22 5.2)

```yaml
version: 1
guild_id: "1029384000"
defaults:
  unmapped_human: placeholder
  unmapped_bot: placeholder
  uncategorized_space: skip
spaces:
  - discord_category: "1029384001"
    target: {create: {name: "Support"}}
channels:
  - discord_channel: "1029384002"
    target: {create: {name: "tickets"}}
    planned_operator: "agent:support-bot"
    lead: discord
users:
  - discord_user: "1029384003"
    target: "user:alex"
  - discord_user: "1029384004"
    target: placeholder
```

The owner creates the source with this mapping in the web console and starts it there; the tool
never calls `import.manage`.

## Output

Standard output carries the report, standard error the errors; neither the report nor any error
or log line carries message content, a file name of an attachment, a name of a person, a token or
a signed URL (22 10.2 rule 4, `TestNoSecretInOutput`). The report names Discord channels and their
targets, which are structure names.

## Tests

Unit tests use the fake Discord and CDN of `discord/discordtest` and the fake Bothy of
`bothyapi/bothytest`; the guarantees that need the real server (counts, no duplicate after a crash,
the lease, the pause, imported messages never trigger, the read mirror) run against the harness in
[`test/importtool`](../../../test/importtool).
