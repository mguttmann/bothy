# Template: record of processing activities (Art. 30 GDPR)

Status: template, activities as built up to M8 and A1 (built-in agents). For instance operators.
Not legal advice. Section 4 names what the template does not cover.

How to use the template:

1. Copy the file into your own documentation (do not maintain it in the product repository).
2. Replace everything in `<angle brackets>`. Prefilled information describes what the product does;
   check it against your configuration (agents, apps, telemetry, push).
3. Update the record on every change to agents, apps, providers or periods.

Obligation and exemption: Art. 30(5) GDPR exempts organizations with fewer than 250 employees.
The exemption does not apply if the processing involves a risk, is not only occasional
or concerns special categories or data about criminal offenses (https://gdpr-info.eu/art-30-gdpr/,
retrieved 2026-10-10 [verified]). A team chat processes data continuously; the exemption therefore
usually does not apply [derived].

Basis of the contents: [../spec/20-privacy.md](../spec/20-privacy.md) (data categories per
table, periods), [../spec/19-security.md](../spec/19-security.md) (technical measures),
[processors-template.md](processors-template.md) (recipients and processors).

## 1. Controller (Art. 30(1)(a))

| Field | Information |
|---|---|
| Name | `<organization>` |
| Address | `<address>` |
| Contact | `<email or phone of the controller>` |
| Representative in the EU (if needed) | `<...>` |
| Data protection officer | `<name, contact or "not appointed">` |
| Joint controllers | `<none or name>` |
| Record as of | `<date>` |
| Product version | `<version of the instance>` |

## 2. Activities

Every activity has the mandatory fields from Art. 30(1)(b) to (g) GDPR.

### V1 Team chat

| Field | Information |
|---|---|
| Purpose (b) | internal communication in spaces, channels and threads; sharing files and voice messages; search |
| Legal basis | `<to be filled in by the instance operator>` |
| Data subjects (c) | employees and other members of the instance; third parties who appear in contents |
| Data categories (c) | master data (handle, display name, avatar, email for display), contents (messages, files, transcripts, reactions, mentions), communication metadata (times, thread, channel, DM participants), personal settings (read state, drafts, quiet hours). Contents can include special categories under Art. 9: `<assessment of the instance operator>` |
| Recipients (d) | members with a grant on the respective channel; owners (except direct messages); hosting `<provider or own operation>`; object storage `<...>` |
| Third country (e) | `<none, or country and safeguard>` |
| Deletion periods (f) | contents `retention.content` = `<value>`; deletion by people at any time; accounts pseudonymized after a privacy request (`privacy.account_deletion_content` = `<value>`); event log `sync.event_retention` = `<value>`; backups `<backup period>` |
| Technical and organizational measures (g) | see section 3, references M1 to M8 |

### V2 Agent processing

| Field | Information |
|---|---|
| Purpose | tasks performed by AI agents in channels and threads (answers, tools, approval cards) |
| Legal basis | `<...>` |
| Data subjects | people who trigger agents; people whose messages are in the context of a run; third parties in contents |
| Data categories | contents of the thread and up to `history_backfill_limit` earlier messages, files of the thread, author as handle and kind, channel prompt, run metrics, costs |
| Recipients | enter per agent: runtime `<name, run by, location>`, LLM provider `<name>`, tools with external effect `<list>`; name agents with tool class `host` or `scoped` separately |
| Third country | `<per LLM provider; region of processing>` |
| Deletion periods | in the product as in V1; at the runtime and the LLM provider according to their periods `<enter>` |
| Measures | M2, M3, M6; agent badge (20, section 8); the one-time AI notice of 20 section 8 is not built, enter your own notice `<...>` |

An agent with `billing_mode = personal` runs on the subscription of its `credential_holder`
([../spec/10-backend-policy.md](../spec/10-backend-policy.md) 1). Only that person triggers it, but
the context of a run holds the messages of the other people of the thread, and they reach the
LLM provider under the consumer terms of that subscription: no processing contract under Art. 28,
the provider's own retention and training rules. Name such agents in the table below and assess
this flow `<...>`.

Table of agents (one row per agent):

| Agent | Served spaces and channels | Tool class | Backend policy | Runtime | LLM provider, region | Memory across runs |
|---|---|---|---|---|---|---|
| `<name>` | `<...>` | `<chat, scoped, host>` | `<personal, organization>` | `<...>` | `<...>` | `<yes, no, unknown>` |

### V3 Login, devices and security log

