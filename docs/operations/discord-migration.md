# Migration from Discord

Status: draft M8. Guide to [22-import-migration.md](../spec/22-import-migration.md) 13 for the
owner of an installation that moves an existing Discord server with agent bots to Bothy, channel
by channel, without loss, without duplicates and without a big bang. The tool is
`bothy import` ([importtool README](../../services/server/cmd/bothy/importtool/README.md)); the
owner steps that need step-up happen in the web console, page "Import from Discord".

Every step below says how to check it and how to go back. Values in angle brackets are yours to
fill in (`<guild_id>`, `<channel_id>`); the guide holds no values of any installation. The
installation's own runbook (its mapping file, the export of its session references, its dates)
stays with the installation, not in this repository.

The commands of this guide are rehearsed: the test `TestMigrationRehearsal`
(`services/server/test/importtool/rehearsal_test.go`) runs them literally, in this order,
against a fake Discord server and a real Bothy server, and checks the end state of section 10.

Overview:

| Step | Where | Leads afterwards |
|---|---|---|
| 1. Prerequisites | installation | Discord |
| 2. Import bot | Discord developer portal | Discord |
| 3. Login of the tool | tool, app or console | Discord |
| 4. Mapping and dry runs | tool | Discord |
| 5. Source | console | Discord |
| 6. Pilot | tool | Discord |
| 7. Full import and parallel operation | tool, read mirror | Discord |
| 8. Switch-over per channel | Discord, tool, console | Bothy in the switched channel |
| 9. Revert of a channel | console, tool, Discord | Discord in that channel |
| 10. Completion | console | Bothy everywhere |

## 1. Prerequisites

| Item | Requirement | Check |
|---|---|---|
| Installation | a released version that passed the portability test of its release (PLAN row M7-14), installed per [install-compose.md](install-compose.md) | version in the console; `/readyz` |
| Status page | the place to announce the migration window and each switch-over (PLAN row M7-21) | a maintenance entry is visible in the apps |
| Apps | the mobile apps proven on devices (PLAN row M5-11), so people can follow channels after their switch-over | the people who work in the first pilot channels are signed in |
| Backup | a successful backup and a restore test before the first `--apply` ([backup-restore.md](backup-restore.md) sections 2 and 4) | the restore test of the day is green |
| Owner | an owner account with a passkey in the app or the web console; every console step asks for step-up | the console shows the page "Import from Discord" |
| Agents | every agent that answers in Discord today exists in Bothy with the same role, and its runtime can reach Bothy | the agent is listed under Agents |
| Workstation | the binary `bothy` of the same version as the server | `bothy import --help` |

Cost: none beyond the existing installation. A Discord bot application with read permissions is
free, and the tool needs no paid plan or service.

Going back: nothing has happened yet.

## 2. The import bot in Discord

The import uses its own Discord bot, never the token of a productive agent bot (22 4.2): Discord
counts rate limits per bot, so a shared token would slow down the answers of the running agents,
and the import bot needs only read permissions.

1. In the Discord developer portal create an application with a bot. Enable the privileged
   intent `MESSAGE_CONTENT`; without it Discord returns empty message content also over REST, and
   the dry run aborts (exit `2`, with a preflight message on the missing intent).
2. Invite the bot to the Discord server with the channel permissions `VIEW_CHANNEL` and
   `READ_MESSAGE_HISTORY`. Add `MANAGE_THREADS` only if archived private threads are to be taken
   over; without it the report notes `403 on archived private threads` and skips them.
3. Keep the token in your password manager. The tool reads it from the variable
   `BOTHY_IMPORT_DISCORD_TOKEN` or from a file named by `--discord-token-file` that only your user
   can read (mode `0600`), never from the command line:

```sh
read -rs BOTHY_IMPORT_DISCORD_TOKEN && export BOTHY_IMPORT_DISCORD_TOKEN
```

Check: the bot appears in the member list of the Discord server and can see every channel that is
to be imported. The tool never writes to Discord: its client has no method other than `GET`.

Going back: remove the bot from the Discord server and delete the application.

## 3. Login of the tool

The tool logs in as a device with platform `cli` through device pairing (22 10.1):

