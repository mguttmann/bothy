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
| `bothy import discord status --guild ID` | prints the source, its state, per mapped channel the lead, the Bothy channel ID, its read-only state and its operator against the planned one (flagging a lead that disagrees with the read-only flag as `inconsistent`), and the counters; no names |
| `bothy import discord switch --guild ID --map FILE --channel ID --discord-locked` | last mirror round and settle check of the switch-over of one channel; prints the owner's steps in the web console (section "Migration per channel") |
| `bothy import discord revert --guild ID --channel ID` | only reads: the messages written in Bothy per thread of the channel and the owner's steps of reverting |

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
| `--sessions FILE` | earlier agent sessions of the threads (22 9), see "Sessions file" |

An unknown or non-matching ID in `--channel` or `--only-thread` ends the run before any write,
also for the valid IDs of the same call. `--takeover` without `--apply`, another `--report` value,
an ID that is not a Discord ID, `--follow` with `--once` and `--only-thread` with `--follow` end
with exit `2` without any call (22 10.2 rule 3).

## Sessions file (22 9)

`--sessions FILE` lets imported threads continue the agent session they had before the migration.
The file holds JSON lines, one object per line (blank lines are skipped):

```json
{"discord_thread": "13001001", "agent": "lotse", "external_session_ref": "sess-7f3a", "cost_baseline_micros": 1500000, "currency": "USD"}
```

- Members exactly `discord_thread` (digits), `agent` (the handle of an agent of the server) and
  `external_session_ref` (non-empty display text of at most the largest
  `agent.session_ref_max_bytes`), optionally `cost_baseline_micros` (an integer from 0, the amount
  the old session already booked) and `currency` (ISO 4217, required when the baseline is above 0).
  An unknown member, a bad value, a `discord_thread` named twice or an unknown agent ends the run
  with exit `2` before any write; the message names the line, never the reference.
- A line whose agent is not the `planned_operator` of the thread's channel is dropped and counted
  as discarded; a line whose thread is not part of the run (another channel, `--channel`,
  `--only-thread`) is ignored and counted in a note.
- An accepted line travels with the thread item as `earlier_session`; the server writes the earlier
  session row exactly once per thread, agent and reference, also for a thread an earlier run posted,
  and never after the thread has had a current session. The column "Sessions" of the report counts
  taken over, already present, discarded and late lines (`sessions_taken`, `sessions_present`,
  `sessions_discarded`, `sessions_late`); a dry run validates and counts the accepted lines as
  "would take over" without writing.
- The references are opaque: no output, report or log line carries one.

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
paused or draft source makes the round a no-op with a note (a mirror may also start on one and
takes the lease in its first round on a running source); a completed source ends the mirror with exit
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

## Migration per channel (22 13)

The migration moves one channel at a time, in an order in which Discord and Bothy never lead at
the same time (22 12.1: the stored `lead` of the ownership list and the read-only flag of the
Bothy channel change in one transaction, and the mirror copies only channels led by Discord).

Switch-over (22 13.3):

1. Discord, owner: set the channel read-only and remove the agent from this channel in the Discord
   configuration of its runtime. The tool cannot see this step.
2. Tool: `bothy import discord switch --guild ID --map FILE --channel ID --discord-locked`. The
   flag `--discord-locked` confirms step 1; without it the command ends with exit `2` before any
   call. The channel must be imported (its channel item posted), led by Discord, and the source
   running, else exit `2`. The command runs one mirror round for the channel (with the lease of a
   run, exit `3` and `--takeover` as for `--once`), then reads the channel once more and applies
   the settle rule with the current time: anything still held back ends with exit `1` and the
   counts per reason (busy, unsettled, lookback_busy); repeat after `import.follow.settle_after`.
   Messages after the cursor that the rule does not hold but the round did not copy (written after
   it) also end with exit `1`: the Discord lock of step 1 is evidently not in place yet. On success
   standard error carries steps 3 and 4.
3. Web console, owner (step-up): Import, the channel, "Bothy leads", which lifts read-only in the
   same transaction; then the channel's operator, the planned operator of the mapping (the
   imported threads carry it, R-IM-08).
4. Probe: write in an imported thread; the agent answers in Bothy only, and an imported session
   reference binds on this first trigger (22 9).

Revert (22 13.4):

1. `bothy import discord revert --guild ID --channel ID` only reads (no lease, no Discord, no
   token): it prints per Bothy thread of the channel the messages written in Bothy (origin other
   than `import`, kind other than `system`, not deleted) and the total. They never go to Discord
   (R-IM-12); the owner transfers or discards them.
2. Web console, owner: "Discord leads", which sets read-only and cancels the active runs of the
   channel in the same transaction; then remove the channel's operator.
3. Discord, owner: lift read-only and add the agent there again.
4. The read mirror resumes at the cursor with its next round.

`status` shows the state of every channel during the migration; `inconsistent` means the lead and
the read-only flag disagree.

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
the lease, the pause, imported messages never trigger, the read mirror, the switch-over and the
revert with the probe) run against the harness in
[`test/importtool`](../../../test/importtool).
