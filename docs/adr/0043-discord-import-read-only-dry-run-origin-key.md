---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team Migration (I), Team API (B), Team Security and permissions (C)
informed: all teams
---

# Discord import: read only, dry run, origin key

## Context and Problem Statement

Organizations that introduce Bothy often have their history in Discord: categories, channels,
forums, threads with agent sessions, attachments. Bothy is to take over this history, switch over
channel by channel and pull in new Discord messages during a transition period
([22-import-migration.md](../spec/22-import-migration.md)).

Hard limits of the baseline apply: the server never loads external URLs (section 4, SSRF),
extensions never run in the server process (B6), every route goes through `authorize` (B7), and
imported messages must never trigger an agent. The predecessors showed two error classes
repeatedly: duplicates after unreliable delivery and a tool that writes when a switch is
forgotten (REUSE.md, lessons L02, L17, L18).

Question: Where does the import run, how does it access Discord, and how does it prevent
duplicates on abort and retry?

## Decision Drivers

- No server-side loading of external URLs; the Discord token does not belong in the server.
- A run can be aborted and resumed at any point, without duplicates.
- The dry run is the default and provably writes nothing.
- Discord is only read; the tool cannot change Discord.
- Permissions, audit and events apply as for any other write path.
- The path also suits a later second source type (Zulip).

## Considered Options

1. Separate process `bothy import discord` through an import interface of the public API;
   Discord only `GET`; idempotency through `import_items (source_id, origin_key)`.
2. Import as a background job in the server (role `worker`) that fetches from Discord itself.
3. Direct database import (a converter writes SQL bypassing the application).

## Decision Outcome

Chosen option: option 1, a separate process through the import interface.

Rulings:

1. The Discord client in the tool knows only `GET`; the list of endpoints is in chapter 22,
   section 4.1. Only the tool holds the token.
2. Without `--apply` every call is a dry run: only `GET` against Discord and Bothy, no
   file, no server state.
3. Every imported item has an origin key. The server writes `pending`, creates the
   target and sets `posted` in the same transaction; a repetition returns the existing
   target.
4. Progress is kept in the server (`import_sources.cursor`, `stats`), not in a local file.
5. Imported messages have `origin = import`, `silent = true` and no `mentions`.

### Consequences

- Good, because the server opens no connection to Discord and knows no Discord token; the
  SSRF limit of the baseline stays without an exception.
- Good, because permissions, audit and events run through the normal layers; only owners may
  import.
- Good, because an aborted run continues on any machine; the state is in the server.
- Good, because the same path carries a later Zulip import.
- Bad, because the API gets an owner interface with backdating and a different author
  (`import.apply`, 05 section 4.12). Mitigation: only owners, only for a running
  source, only with `origin = import`, start with step-up, audit.
- Bad, because a long read mirror needs a running client process. Mitigation: the
  process is stateless and can run as a separate container with a restart policy.

### Confirmation

These tests hold the decision in the repo (chapter 22, section 16):

- Dry-run test: fake Discord and fake Bothy count methods; only `GET` is allowed; temp and
  work folders stay empty.
- Static check: the Discord client exports no method with a verb other than `GET`.
- Crash test: abort between batch and cursor; a new run creates every item exactly
  once and keeps the order per thread.
- Fault injection: fake Bothy stores, then answers `502`; the repetition returns the same
  target.
- Trigger test: an imported message in a channel with an operator creates no run.
- Dependency rule according to ADR 0008: no server package imports the Discord client.

## Pros and Cons of the Options

### Option 1: Separate process through the import interface

- Good, because the server never loads external URLs and holds no third-party token.
- Good, because the dry run works without server write permission and without a local file.
- Good, because the idempotency lies in the target and not in the tool.
- Neutral, because the import throughput is limited by the API and its rate-limit class; the
  Discord limits are tighter anyway.
- Bad, because an additional, powerful owner route arises.

### Option 2: Background job in the server

- Good, because resumption and progress use the existing job infrastructure.
- Good, because no client process has to run for days.
- Bad, because the server opens outgoing connections to Discord and to the CDN; that breaks
  the baseline limit against server-side loading and needs an exception with a host list.
- Bad, because the Discord token has to go into the secret provider of the server.
- Bad, because Discord-specific code moves into the core.

### Option 3: Direct database import

- Good, because it writes fastest.
- Bad, because it bypasses `authorize`, audit, event log and permission version (B7, B8).
- Bad, because every migration of the schema breaks the converter.
- Bad, because resuming after an abort would need its own logic next to `import_items`.

## More Information

- Discord endpoints, intents and rate limits with sources: chapter 22, section 4 and Sources.
- Design of the command line and login as owner (device `platform = cli`, step-up):
  [08-identity.md](../spec/08-identity.md).
- Lessons of the predecessors (dry-run proof, origin key, settle rule, CDN fetch):
  [REUSE.md](../REUSE.md).
- Alignment with the core contract: chapter 22, section 18.
- Re-evaluate when a source type is reachable only through a server push (webhook); then
  the server needs an inbound path, but still no outgoing fetch.