<!-- rehearsal: login -->
```sh
bothy import login --server <server_url>
```

It prints a pairing code. In the app or the web console choose "Add device" under Devices and enter
the code; that step asks for step-up. The tool polls every `import.login_poll_interval` and
then stores its session file (mode `0600`, `--session-file` names another path).

Check: the device list shows the new device. Going back:
`bothy import logout`, or revoke the device in the device list.

## 4. Mapping file and dry runs

Write the skeleton of the mapping file. `init-map` reads the Discord server like a dry run and
writes only this file; it never overwrites an existing one:

<!-- rehearsal: init-map -->
```sh
bothy import discord init-map --guild <guild_id> --out mapping.yaml
```

Edit `mapping.yaml` (schema of 22 5.2):

- the target of every category and channel: `create` with a name, `existing` with the ID of a
  space or channel, or `skip`;
- the `planned_operator` of each channel: the agent that answers there today, `agent:<handle>`;
- `defaults.uncategorized_space` for channels without a category (otherwise they are skipped);
- the authors: people with an account as `user:<handle>`, the Discord bots of your agents as
  `agent:<handle>`, everybody else stays `placeholder` (a disabled person without login, 22 5.3);
- `lead: discord` everywhere; the lead changes only in the console (section 8).

Voice and stage channels appear as comments; they are never imported. Then run the dry run over
the whole Discord server. It only reads, writes nothing (not even a local file, apart from a
refreshed session file) and prints the check report:

<!-- rehearsal: dry-run -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml
```

Correct the mapping file and repeat until the dry run ends with exit `0`. An error in the file
(unknown field, unknown handle, an ID named twice) ends with exit `2` and names its line.

The check report (22 11) has one row per channel and a total; it never shows message content or
names of people. A dry run says "would take over":

| Column | Meaning |
|---|---|
| Threads open/done | threads that stay open, and threads that become done (archived in Discord and not active, or a tag of `import.discord.closed_tags`) |
| Messages | taken over, already present, skipped (by type, empty, a thread starter already in the main thread, an author the mapping skips), failed, held back by the settle rule |
| Attachments | taken over, too large (above `import.attachment_max_bytes`: the message gets a line with name, size and the jump link to Discord instead), failed |
| People | authors mapped to accounts, and placeholders |
| Sessions | session references of the sessions file taken over, already present, discarded (the agent is not the planned operator), late (the thread had a session since the import) |
| Pins, Tags | pins and forum tags taken over |
| Reactions not taken | reactions are never taken over (22 6.6) |
| Notes | `permission_overwrites present` (set grants deliberately, section 5), `403` (the bot cannot read a channel), `changed at the source`, `written in Discord after the switch-over` |

Going back: nothing was written; delete `mapping.yaml`.

## 5. The source in the web console

1. Web console, "Import from Discord", "Add source": a name and `mapping.yaml`. The console checks
   the file and lists every error at its place.
2. Start the source ("Start", step-up). Only a started source accepts `--apply`.
3. Grants: imported channels get no grants and are visible only to owners (22 5.1). Grant the
   people and agents of each channel deliberately, at the latest before its switch-over: the
   people `trigger` (or `read`) on its space or channel, its operator agent `write` on the channel.

Check: the source shows "Started", and its channel mapping lists every channel as "Not imported".
Going back: "Cancel import" ends the source; the tool can no longer write into it.

## 6. Pilot

Take over a few channels first (22 13.2 step 2), with the session references of their threads.
The sessions file `sessions.jsonl` holds one JSON line per thread whose agent session is to
continue (22 9): `discord_thread`, `agent` (handle), `external_session_ref`, and for runtimes that
report cumulative costs `cost_baseline_micros` and `currency`, so earlier costs are not booked
twice. The Claude Code runner needs no baseline: it reads the session's totals from the session
transcript before the first resumed turn (09 18.7); a baseline in the file only applies when the
transcript has none. The references stay opaque; no output carries one.

<!-- rehearsal: pilot -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml --apply --channel <channel_id> --channel <second_channel_id> --sessions sessions.jsonl --report json > pilot-report.json
```