| Field | Information |
|---|---|
| Purpose | access protection, device binding, evidence of administrative actions and denials |
| Legal basis | `<...>` |
| Data subjects | members, owners |
| Data categories | identities at the provider (subject), devices (name, platform, public keys), sessions (times, login method, last IP), audit log (actor ID, action, decision, IP only as a pseudonym) |
| Recipients | owners; identity provider `<name>` |
| Third country | `<...>` |
| Deletion periods | security data `retention.security_after_end` = `<value>`; audit log `retention.audit` = `<value>`; IP pseudonym key destroyed after `privacy.audit_ip_key_retention` = `<value>` |
| Measures | M1, M4, M5 |

### V4 Push notifications

| Field | Information |
|---|---|
| Purpose | notice of new messages on mobile devices and desktop |
| Legal basis | `<...>` |
| Data subjects | members with registered devices |
| Data categories | push token of the device, app identifier, IDs of message and thread, neutral placeholder text, time. No contents. |
| Recipients | `<Apple Push Notification service, Firebase Cloud Messaging, if needed Windows Push Notification Services>` |
| Third country | `<...>` |
| Deletion periods | registration together with the device or account; at the provider according to its periods |
| Measures | M6 |

### V5 Operations and telemetry

| Field | Information |
|---|---|
| Purpose | stability, debugging, capacity, status page |
| Legal basis | `<...>` |
| Data subjects | members (pseudonymous) |
| Data categories | metrics, traces and logs without contents; principal IDs only as rotating pseudonyms; no IP addresses |
| Recipients | telemetry backend `<name or own operation>`; crash reports of the apps `<provider or "off">` |
| Third country | `<...>` |
| Deletion periods | `<period in the backend>`; pseudonym keys rotate after `privacy.telemetry_pseudonym_rotation` = `<value>` |
| Measures | M7 |

### V6 Cost tracking for agents

| Field | Information |
|---|---|
| Purpose | attribution and reconciliation of agent costs per run, thread, space, agent |
| Legal basis | `<...>` |
| Data subjects | people who trigger runs |
| Data categories | cost entries per run, model, token counts; attributable to a person through the run |
| Recipients | owners; space administration for its own spaces |
| Deletion periods | `retention.cost` = `<value>` |
| Measures | M2; statistics per person off by default (20, section 9) |

### V7 Import from earlier systems

| Field | Information |
|---|---|
| Purpose | transfer of earlier histories `<for example from a Discord server>` |
| Legal basis | `<...>` |
| Data subjects | people of the source, including those without an account in the instance |
| Data categories | imported messages, files, names from the source, origin keys with identifiers of the source |
| Recipients | as in V1 |
| Deletion periods | import state `retention.import_after_completion` = `<value>`; contents as in V1 |
| Measures | as in V1; the import only reads from the source ([../spec/22-import-migration.md](../spec/22-import-migration.md)) |

### V8 Data subject requests

| Field | Information |
|---|---|
| Purpose | fulfillment of the rights under Art. 15 to 22 GDPR |
| Legal basis | Art. 6(1)(c) GDPR |
| Data subjects | requesting persons |
| Data categories | request, status, export package |
| Recipients | owners, the data subject |
| Deletion periods | export packages `privacy.export_ttl` = `<value>`; requests `retention.privacy_job` = `<value>` |
| Measures | step-up on download, time limit, audit |

### V9 Backups

| Field | Information |
|---|---|
| Purpose | restore after an outage |
| Legal basis | `<...>` |
| Data subjects | all from V1 to V8 |
| Data categories | image of the database and the object storage, encrypted |
| Recipients | backup target `<provider or location>` |
| Deletion periods | `<backup period>`; deleted data does not come back on a restore (deletion journal, 20 section 4.6) |
| Measures | M8 |

### V10 Terminal

| Field | Information |
|---|---|
| Purpose | shell access of people to connected hosts through the server ([../spec/24-terminal.md](../spec/24-terminal.md)) |
| Legal basis | `<...>` |
| Data subjects | people with terminal access, host assignees, holders of an LLM login on a host |
| Data categories | hosts (host name, assignee, operating system, public keys, names of the session users), assignments of session users, terminal sessions (who, when, from which device, on which host, profile, duration, bytes per direction, reason for the end); never terminal contents |
| Recipients | owners; the person themselves; the host where the session runs `<host operator>` |
| Deletion periods | class `security`: `retention.security_after_end` = `<value>` after the end of the host or session |
| Measures | M1, M2, M5; no content recording (20, R-DP-07); terminal contents stay on the host and in the memory of the host service |

