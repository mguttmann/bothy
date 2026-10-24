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
also for the valid IDs of the same call. `--takeover` without `--apply`, another `--report` value
and an ID that is not a Discord ID end with exit `2` without any call.

## Exit codes (22 10.3)

| Code | Meaning |
|---|---|
| `0` | done, or a dry run without errors |
| `1` | aborted: network, rate limit above the limit, server error, or the owner paused the source; the state stays and a new run resumes |
| `2` | invocation, configuration, mapping file or preflight faulty; nothing written |
| `3` | another run holds the lease of the source; use `--takeover` |

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
the lease, the pause, imported messages never trigger) run against the harness in
[`test/importtool`](../../../test/importtool).