Then turn the read mirror on for the pilot channels. During the pilot, prefer scheduled single
rounds narrowed to the pilot channels; a `--follow` mirror would have to be stopped before the full
import of section 7 (see "One writing run at a time" there). One round is:

<!-- rehearsal: pilot-mirror -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml --apply --once --channel <channel_id> --channel <second_channel_id>
```

Check: both commands end with exit `0`; the counts of `pilot-report.json` per channel match what
Discord shows (spot checks of section 7); the pilot channels appear in Bothy read-only with their
history, and the people of the pilot can read them after the grants of section 5.

Going back: the imported channels stay read-only and do no harm; Discord still leads. A repeated
run never copies a posted item twice (22 7).

## 7. Full import and parallel operation

Import the whole server. A run resumes at the cursors where an earlier one stopped, so the pilot
channels are not taken again:

<!-- rehearsal: import -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml --apply --sessions sessions.jsonl --report json > import-report.json
```

Then run the read mirror for the whole parallel operation (22 12), without `--channel`. Discord
leads in every channel; Bothy shows the history read-only and copies new Discord messages in this
direction only. Run it as a long-running process of the installation (a service or a container
with a restart policy; give it the bot token through its environment or `--discord-token-file`):

<!-- rehearsal: follow -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml --apply --follow
```

or as scheduled single rounds. A scheduled round that finds another one still running ends with
exit `3`; that is normal:

<!-- rehearsal: mirror -->
```sh
bothy import discord --guild <guild_id> --map mapping.yaml --apply --once
```

One writing run at a time: every `--apply` run, every mirror round and `switch` takes the lease of
the source (22 7.4), and a second writing run of the same source ends with exit `3` while the lease
is younger than `import.lease_stale_after`. A `--follow` mirror holds the lease for as long as it
runs, so stop it (Ctrl-C or SIGTERM, which a stopped service sends; it finishes the current batch
and frees the lease; a forced kill keeps the lease until `import.lease_stale_after`) before the
full import or a `switch`, and start it again afterwards. A scheduled `--once` round
holds the lease only while it runs; a command that meets one ends with exit `3` and is simply run
again. `--takeover` is only for the lease of a run that crashed.

`--follow` starts a round every `import.follow.poll_interval`; after failed rounds the wait grows
up to `import.follow.max_backoff`. A round does not copy a message that an agent is still working
on (a reaction of `import.follow.busy_reactions`) or that changed less than
`import.follow.settle_after` ago; a later round copies it once it is settled (22 12.3).

Spot checks during parallel operation, per channel:

- the messages of a channel taken over and skipped, summed over the reports of its runs (pilot,
  full import, mirror rounds), against the message count of the Discord channel and its threads;
- a few messages: the author (a person, a placeholder, the agent of a bot), the order and the time;
- attachments: within the limit as a file of the message, above it a line with the jump link;
- session references: an imported thread with a reference has its earlier session;
- the state of every channel:

<!-- rehearsal: status -->
```sh
bothy import discord status --guild <guild_id>
```

`status` prints per channel the lead, the Bothy channel ID, read-only or writable and the operator
against the planned one; `inconsistent` means the lead and the read-only flag disagree.

Check: the spot checks agree. Going back: stop the mirror; Discord still leads everywhere.

## 8. Switch-over per channel

One channel at a time, exactly in this order, so two places never lead at the same time (22 13.3).
Announce the switch-over of the channel on the status page and in the Discord channel.

Step 1, Discord, owner: set the channel read-only and remove the agent from this channel in the
Discord configuration of its runtime. The tool cannot see this step.

Step 2, tool: the last mirror round for the channel and the settle check. Stop a `--follow` mirror
first and start it again after this step (section 7, "One writing run at a time"); with a running
mirror `switch` ends with exit `3`. `--discord-locked` is your statement that step 1 is done;
without it the command ends with exit `2` before any call:

<!-- rehearsal: switch -->
```sh
bothy import discord switch --guild <guild_id> --map mapping.yaml --channel <channel_id> --discord-locked
```

Exit `1` with counts per reason (busy, unsettled, lookback) means messages are still held back:
repeat after `import.follow.settle_after`. Exit `1` with "run switch again" means messages arrived
after the round: the Discord lock of step 1 is not in place yet. Exit `0` prints steps 3 and 4.

Step 3, web console, owner (step-up): in the channel's row "Lead here", which makes the channel
writable in the same step; then "Set operator" with the planned operator of the mapping. Another
operator would leave the imported threads without their agent (R-IM-08).

Step 4, probe: write a test message in an imported thread of the channel. Exactly one answer of the
operator follows, in Bothy only; a thread with an imported session reference continues that
session (22 9).

Check: `status` shows the channel with lead `bothy`, `writable` and `operator ok`; the probe got its
answer. Going back: section 9.

## 9. Revert of a channel

Possible at any time before completion (22 13.4).

Step 1, tool: count what was written in Bothy while it led. The command only reads Bothy (no
Discord, no token):

<!-- rehearsal: revert -->
```sh
bothy import discord revert --guild <guild_id> --channel <channel_id>
```

These messages never go to Discord (R-IM-12); transfer them by hand or discard them knowingly. Run
the command again after step 2 for the final count: until then people can still write in Bothy.

Step 2, web console, owner: "Discord leads", which sets the channel read-only and cancels answers in
progress in the same step; then remove the operator.

Step 3, Discord, owner: lift read-only and add the agent there again.

Step 4: the read mirror resumes at its cursor with its next round.

Check: `status` shows the channel with lead `discord`, `read-only` and `no operator`. A later
switch-over follows section 8 again.

## 10. Completion

When every imported channel is led by Bothy:

1. End the read mirror (stop the service, remove the schedule).
2. Web console, owner: "Complete" (step-up). The confirmation warns: the mapping in
   `import_items` (retention class `import`) stays until it is deleted after
   `retention.import_after_completion` or by the owner ([20-privacy.md](../spec/20-privacy.md));
   after that deletion a repeated import is no longer idempotent and would copy everything again.
3. Remove the import bot from the Discord server (section 2) and log the tool out:
   `bothy import logout`.

Check: the source shows "Completed"; every channel row shows "Led here, operator ok". Going back:
a completed source cannot be started again, and nothing mirrors Discord any more.

## 11. Troubleshooting

| Sign | Meaning | What to do |
|---|---|---|
| exit `2`, preflight note on `MESSAGE_CONTENT` | the intent probe found only empty messages | enable the intent in the developer portal (section 2), run again |
| exit `2`, mapping error with a line | the file breaks the schema, a handle does not exist, an ID is named twice | correct the line |
| exit `2` on `--apply`, no running source | no started source of this Discord server, or its stored mapping differs from the file apart from `lead` | start the source in the console, or add a source with the corrected file |
| exit `1`, rate limit | Discord asked for a wait longer than `import.discord.max_rate_wait` | wait, run again; the run resumes. Never use the token of an agent bot (RK-76) |
| exit `1`, `import_not_running` | the source was paused, cancelled or completed while the run wrote | start a paused source again in the console if the pause was not intended; a cancelled source cannot be started again, add a new one |
| exit `1`, network or server error | the run stopped; nothing is lost | run again; it resumes at the cursor |
| exit `3`, lease held | another writing run of this source holds the lease (section 7, "One writing run at a time") | a `--follow` mirror: stop it, run the command, start the mirror again; a scheduled round: run the command again; a run that crashed: wait `import.lease_stale_after` or use `--takeover` |
| note `403: the channel is not readable` | the bot lacks `VIEW_CHANNEL` or `READ_MESSAGE_HISTORY` there; the run avoids the channel | fix the permission in Discord, run again |
| note `403 on archived private threads` | the bot lacks `MANAGE_THREADS` | add it if private threads matter |
| note `written in Discord after the switch-over` | Discord shows new messages in a channel Bothy leads; nothing is copied | lock the Discord channel (section 8 step 1); transfer the messages by hand |
| `status` shows `inconsistent` | lead and read-only flag disagree | set the lead in the console again |

Risks that the steps contain: a missing intent (preflight abort, RK-73), half-finished agent
outputs (settle rule, RK-74), costs booked twice for continued sessions (cost baseline, RK-75),
an address block after many error responses (rate-limit guard and the separate bot, RK-76); see
[RISKS.md](../RISKS.md).