### V11 Search assistant

| Field | Information |
|---|---|
| Purpose | answer a search question in the person's own words with a short list of threads ([../spec/14-search.md](../spec/14-search.md) 4.10) |
| Legal basis | `<...>` |
| Data subjects | the asking person; authors and people named in the candidate threads |
| Data categories | the question and earlier questions of the popup, per candidate thread the title, channel and space name, time and an excerpt; only candidates the assistant agent may read itself |
| Recipients | the agent named in `search.assist.agent_id`, its runtime and LLM provider `<name, region>` |
| Deletion periods | the server stores and logs no question, term or candidate; at the runtime and the LLM provider according to their periods `<enter>` |
| Measures | M2 (the agent's own grants filter the candidates); off while `search.assist.agent_id` is empty |

### V12 Built-in agents and the interview

| Field | Information |
|---|---|
| Purpose | create and run agents defined in the product, with the interview of "New agent" ([../spec/26-builtin-agents.md](../spec/26-builtin-agents.md)) |
| Legal basis | `<...>` |
| Data subjects | owners who run the interview; people whose messages reach the agents (as in V2) |
| Data categories | the answers of the interview and the draft of the definition; the definition (instructions, skills, settings) |
| Recipients | the builder agent named in `agents.builder_agent_id`, its runtime and LLM provider `<name, region>`; the runner that hosts the agents |
| Deletion periods | the server stores no interview; definitions with the agent `<...>` |
| Measures | M2; the interview is for owners only and audited without its contents |

### V13 Apps and integrations

One activity or one recipient entry per installed app. The reference apps of the repository:

| App | What leaves the instance | Recipient |
|---|---|---|
| `github-app` | the form of `/issue` (title, text) and, with mirroring, replies of people in a linked post as comments | GitHub `<account, region>` |
| `jira-app` | the form of `/issue` and, with mirroring, replies of people in a linked post as comments | Atlassian Jira Cloud `<site, region>` |
| `cost-reconciler` | nothing personal: it reads the provider's daily cost report with the admin key and submits amounts per run to the server | the LLM provider (read only) |

Autonomous runs that wake an agent without a message (stage AR) are not built (AR-05); they add
no activity yet.

`<further activities>`

## 3. Technical and organizational measures (Art. 30(1)(g), Art. 32)

Prefilled with product measures; add the operations side (host, network, staff).

| No. | Measure | Product | Operations `<add>` |
|---|---|---|---|
| M1 | System access control | no self-registration, device confirmation, device-bound tokens (DPoP), step-up for administration ([../spec/08-identity.md](../spec/08-identity.md)) | `<identity provider, mandatory MFA>` |
| M2 | Data access control | grants per space and channel, a single decision point, guard rules, direct messages protected even from owners ([../spec/07-permissions.md](../spec/07-permissions.md)) | `<grant process, review>` |
| M3 | Separation | tool classes, dev environments per person with a VM boundary ([../spec/21-dev-isolation.md](../spec/21-dev-isolation.md)) | `<...>` |
| M4 | Transport encryption | TLS, HSTS, no plaintext connections from outside ([../spec/19-security.md](../spec/19-security.md)) | `<certificates, proxy>` |
| M5 | Logging | hash-chained audit log, IP pseudonymized, separate from telemetry | `<check interval of the hash chain>` |
| M6 | Data minimization | push without content, no link preview, no external resources in clients, metadata removed from images | `<...>` |
| M7 | Pseudonymization | telemetry without contents with rotating pseudonyms | `<...>` |
| M8 | Availability and recovery | encrypted backups, deletion journal, restore drill | `<backup target, drill schedule>` |
| M9 | Encryption of data at rest | tokens only as hashes, secrets only in the secret provider; local caches encrypted | `<disk encryption of the host>` |
| M10 | Testing of effectiveness | tests per control, red team per milestone, external test before publication | `<...>` |

## 4. Not covered by this template

The template is the record of Art. 30. It holds no framework for a data protection impact
assessment (Art. 35, which [../spec/20-privacy.md](../spec/20-privacy.md) section 9 names), no
procedure for a personal data breach (Art. 33 and 34) and no procedure for the rights to
restriction and to object (Art. 18 and 21; the product builds access, export, rectification and
deletion, [../spec/20-privacy.md](../spec/20-privacy.md) section 5). The instance operator keeps
these in their own documentation; templates for them are plan row V3-48.
