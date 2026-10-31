# 01 Product

Status: draft M0. Terms: [00-glossary.md](00-glossary.md). Data: [03-data-model.md](03-data-model.md).
Sync and offline: [04-sync-protocol.md](04-sync-protocol.md). Permissions and actions:
[05-actions.md](05-actions.md), [07-permissions.md](07-permissions.md). Agents: [09-agents.md](09-agents.md),
[10-backend-policy.md](10-backend-policy.md). Push: [13-push-notifications.md](13-push-notifications.md).
Apps, technical: [15-client-architecture.md](15-client-architecture.md). Terminal:
[24-terminal.md](24-terminal.md). Visualization: [25-visualization.md](25-visualization.md).
Design: [docs/design/](../design/README.md).

This chapter describes what people see and operate. It does not define permissions; every
operation names the action from the action catalogue that the server checks. The app shows only
what the server allows and contains no permission logic of its own.

## 1. Vision

Bothy is a **command center for AI agents in chat form**: chat, live view, grants,
terminal, costs and statistics in one self-hosted, open-source app. Agents are Hermes
Agent, Claude Code and others that connect through the public API.

1. **Chat like Discord.** Structure and operation like Discord; AI agents are full participants.
2. **Live view.** Every thread is an agent session: you see the agent working
   (live box with tool and status lines), answer its questions and approve actions with a
   button.
3. **Grants.** Every person sees and controls only what has been granted to them; the server
   checks this on every access ([07-permissions.md](07-permissions.md)).
4. **Terminal.** A real shell or an interactive Claude Code session on a connected
   host, in the desktop app, on the phone and in the web console. Security is built in and cannot
   be turned off: a separate permission per host, confirmation on every opening, an end-to-end
   encrypted stream, audit, idle timeout ([24-terminal.md](24-terminal.md)).
5. **Costs.** Every agent shows what it runs on (personal subscription or organization key),
   and in organization mode what a run, a thread, a space costs
   ([10-backend-policy.md](10-backend-policy.md)).
6. **Statistics.** Own, native visualizations in apps and console: live topology of the runs,
   backend view, delivery live, executive page, playable timeline, TV mode; for
   operations the same views as a Grafana panel plugin ([25-visualization.md](25-visualization.md)).
7. **Own instance.** An instance belongs to one organization and runs on its own hardware or in
   its own cloud. Hermes Agent is one agent framework among several, not the core.
8. **Showcase standard.** What people see (apps, console, Grafana plugin, README, docs site,
   release pages) should be clear to a person within one minute at first sight. Images and video
   come from a demo instance with invented data (PLAN, milestone M7).

## 2. Personas

Described neutrally; the names are examples. Roles are templates for grants, not a separate
kind of permission (Baseline section 3).

| Persona | Goal | Typical tasks | Devices | Grants (example) |
|---|---|---|---|---|
| **Owner** (Alex) | Work with agents and lead the instance in terms of content | Chats with agents in all spaces, give grants, configure agents and backend policy, keep an eye on costs, read the audit; on the go, open a terminal on their own host from the iPhone; live topology and executive page | Desktop (Mac or Windows), iPhone | Owner: everything except other people's direct messages; terminal on every host, unless I6 blocks it ([24-terminal.md](24-terminal.md), section 7) |
| **Deputy** (Sam) | Operate granted spaces day to day | Trigger agents in the Support space, answer questions, review drafts; approve actions only if the capability has been given | iPhone, Android, laptop | `trigger` on the Support space, plus `upload`; no `approve_actions`; no terminal |
| **Developer** (Robin) | Work on code with a runner agent | Posts in dev forums, read tool output, stop runs, watch costs and context, resume sessions; in the terminal on her own host, use `claude` interactively | Desktop with keyboard | `trigger` on a dev forum with project binding; grant `terminal` only on the host assigned to her (SR10); for isolation see [21-dev-isolation.md](21-dev-isolation.md) |
| **Instance operator** | Install the instance and keep it running | Compose installation, login providers, backup, updates, status page, monitoring; pair hosts and declare the LLM login; Grafana with the panel plugin; TV mode in the office | Web console, command line, Grafana | Owner or a separate person with the owner role; not to be confused with the operator agent (`operator`) |
| **Bot developer** | Connect an agent or an app | Build an adapter or app with the SDK, write the manifest, request a token from the owner, test webhooks | Own environment, web console of the owner | App or agent principal with scopes; at most `write` (R-DM-08) |

## 3. Feature list

Column "Milestone": **core** = content of a build milestone (the plan defines the order), **later** =
wanted, but after the core milestones, **never** = deliberately excluded, with a reason. Column
"Where": D = desktop (macOS, Windows), M = mobile (iOS, Android), W = web console. Where a feature
of the addendum of 2026-10-10 is built in several milestones, the remark names the build
milestones from the [plan](../PLAN.md).

### 3.1 Structure and conversations

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Spaces collapsible per device, personal order across devices | core | D, M | `sidebar.update` | global order only through administration (`space.reorder`) |
| Text channels with main thread and threads from messages | core | D, M | `thread.create`, `message.create` | |
| Forum channels with a dense post list, tags, Done section | core | D, M | `thread.create`, `thread.tags.set` | list instead of cards (study B); on desktop in the forum overview D15, pinnable as a column (R-PR-17) |
| Open posts and threads nested under their channel in the sidebar | core | D, M | `thread.read` | as in Discord; selection, order and markers in 6.8 (addendum of 2026-10-27) |
| Dev forum with project binding and runner agent | core | D, M | `channel.project.bind` | triggering only with `project_bindings.ready` |
| Button "New post" or "New chat" | core | D, M | `thread.create` | replaces the detour through a helper channel; also as "+" at the forum row in the sidebar, without a dialog (6.8) |
| Fixed thread ID, auto title with source, edit title | core | D, M | `thread.title.update` | a manual title is never overwritten automatically |
| Set Done, a new message reopens | core | D, M | `thread.close`, `thread.reopen` | section 6.1 |
| Move a thread (only channels with the same operator) | core | D | `thread.move` | the dialog shows only valid targets |
| Direct messages between people | later | D, M | `dm.create` | switch `dm.enabled`; without an agent |
| Threads in threads | never | | | one level is enough; deeper trees fragment agent sessions |
| Voice and video channels, screen sharing | never | | | not core to the product; large operational effort (Baseline 4) |
| Multiple servers, server bar, public invite links | never | | | one instance is one organization (B13); outsiders join only when the owner creates them |
| Role hierarchy like Discord | never | | | grants with levels and capabilities replace roles (ADR 0018) |

### 3.2 Agents

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Operator per channel visible in the channel header | core | D, M | `channel.read` | without an operator it says "No agent answers here" |
| Live box running, pinned, with stop | core | D, M | `run.read`, `run.cancel` | section 6.2 |
| Live box collapsed after the end, expandable | core | D, M | `run.read` | |
| Retry a failed or cancelled run on its line | core | D, M | `agent.invoke` (`retryRun`) | section 6.2, addendum of 2026-10-30 |
| Trigger state as icons on the message | core | D, M | | replaces status emoji (section 6.3) |
| Follow-ups during a run | core | D, M | `run.followup.create` | |
| Buttons, selects, forms on messages | core | D, M | `interaction.create` | once, only for those authorized |
| Slash commands with suggestions and options | core | D, M | `command.invoke`, `command.autocomplete` | control commands (`/stop`) never go to the model |
| One agent session per thread, earlier sessions, start over, resume | core | D, M | `agent_session.reset`, `agent_session.bind` | |
| Backend policy badge in the thread header and on the agent card | core | D, M, W | `agent.read` | Personal or Organization |
| Costs per run and thread (organization: estimate until reconciliation) | core | D, M | `cost.read.thread` | with Personal only information |
| Overview "Running now" | core | D, M | `run.overview.read` | as live cards or list (D10, M11; addendum of 2026-10-24) |
| Run graph: agent, subagents, workflows and background jobs as a tree on a time axis, live | core | D, M | `run.read` | D14, M12; addendum of 2026-10-24; not in the web console (no chat, ADR 0028) |
| Typing indicator (people); the agent "types" through the live box | core | D, M | `typing.start` | |
| Presence of agents (online, degraded, offline) | core | D, M | `agent.read` | people have no presence |
| Presence of people ("online") | never | | | reveals working hours; privacy and co-determination (study C, R5) |

### 3.3 Messages

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Markdown, code blocks, long answers without splitting, collapsed with "Show more" | core | D, M | `message.create` | limit `message.max_content_bytes`; a single code or quote block copies on its own (6.12); a longer text goes as one attachment of the same message and a long paste becomes a file (6.13, addendum of 2026-10-29) |
| Edit and delete own messages, view versions | core | D, M | `message.update`, `message.delete`, `message.revisions.read` | no editing deadline |
| Reply with quote | core | D, M | `message.create` | |
| Reactions with a free Unicode character | core | D, M | `reaction.add` | status is not a reaction |
| Mentions of people and agents | core | D, M | `message.create` | suggestions only from what is visible |
| @everyone, @here, role mention | never | | | noise; notification per channel is enough |
| Pin (moderation) | core | D, M | `pin.create` | needs `manage` |
| Templates (personal and per channel) | core | D, M | `template.read`, `template.manage` | |
| Drafts across devices | core | D, M | `draft.write` | notice when a newer, different draft of another device was overwritten (04 14) |
| Read receipts of other people | never | | | performance monitoring; the read state stays personal (ruling R-PR-03) |
| Polls, widgets, to-do lists in text | never | | | agents deliver forms through components |
| Stickers, GIF search, profile decorations, banners | never | | | distraction, license questions (Baseline 4) |
| Link preview from the server | never | | | the server loads no foreign URLs (SSRF) |
| Link preview in the client | later | D, M | | only after opt-in per person, because the app then contacts foreign servers |

### 3.4 Files and voice

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Files and images with preview, in and out | core | D, M | `file.upload`, `file.read` | served only after the scan |
| iPhone: photo, camera, file | core | M | `file.upload` | |
| Record a voice message, play it, transcript | core | D, M | `file.upload`, `file.transcribe` | switch `voice.enabled`; AAC (15-client-architecture.md) |
| Share from other apps into a chosen target | core | M, macOS | `message.create`, `file.upload` | Windows see the next row |
| Share from other apps on Windows | later | D | | needs MSIX identity; after the first Windows release |
| Export of a channel | core | D, W | `export.channel` | capability `export`, step-up |

### 3.5 Notification, search, navigation

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Unread counter, mention counter | core | D, M | | from `read_states` and `mentions` |
| View "Mentions" | core | D, M | `mention.list` | |
| Notification per space, channel, thread (all, mentions, mute), quiet hours | core | D, M | `notification_pref.update`, `quiet_hours.update` | section 10 |
| Push without content; the app fetches the text | core | M, macOS | `push.register` | Windows: notification area with its own connection |
| Push through the Windows service (WNS) | later | D | | without a running app no notification; Baseline R-Push |
| Email notifications | never | | | content does not belong in mailboxes (Baseline 4) |
| Full-text search with permission filter | core | D, M | `search.query` | hits only from what is visible |
| Quick switcher Cmd/Ctrl+K | core | D | | on mobile as a search field at the top |
| Keyboard shortcuts | core | D | | section 7 |
| Multiple windows, thread as its own window | later | D | | multi-window in Flutter still experimental (15-client-architecture.md) |
| iPad layout with columns | later | M | | until then the scaled iPhone layout |

### 3.6 Offline, sync, devices

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Read offline (recently opened threads) | core | D, M | | cache according to [04](04-sync-protocol.md) section 13 |
| Send offline with a queue, states visible | core | D, M | | section 6.6 |
| Read state, drafts, settings, sidebar in sync | core | D, M | | |
| Approve, revoke a device; end sessions | core | D, M, W | `device.approve`, `device.revoke`, `session.revoke` | new devices wait |
| End-to-end encryption of chat content | never | | | agents must read (Baseline 4); the terminal stream, by contrast, is end-to-end encrypted (3.8) |
| Chat in the browser | never | | | web only for the console (ADR 0028) |

### 3.7 Administration, operations, integration

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Create, disable people; groups | core | D, W | `user.create`, `user.disable`, `group.*` | no self-registration |
| Grant wizard with plain-language preview and I1 check | core | D, W | `grant.create` | step-up |
| "Who has access?" with origin | core | D, W | `access.explain` | |
| "View as person" (read only) | core | D, W | `access.view_as` | logged |
| Agents: backend switch, costs, tokens, commands | core | D, W | `agent.billing_mode.set`, `cost.read.aggregate`, `token.create` | |
| Filter the audit, verify the chain | core | D, W | `audit.read`, `audit.verify` | |
| Statistics | core | D, W | `stats.read` | |
| Live topology of the agent runs, backend view, delivery and sync live, playable daily timeline | core | D, M, W | `stats.read`, `cost.read.aggregate` | own measurement, no values per person; [25-visualization.md](25-visualization.md); mobile screen M10; build: visualization track after the early M4 tickets against fixtures, server and apps M6 |
| Executive page with SLO traffic light "own measurement" | core | D, M, W | `stats.read`, `cost.read.aggregate` | traffic light only for owners; build M6 |
| TV mode (full screen, dark, rotating views) | core | D, W | `stats.read` | costs hidden by default; build M6; screen D13 |
| Grafana panel plugin "Bothy Live" for operations | core | Grafana | none (permissions of Grafana) | build M7; unsigned locally until V1, signature later |
| Embed Grafana dashboards in apps | never | | | only optionally in the web console, off by default ([ADR 0069](../adr/0069-visualization-on-two-levels-without-iframes-in-apps.md)) |
| Status page | core | D, M, W | `status.read` | public only with `status.public_enabled` |
| Import from Discord | core | W | `import.manage` | [22-import-migration.md](22-import-migration.md) |
| Install apps, webhooks | core | W | `app.install`, `webhook_out.manage`, `webhook_in.create` | |
| Emergency actions on the phone: revoke a device, revoke a grant, disable a person, stop a run | core | M | `device.revoke`, `grant.revoke`, `user.disable`, `run.cancel` | large flows (create a grant, audit, statistics) only on desktop and web |
| Marketplace for bots | never | | | the owner installs apps deliberately; no catalogue of third-party providers |

### 3.8 Terminal

Normative: [24-terminal.md](24-terminal.md). Build: server and host service M2, desktop M4, mobile M5,
web console M6.

| Feature | Milestone | Where | Action(s) | Remark |
|---|---|---|---|---|
| Open a terminal: login shell on a connected host (macOS, Linux, Windows) | core | D, M, W | `terminal.open` | fresh step-up per opening, in the console passkey only ([ADR 0070](../adr/0070-terminal-in-the-web-console.md)); screens D11, M8 |
| Claude Code session in the terminal (profile `claude`) | core | D, M, W | `terminal.open` | interactive `claude` in a work folder from the allow list of the person's own session user (ruling T13), separate from the runner; only on hosts with a declared LLM login (I6) |
| Session list per host, detach and reattach, also from another device | core | D, M, W | `terminal.read_own`, `terminal.attach` | the session keeps running for `terminal.detach_grace`; another device with a new step-up |
| End a session | core | D, M, W | `terminal.close` | owners end any session |
| Extra keys bar (Esc, Tab, Ctrl, Alt, arrows, `\|`, `~`, `/`, `-`) and hardware keyboard | core | M | | build M5; screen M8 |
| Host fingerprint and key pinning per device with a hard warning on a change | core | D, M, W | | [24-terminal.md](24-terminal.md), section 5.5 |
| Create a host, pair it, confirm the fingerprint, declare the LLM login, assign a session user per person, disable, retire | core | D, W | `host.read`, `host.manage` | owner only, step-up, audit; without an own session user no terminal (SR12); screen A8 |
| Grant `terminal` per host | core | D, W | `grant.create` | owner only; for non-owners it takes effect only on the host assigned to them (SR10); the preview says so |
| Take over a runner's thread in the terminal | later | D | | only when runner and host service run as the same user in a dev environment ([24-terminal.md](24-terminal.md), section 11.3) |
| File transfer and clipboard through the terminal channel | later | D, M, W | | blocked in this version (OSC 52, [24-terminal.md](24-terminal.md), section 11.2) |
| Recording of terminal content | never | | | metadata only; whether evidence obligations require a recording is an owner question (default: no) |
| Terminal for agents or apps | never | | | an agent that prompt injection controls must not reach a host (T-TM-09) |

## 4. Information architecture

```text
Instance (branding, status page)
+-- Spaces (sidebar, collapsible, category color, icon, default operator)
|   +-- Text channel -> Main thread -> threads from messages (open ones nested in the sidebar)
|   +-- Forum        -> Post list   -> one thread per post (tags, Done section; open posts
|                       (forum overview)   nested in the sidebar; with a project binding every
|                                          post is one agent session)
+-- Direct messages (later)
+-- Cross-cutting: Running now, Mentions, Search, Quick switcher, Settings
+-- Hosts (terminal): profiles and own sessions per host
+-- Live and statistics: live topology, executive page, timeline, TV mode
+-- Administration (desktop and web console; emergency actions on mobile)
Before login: server address, login, enrollment code, device waiting, pairing (5.6)
```

A thread is the same object with a fixed ID everywhere: in a forum a post, in a text channel a
thread from a message. Its agent session, runs and costs are attached to it.

The two channel kinds differ in what one conversation is. A **text channel** (`#`) is one shared
conversation (the main thread); side conversations are threads started from its messages. In a
**forum** every post is its own conversation with its own title, tags and Done state; in a forum
with a project binding (dev forum, icon `channel.project`) every post is one agent session in the
project, so a project has as many parallel chats as it has open posts. The app says this in the
tooltip of the channel icon, as the subtitle of the channel header and in the empty state of a
forum (addendum of 2026-10-27, D-SB-10).

Every forum whose operator is the Claude Code runner is a Claude Code chat (addendum of
2026-10-30, R-PR-27, [09-agents.md](09-agents.md) 18.1). The project folder need not be a Git
repository: the runner reports a bound folder without Git as ready. A forum without a project
binding works in the runner's default folder (`runner.default_workspace`) when its operator set
one; without it, a message there fails with a sentence that says the forum has no project folder
and to bind one. Every failed run shows its cause in the thread next to the reason, never only
"Error in the adapter".

## 5. Screens

Symbols in the wireframes: `v`/`>` expanded/collapsed, `#` text channel, `=` forum, `{}` forum
with a project binding, `'-` thread row nested under its channel, `[+]` new chat or new post, `*`
unread or run active, `(3)` counter, `[ ]` button or field, `[A]` agent (rounded square),
`(AL)` person (round), `(o)` running, `[!]` waiting for you, `(@)` mention, `(z)` queue,
`(v)` done, `(x)` failed, `(-)` cancelled. Trigger state on messages in
curly braces: `{ok}` done, `{..}` in progress, `{z}` queue, `{x}` failed,
`{->}` follow-up delivered. Rendered design: [design system](../design/README.md) with the
images from `docs/design/previews/`.

### 5.1 Desktop (macOS, Windows)

**D1 Main view, forum.** Three columns: spaces and channels with their open threads (brand
surface), history with a pinned live box, right session column (collapsible, instead of a member
list). No server bar on the left (B13). Under each forum its open posts and under each text channel
its open threads appear as nested rows with a tree line, as in Discord; in a forum with a project
binding every post is one agent session, so the sessions of a project are visible and open with one
click. Which rows show, their order, markers and the "+" action are in 6.8. A click on the forum
name opens the forum overview D15 in the content column; pinned (Cmd+Shift+O, button "Pin" in the
overview), the overview is its own column between sidebar and history, and D1 has four columns.
Every column border can be dragged: between sidebar, pinned overview, history and session column,
and between the main history of a text channel and a thread open beside it (D2); widths are
stored as the setting `layout.column_widths` (names `sidebar`, `post_list`, `main_history`,
`session`; the history, or the thread beside a main history, takes the rest). No column becomes
narrower than `DsLayout.columnMin` by a drag; a stored width wider than the window shrinks to fit.
Below `ui.desktop_min_width_four_columns` the session column collapses, below
`ui.desktop_min_width_three_columns` a pinned overview column returns to the content column
(navigation stack as on the phone). Addendum of 2026-10-27 (R-PR-17, replaces the four fixed
columns of R-PR-01).

When a thread or a forum post is open next to its channel column (the main history of a text
channel, D2, or the pinned overview of a forum), the button "Hide channel" in the thread header
and Cmd+Shift+H (Ctrl+Shift+H) hide that column, so the thread takes the whole content width;
"Show channel" and the same keys show it again. The choice is kept on this device like the pin
of the overview, never synchronized, and stays while the person opens other threads, from the
sidebar or elsewhere; the pin itself does not change. The main history of a channel alone has
nothing to hide. Addendum of 2026-10-28 (R-PR-19).

The right session column is collapsible the same way: the button "Hide session" in the header of
a thread or main history and Cmd+Shift+I (Ctrl+Shift+I) hide it, so the history takes its width;
"Show session", the same keys, "Pin" and a slim rail with a "Show session" button at the right
edge of the window show it again. The rail is there only while the column is hidden and the
window is wide enough for it. The choice is kept on this device like the channel column, never
synchronized, and stays while other threads open; it is kept apart for the two layouts (shown by
default beside a forum post or a main history alone, collapsed by default beside a thread of a
text channel, D2). Addendum of 2026-10-29 (R-PR-22).

```text
+----------------------------------+---------------------------------------+------------------------+
| [^] Bothy                        | Gateway: resume after network change  | Session            [x] |
| [Jump to ...  Cmd+K]             | {} server  [Organization]  1,84 USD   |                        |
| (o) Running now           (2)    | estimate  [Done] [Pin] [Hide session] | [A] dev-runner         |
|                                  | +- (o) Running 4:12  62 % 0,62 USD -+ |     online             |
| v [H] Support                    | | Checking if resume restores ...   | | Backend [Organization] |
|     # general                    | | [ok Read] internal/sync/gw.go     | | Model      model-large |
|       '- Export format       (1) | | [.. Bash] go test ./internal/...  | | Session   0193f2a7-..  |
|     = requests          * (3)    | | > On resume, the focus is taken.. | | Project   server, main |
|     # alerts  (muted)            | | [Stop]  Messages = follow-up      | |                        |
| v [<>] Engineering               | +-----------------------------------+ | Context  [######----]  |
|     {} server  [+]          *    | [A] dev-runner AGENT 10:14            | Thread cost   1,84 USD |
|       '- New chat (draft)        |   Cause: ... [code block]             |         (estimate)     |
|      >'- (o) Gateway: resume a.. |   [v Ran 12 min, 3 tools  >]          | Runs                5  |
|       '- [!] Check migration 0.. | [A] dev-runner AGENT 10:16            |                        |
|       '- (@) Review: grant.. (1) |   Approval needed: go test ...        | Earlier sessions (2)   |
|       '- Search: hits in tran..  |   [Allow once][Session][Always]       | Pinned (2)             |
|       '- All posts           (1) | |(SA) Sam: @Alex can you ...     (!)  | Files (3)              |
|   > {} apps                 *    | (AL) Alex 10:22 {..} @dev-runner ...  |                        |
|     # releases                   | ... Sam is typing                     |                        |
| > [M] Marketing             *    |                                       |                        |
| > [R] Research                   |                                       |                        |
|                                  |                                       |                        |
| (AL) Alex         [Settings]     | [+] Follow-up to dev-runner [Mic][>]  |                        |
+----------------------------------+---------------------------------------+------------------------+
```

**D2 Text channel with an open thread.** No post list. A thread opens to the right of the
main thread; the session column is then collapsed until the person shows it ("Show session",
Cmd+Shift+I or the rail, D1), and that choice is kept on this device. The border between the
two can be dragged (`main_history`; without a stored width they share the room), and "Hide
channel" or Cmd+Shift+H leaves the thread alone in the content column (D1, R-PR-19). The header names the
operator. The open threads of the channel are nested under it in the sidebar (6.8); the selected
one is marked there.

```text
+--------------------------+------------------------------------------------+------------------------------------------+
| v [H] Support            | # general   Operator: support-agent    [Topic] | #general > Request summary               |
|     # general            |                                                | [Organization]  [Done]  [...]            |
|       '- Export format   | (SA) Sam 09:12                                 |                                          |
|      >'- Request summ..  |   Customer asks about the export format.       | [v] Ran 2 min, 1 tool, 0,04 USD >        |
|     = requests    * (3)  |   [Thread: Export format, 4 replies >]         | [A] support-agent AGENT 09:31            |
|     # alerts             | (AL) Alex 09:30 {ok}                           |   The request concerns ... (truncated)   |
|                          |   @support-agent summarize the request         |   [Show more]                            |
|                          |   [Thread: Request summary 4711 >]             | (AL) Alex 09:33 {->}                     |
|                          | (RO) Robin 09:41                               |   Please also name the deadline.         |
|                          |   Thanks, I'll take it from here.              |                                          |
|                          |                                                |                                          |
|                          | [+] Message to #general              [Mic][>]  | [+] Follow-up to support-agent [Mic][>]  |
+--------------------------+------------------------------------------------+------------------------------------------+
```

**D3 Thread view: live box in four states.** Running (pinned), collapsed after the end, waiting
for an answer, failed. Specification: [components.md](../design/components.md) section 2. The
expanded box has a button "Graph" that opens the run graph D14 (addendum of 2026-10-24).

```text
+--------------------------------------------------------------------------------+
| (o) Running 4:12   Context 62 %   48,1k tokens   0,62 USD   [Organization] [v] |
| Checking if resume restores the focus after the network change.                |
|   [ok Read ] internal/sync/gateway.go                              0,4 s       |
|   [ok Grep ] resume_window                                          0,2 s      |
|   [.. Subagent] Write test cases for 4006                                      |
|       [.. Bash] go test ./internal/sync/...                     running 38 s   |
|   2 earlier steps [show]                                                       |
| > On resume, the focus is taken over from the gateway session; it is still     |
| > open whether expired threads are dropped silently.                           |
| [Stop]   New messages in this thread go to the run as a follow-up.             |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| (v) Ran 28 min   3 tools   2 subagents   1,20 USD estimate                [>]  |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| [!] Waiting for you  2:03  0,18 USD                                [Personal]  |
| Which database should the test use?                                            |
|   Database [Local container  v]   Port [55432   ]   [Answer]                   |
+--------------------------------------------------------------------------------+
```

```text
+--------------------------------------------------------------------------------+
| (x) Failed   Backend mismatch: reported organization, configured personal      |
| [Try again]  [Details]                                                         |
+--------------------------------------------------------------------------------+
```

**D4 Components on messages.** Approval buttons (once, session, always, deny), selection,
form; after use, a line with the person and the choice.

```text
+--------------------------------------------------------------------------------+
| [A] dev-runner AGENT 10:16                                                     |
|   +- [Shield] Approval needed ------------------------------------------+      |
|   | go test wants to use the network for the test database.              |     |
|   | [Allow once] [For this session] [Always] [Deny]                      |     |
|   | [Lock] Only Alex or an owner can approve this.                       |     |
|   +---------------------------------------------------------------------+      |
|   +- Choose model ------------------+   +- Question ---------------------+     |
|   | [model-large            v]      |   | Customer    [Text          ]   |     |
|   |  > model-large                  |   | Priority    [normal v]         |     |
|   |    model-fast                   |   | [Submit]                       |     |
|   +---------------------------------+   +--------------------------------+     |
|   After use:  {ok} Alex chose 'Allow once', 10:24                              |
+--------------------------------------------------------------------------------+
```

**D5 Slash autocomplete.** Opens on "/" at the start of a line. Commands of the operator first,
commands of the same name from other agents with their handle. Options with suggestions from the
agent.

```text
+--------------------------------------------------------------------------------+
| +- Commands from dev-runner (operator of this channel) -------------------+    |
| | > /status   State of the current run                    control command |    |
| |   /stop     Cancel the run, never goes to the model     control command |    |
| |   /model    Choose a model for this session             name            |    |
| |   /stats    Channel metrics                             research-bot    |    |
| +-------------------------------------------------------------------------+    |
| [+] /st|                                                          [Mic][>]     |
|                                                                                |
| After selection:  /model [name: model-l|]   Suggestions from the agent:        |
|                   > model-large   model-fast                                   |
+--------------------------------------------------------------------------------+
```

**D6 Quick switcher (Cmd+K or Ctrl+K).** Jumps to a channel, thread, person, agent or command.
Results only from what is visible; recently visited targets first. At most
`ui.quick_switch_max_results` results.

```text
+------------------------------------------------------------------------+
| [Lens] serv|                                                    [esc]  |
+------------------------------------------------------------------------+
| > = server                                            Engineering      |
|   [Thread] Gateway: resume after network change            server      |
|   [A] dev-runner                                            Agent      |
|   (SA) Sam                                                 Person      |
+------------------------------------------------------------------------+
| Tab: Filter  Channels | Threads | People | Agents | Commands           |
+------------------------------------------------------------------------+
```

**D7 Search.** Full text across messages, thread titles, file names and transcripts; filters on
the left; results with highlighting and path. The result count counts only what is visible.

```text
+--------------------------+------------------------------------------------------------------------+
| Filter                   | [Lens] resume_window                                    [Search]       |
+--------------------------+------------------------------------------------------------------------+
| Space    [all v]         | 14 results, only from channels you may see                             |
| Channel  [all v]         |                                                                        |
| From     [all v]         | = server > Gateway: resume after network change                        |
| Type [x] Messages        |   (A) dev-runner 10:14  ... window **resume_window** is measured       |
|      [x] Threads         |   from the last heartbeat ...                                          |
|      [x] Files           | # releases > Version 0.4                                               |
|      [ ] Transcripts     |   (RO) Robin yesterday  ... new **resume_window** ...                  |
| Time [any v]             | [File] notes-sync.md  ... **resume_window** ...                        |
| Status [open+done]       | [load more]                                                            |
+--------------------------+------------------------------------------------------------------------+
```

**D8 Settings.** Appearance, notifications, language, keyboard, devices and sessions, privacy,
about (version, licenses). Under "Devices and sessions" the person confirms a waiting device with
its code (E4) and adds a device with a pairing code (E5), both with step-up.

```text
+--------------------------+------------------------------------------------------------------------+
| Settings                 | Appearance                                                             |
+--------------------------+------------------------------------------------------------------------+
| > Appearance             | Theme        ( ) System  ( ) Light  (x) Dark                           |
|   Notifications          | Density      (x) Normal  ( ) Compact                                   |
|   Language               | Motion       (x) System  ( ) Full  ( ) Reduced                         |
|   Keyboard               | Enter sends  [x]   (Shift+Enter: new line)                             |
|   Devices and sessions   | Column widths [Reset]                                                  |
|   Privacy                |                                                                        |
|   About                  | Language     [German v]  Time zone [Europe/Berlin v]                   |
|                          | Devices: iPhone (active), Laptop (this device), [Revoke device]        |
|                          | Waiting: Tablet [Enter code]   [Add device]                            |
|                          | Privacy: [x] Crash reports  [x] Performance data  [Export]             |
+--------------------------+------------------------------------------------------------------------+
```

**D9 Notification settings.** Default, quiet hours, content in notifications, level per space,
channel and thread with visible inheritance; time-limited muting. The row "Runs I triggered" is
an explanation of 10.1, not a setting: a question of the person's own run always notifies, its
end or failure follows the level; 13 3.3 has no key for it (docs/DECISIONS.md D-M4-33-4).

```text
+------------------------------------------+--------------------------------------------------------+
| Notifications                            |                                                        |
+------------------------------------------+--------------------------------------------------------+
| Default for new channels                 | ( ) All  (x) Mentions  ( ) Mute                        |
| Runs I triggered                         | Questions always; done or failed by the level          |
| Show content in notifications            | [x] Load text (otherwise only 'New message')           |
| Quiet hours                              | Mon-Fri 19:00-07:00, Sat-Sun all day  [Edit]           |
+------------------------------------------+--------------------------------------------------------+
| Per space, channel and thread            | Level                                                  |
| v Support                                | [All v]                                                |
|     # general                            | [inherits: All]                                        |
|     = requests                           | [Mentions v]                                           |
| v Engineering                            | [Mentions v]                                           |
|     = server > Gateway: resume ...       | [All v]  until [today 18:00 v]                         |
| > Marketing                              | [Mute v]                                               |
+------------------------------------------+--------------------------------------------------------+
```

**D10 Running now (control room).** All active runs that the person may see; stop only where
`run.cancel` is allowed (triggering person, `manage`, owner). Addendum of 2026-10-24 (run
visualization, [design note](../design/run-visualization.md)): the view has the switch "Cards" and
"List"; cards are the default. A card shows agent, path, state, duration, current tool, open
subagents, workflows and background jobs, the stall notice ("no movement since ..."), costs with
the suffix rules below, Stop where allowed, and opens the run graph D14 on click. Cards visible on
the screen are live; a card outside the live budget shows the summary with "live details paused"
(data: [09-agents.md](09-agents.md), section 5.6). "List" shows the rows below and is also the form
for screen readers.

```text
+----------------------------------------------------------------------------------------------------+
| Running now (3)                  [Cards | List]    [Filter: Space v] [Agent v] [mine only]         |
+----------------------------------------------------------------------------------------------------+
| +- dev-runner ---------------------------+  +- dev-runner ---------------------------+             |
| | (o) running  4:12     0,62 USD est.    |  | [!] waiting for you  2:03              |             |
| | server > Gateway: resume after ...     |  | server > Check migration 0012          |             |
| | [.. Bash] go test ./internal/sync/...  |  | Approval waiting                       |             |
| | 2 subagents, 1 background job          |  |                                        |             |
| | [Graph]                        [Stop]  |  | [Graph]                        [Open]  |             |
| +----------------------------------------+  +----------------------------------------+             |
| +- research-bot -------------------------+                                                         |
| | (~) running  31:40                     |                                                         |
| | no movement since 12:41                |                                                         |
| | Research > Weekly report               |                                                         |
| | live details paused                    |                                                         |
| | [Graph]                        [Stop]  |                                                         |
| +----------------------------------------+                                                         |
+----------------------------------------------------------------------------------------------------+
| Organization: sum of active runs 0,80 USD (estimate). Personal runs: information only.             |
+----------------------------------------------------------------------------------------------------+
```

```text
+----------------------------------------------------------------------------------------------------+
| Running now (3)                    [Cards | List]    [Filter: Space v] [Agent v] [mine only]       |
+----------------------------------------------------------------------------------------------------+
| (o) dev-runner     server > Gateway: resume after network change   4:12  62 %  0,62 USD  [Stop]    |
| [!] dev-runner     server > Check migration 0012         waiting for you  2:03  [Open]             |
| (z) research-bot   Research > Weekly report                Queued                 [Open]           |
+----------------------------------------------------------------------------------------------------+
| Organization: sum of active runs 0,80 USD (estimate). Personal runs: information only.             |
+----------------------------------------------------------------------------------------------------+
```

**D11 Terminal.** Sidebar with the group "Hosts" below the spaces; on the right the terminal view
with one tab per own session on the host. The header shows the host, the connection, the badge of
the LLM login, the start of the fingerprint and the time left until
`terminal.max_session_duration`. "Detach" keeps the session running (it can be reattached), "End"
closes it. Opening requires confirmation with a passkey or a step-up device key; the dialog
names the host and the profile. All keys go to the shell, except the shortcuts from section 7.

```text
+----------------------------+-------------------------------------------------------------------------------+
| [^] Bothy                  | Build host  [online]  [Organization]  Fp 7KQ2 9XTA ..  Left 11:42     [i]     |
| [Jump to ...  Cmd+K]       | [shell 1 x] [claude: server x] [+ New session v]             [Detach] [End]   |
| (o) Running now       (2)  +-------------------------------------------------------------------------------+
|                            | ~/src/server $ go test ./internal/sync/...                                    |
| > [H] Support              | ok      internal/sync           4.212s                                        |
| v [<>] Engineering         | ok      internal/sync/gateway   1.874s                                        |
|     = server         *     | ~/src/server $ git status --short                                             |
|     # releases             |  M internal/sync/gateway.go                                                   |
|                            | ~/src/server $ _                                                              |
| v Hosts                    |                                                                               |
|   (o) Build host     (2)   |                                                                               |
|   ( ) Linux VM             |                                                                               |
|   (-) Test Windows         |                                                                               |
|       disabled             |                                                                               |
|                            +-------------------------------------------------------------------------------+
| (AL) Alex       [Settings] | End-to-end encrypted | Profile shell | Idle timeout active                    |
+----------------------------+-------------------------------------------------------------------------------+
```

Confirmation on opening and warning on a changed key (24, section 5.5):

```text
+------------------------------------------------------------+  +------------------------------------------------------------+
| Open terminal                                              |  | [!] The host's key has changed                             |
+------------------------------------------------------------+  +------------------------------------------------------------+
| Host      Build host                                       |  | Stored:       7KQ2 9XTA M4PB 1C8D ...                      |
| Profile   (x) shell  ( ) claude [Working folder: server v] |  | Now:          Q0ZJ 55RW 2HNE K7TS ...                      |
| Login     [Organization]  The organization bears the costs |  | Without re-pairing by an owner, this is a sign of an       |
|                                                            |  | attack. Connect only after you have checked the new        |
| Every opening is logged.                                   |  | fingerprint on the host.                                   |
| [Cancel]                  [Confirm with passkey / device]  |  | [Do not connect]                      [Accept new key]     |
+------------------------------------------------------------+  +------------------------------------------------------------+
```

**D12 Live topology.** Lanes per space, run cards with state, tool, badge and costs; a click opens
the thread. The same data as a table via "Table". Normative:
[25-visualization.md](25-visualization.md), section 6.1.

```text
+----------------------------+-------------------------------------------------------------------------------+
| [^] Bothy                  | Live  [Topology] [Backend] [Delivery] [Executive] [Timeline]                  |
| (o) Running now       (3)  | Space [all v]                       12:04:31  live   [Table] [TV mode]        |
| v Live and statistics      +-------------------------------------------------------------------------------+
|   > Live                   | Engineering | = server   +- dev-runner ------------------+                    |
|     Statistics             |             |            | (o) running  4:12             |                    |
|                            |             |            | [.. Bash] go test             |                    |
| > [H] Support              |             |            | [Org.] 0,62 USD estimate      |                    |
| v [<>] Engineering         |             |            +-------------------------------+                    |
|     = server         *     |             | # releases  (no active runs)                                    |
| v Hosts                    | Support     | = requests +- support-agent ---------------+                    |
|   (o) Build host     (2)   |             |            | [!] waiting_input             |                    |
|                            |             |            | Approval waiting  [Personal]  |                    |
|                            |             |            +-------------------------------+                    |
|                            +-------------------------------------------------------------------------------+
|                            | Last minute: messages 14 | runs 3 | tool calls 41 | errors 0                  |
+----------------------------+-------------------------------------------------------------------------------+
```

**D13 TV mode.** Full screen, dark theme, larger font, no controls; rotates through the selected
views every `viz.tv_rotation_interval`. Costs only with `viz.tv_show_costs`. A key press or a click
ends it. Desktop and web console, not mobile.

```text
+--------------------------------------------------------------------------------------------------------+
|  BOTHY LIVE                        Live topology  (1 of 3)                           12:04   [=  =  =] |
+--------------------------------------------------------------------------------------------------------+
|                                                                                                        |
|   ENGINEERING        server     [ (o) dev-runner    running   Bash      Organization ]                 |
|                      releases   [ (v) research-bot  completed 2 min                  ]                 |
|                                                                                                        |
|   SUPPORT            requests   [ [!] support-agent waiting_input   Approval waiting ]                 |
|                                                                                                        |
|   RESEARCH           (no active runs)                                                                  |
|                                                                                                        |
+--------------------------------------------------------------------------------------------------------+
|   Active runs 3      Tool calls/min 41      Delivery on target [ok]      own measurement               |
+--------------------------------------------------------------------------------------------------------+
```

**D14 Run graph.** Addendum of 2026-10-24 (run visualization,
[design note](../design/run-visualization.md)). Opens from the button "Graph" of the live box
(D3) and from a card of D10. An indented trace view: the agent as the root row, subagents,
workflows and background jobs as child rows with their tools below them, each row with state
(color, icon and word), duration, current tool and cost where known (a child's cost only in
Organization mode, as "Estimate", 6.7); to the right a bar per row on
a shared time axis from the start of the run to now. The graph grows live while the run is
active; a finished run shows its final graph. A node with many finished children shows the newest
and "n earlier steps"; running nodes are never collapsed. Rows can be collapsed and expanded with
the mouse or the arrow keys. "As list" shows the same rows without bars as an outline for screen
readers. Data and derivation: [09-agents.md](09-agents.md), section 5.6; permission `run.read`.

```text
+----------------------------+-------------------------------------------------------------------------------+
| [^] Bothy                  | < Thread   Run graph  dev-runner      (o) running 4:12   0,62 USD est.        |
| (o) Running now       (3)  | server > Gateway: resume after network ...  [As list]  [Stop]                 |
|                            +-------------------------------------------------------------------------------+
| > [H] Support              |                                   0:00      1:00      2:00      3:00     4:00 |
| v [<>] Engineering         | (o) dev-runner                    [==========================================]|
|     = server         *     |   (v) Read  internal/sync/...     [=]                                         |
|                            |   (v) Grep  resume_window           [=]                                       |
|                            |   (o) Subagent  Write test cases      [==============================]>       |
|                            |       (v) Read  gateway_test.go        [=]                                    |
|                            |       (o) Bash  go test ./...                   [===================]>        |
|                            |   (o) Background job  npm test             [============================]>    |
|                            |   (x) Subagent  Check migration      [=======]  failed                        |
|                            |       3 earlier steps [show]                                                  |
+----------------------------+-------------------------------------------------------------------------------+
```

**D15 Forum overview.** Addendum of 2026-10-27 (R-PR-17). Opens with a click on the forum name,
with "All posts" at the end of the forum's group in the sidebar, or pinned with Cmd+Shift+O; shows the
post list of components.md 6 in the content column: the explanation of the kind (6.8), "New chat"
in a project forum or "New post" otherwise, the search in the forum (D7 with the filter of this
channel), the filters Open, Waiting for me, Tags and Sort, and the collapsed "Done (n)" section.
"Pin" keeps it as its own column next to the history (D1 with four columns); the choice is kept on
this device. A click on a post opens its history in the content column, or next to the pinned
overview.

```text
+----------------------------------+-------------------------------------------------+
| [^] Bothy                        | {} server   Project: every post is one agent    |
| [Jump to ...  Cmd+K]             | session of dev-runner in this project           |
| (o) Running now           (2)    | [+ New chat]  [Lens Search in server]  [Pin]    |
|                                  | [Open][Waiting for me][Tags v][Last activity v] |
| v [H] Support                    |                                                 |
|     # general                    | (o) Gateway: resume after network ch..  4 min   |
|       '- Export format       (1) |     dev-runner  [sync]                          |
|     = requests          * (3)    | [!] Check migration 0012               12:41    |
|     # alerts  (muted)            |     [Waiting for you]  [db]                     |
| v [<>] Engineering               | (@) Review: grant wizard               11:58    |
|    >{} server  [+]          *    |     Sam mentions you  [access]                  |
|       '- New chat (draft)        | (z) Search: hits in transcripts        11:20    |
|       '- (o) Gateway: resume a.. | (v) Costs per space                       2 h   |
|       '- [!] Check migration 0.. | (x) Upload limit ...               yesterday    |
|       '- (@) Review: grant.. (1) |     failed: project not ready                   |
|       '- Search: hits in tran..  |                                                 |
|       '- All posts           (1) | > Done (14)                                     |
|   > {} apps                 *    |                                                 |
|     # releases                   |                                                 |
| > [M] Marketing             *    |                                                 |
| > [R] Research                   |                                                 |
|                                  |                                                 |
| (AL) Alex         [Settings]     |                                                 |
+----------------------------------+-------------------------------------------------+
```

### 5.2 Administration (desktop and web console)

Before it is submitted, every changing administration action with step-up shows the confirmation
with passkey or device key ([08-identity.md](08-identity.md)).

**A1 People.** List, create (no self-registration, no invitation links), queue of new devices with
the origin of the login.

```text
+------------------------+----------------------------------------------------------------------------+
| Administration         | People                                    [+ Add person]                   |
+------------------------+----------------------------------------------------------------------------+
| > People               | [Lens] Search        Filter: [active v] [Group v]                          |
|   Groups               | (AL) Alex     Owner          2 devices  last seen today                    |
|   Grants               | (SA) Sam      Support (trigger) 1 device  last seen today                  |
|   Devices (1 waiting)  | (RO) Robin    Engineering (trigger) 3 devices yesterday                    |
|   Agents               | (NE) New      waiting for login                                            |
|   Apps                 | ---                                                                        |
|   Audit                | Waiting devices: Sam, 'Pixel', logged in 08:12 via OIDC                    |
|   Statistics           |   [Approve] [Reject]              Key: hardware, attested                  |
|   Instance             |                                                                            |
+------------------------+----------------------------------------------------------------------------+
```

**A2 Create grant with plain-language preview.** Three steps (who, where, level and capabilities).
The preview states the effect in sentences, names newly visible channels and checks I1 before
submitting; a violation is explained, not just rejected ("switch to organization first").

```text
+----------------------------------------------------------------------------------------------+
| Create grant                                                             Step 3 of 3         |
+----------------------------------------------------------------------------------------------+
| Who          (SA) Sam                     [Person v]                                         |
| Where        Support space  (applies to all channels in it)                                  |
| Level        ( ) read  ( ) write  (x) Trigger agents                                         |
| Capabilities [ ] Approve actions  [x] Upload files  [ ] Export  [ ] Manage                   |
| Until        [x] 31.12.2026      Reason [Support cover Q4                       ]            |
+----------------------------------------------------------------------------------------------+
| Plain-language preview:                                                                      |
|   From now on Sam may read all channels in the Support space, write there and trigger        |
|   the agent support-agent. Sam may upload files. Sam may not approve                         |
|   actions (approval buttons stay disabled). Valid until 31.12.2026.                          |
|   Newly visible for Sam: 3 channels, 214 threads.                                            |
| [!] Check: support-agent runs in Organization mode. The organization bears the costs.        |
| [x] Would be rejected: dev-runner (Personal, only Alex may trigger). Switch first.           |
+----------------------------------------------------------------------------------------------+
| [Back]                                           [Confirm with passkey / device]             |
+----------------------------------------------------------------------------------------------+
```

**A3 Who has access?** Per space or channel: people, groups, agents with effective level and
origin (direct, group, inherited, owner).

```text
+--------------------------------+----------------------------------------------------------------+
| Who has access?                | Space Support > Channel requests                               |
+--------------------------------+----------------------------------------------------------------+
| Person or group                | Level, capabilities and origin                                 |
+--------------------------------+----------------------------------------------------------------+
| (AL) Alex                      | trigger, all capabilities      Owner                           |
| (SA) Sam                       | trigger, files       direct on space Support, until 31.12.     |
| [G] Support team (4)           | write                 group, inherited from space Support      |
| (RO) Robin                     | read                  via group Support team and direct        |
| [A] support-agent              | write (operator)      direct on space Support                  |
+--------------------------------+----------------------------------------------------------------+
| [View as person v]             | [Create grant]  [Revoke]  [History]                            |
+--------------------------------+----------------------------------------------------------------+
```

**A4 View as person.** Read-only; clear frame; every use in the audit log.

Console (M6-04, docs/decisions.d/M6-04.md D-M6-04-3): the console shows structure and access only.
Starting the view from A3 or A1 calls `viewAsUser`; the console shows the spaces and channels of
the answer as a structure list and the "Not visible" list (the spaces the owner sees and the
person does not), and keeps the banner with name and remaining time on every page until
`endViewAs` or the end time of `getConsoleSession`. A click on a channel opens nothing, because
the console has no content routes (ADR 0028); the desktop app keeps showing content read-only.

```text
+----------------------------------------------------------------------------------------------+
| [Eye] Viewing as Sam (read-only). Nothing is sent. Logged.                     [Exit]        |
+----------------------------------------------------------------------------------------------+
| Sidebar as Sam sees it:           v Support (# general, = requests, # alerts)                |
|                                   > Research (# reading-club)                                |
| Not visible for Sam: Engineering, Marketing                                                  |
| Clicking a channel shows content as for Sam; input, buttons and reactions are off.           |
+----------------------------------------------------------------------------------------------+
```

**A5 Agents with backend switch and costs.** Reported mode from the heartbeat next to the
configured mode; switching with reason and step-up; explanation of why Personal is not possible
right now (I1).

```text
+--------------------------------+----------------------------------------------------------------+
| Agents                         | support-agent                                 [online]         |
+--------------------------------+----------------------------------------------------------------+
| > support-agent  [Org.]        | Adapter      hermes        Reported mode: Organization         |
|   dev-runner     [Pers.]       | Operates     space Support (default), # alerts                 |
|   research-bot   [Org.]        | Backend      (x) Organization  ( ) Personal                    |
|                                |              Switching needs reason and confirmation.          |
| [Switch all ...]               |              Personal only works if exactly one person         |
|                                |              may trigger (I1). Today: 5 people.                |
|                                | Costs 30 days  41,20 USD (estimate)  [by space v]              |
|                                |   Support 38,10   Marketing 3,10      [Reconcile]              |
|                                | Tokens [Create token]   Commands (6)   Presence history        |
+--------------------------------+----------------------------------------------------------------+
```

**A6 Audit.** Filterable by action, person, result, resource, time range; check of the hash chain.
Export of a time range as the server's NDJSON with hashes (verifiable offline) and as CSV, which
the device converts from the same stream.

```text
+----------------------------------------------------------------------------------------------------+
| Audit   Filter: [Action v] [Person v] [Result: all v] [Time range v]       [Check chain]           |
+----------------------------------------------------------------------------------------------------+
| Time    Who       Action                    Result    Resource          Reason                     |
| 10:24   Alex      interaction.create        allowed   Card in server    approve_actions            |
| 10:20   Sam       interaction.create        denied    Card in server    missing capability         |
| 09:58   Alex      grant.create              allowed   Space Support     Cover Q4                   |
| 09:57   Alex      auth.step_up              allowed   Session laptop                               |
| 08:12   Sam       agent.invoke              denied    Run 0193..        Personal: not holder       |
+----------------------------------------------------------------------------------------------------+
| Chain checked up to no. 18 402: unchanged.  [Export] [Export as CSV]  (owner, step-up)             |
+----------------------------------------------------------------------------------------------------+
```

**A7 Statistics.** Metric tiles, runs per day, costs per agent and space, failure reasons. Values
from the server; the console shows the same view at full width.

```text
+----------------------------------------------------------------------------------------------------+
| Statistics  Time range [30 days v]  Space [all v]                         [Export]                 |
+----------------------------------------------------------------------------------------------------+
| + Runs -----------+ + Costs (Org.) ----+ + Mean duration --+ + Waiting for people --+              |
| | 1 284           | | 41,20 USD        | | 3 min 10 s       | | 12 open              |             |
| | +8 % vs prev.   | | Estimate         | | p95 18 min       | | oldest 2 days        |             |
| +-----------------+ +------------------+ +------------------+ +----------------------+             |
| Runs per day (stacked by result)             Costs per agent                                       |
|   ::::::::::::::::::::::::::::::::::::      support-agent  ##########  38,10                       |
|   done / failed / canceled                  research-bot   #           3,10                        |
| Failure reasons: project not ready 9, backend mismatch 2, time limit 1                             |
+----------------------------------------------------------------------------------------------------+
```

**A8 Hosts.** Owner only. Create a host with name, optional assignee and reason; one-time
pairing code with a command for the host; confirmation by typing the first
`terminal.fingerprint_confirm_chars` characters of the fingerprint that `bothy-host pair` shows there;
mandatory questions about the LLM login (I6) and the server host; mapping of each person to their own
session user on the host, as a choice from the names that the host reports
(`hosts.reported_session_users`; without a mapping no terminal, SR12); per session user its
work folders of the profile `claude` (ruling T13); check results of `bothy-host doctor`
with each session user or former session user that the message names (rule for names: HD6, HD10,
HD11 and HD9) and the person mapped to it, even when the
host is disconnected after a refused start ([24](24-terminal.md), section 2.2); disable,
re-pair, retire. Every change with step-up and reason.

```text
+------------------------+----------------------------------------------------------------------------+
| Administration         | Hosts > Build host                              [active]  [Disable v]      |
+------------------------+----------------------------------------------------------------------------+
|   People               | Assignee      (RO) Robin                                   [Change]        |
|   Groups               | Platform      macOS, host service 1.2.0         connected since 07:58      |
|   Grants               | Fingerprint   7KQ2 9XTA M4PB 1C8D ...   Key generation 2                   |
|   Devices              | LLM login     (x) Organization  ( ) Personal of [Person v]  ( ) None       |
|   Agents               |               Personal: only this person may open a terminal here          |
| > Hosts (3)            |               (I6), not even other owners.                                 |
|   Apps                 | Server host   ( ) yes  (x) no    Report of the host service: no            |
|   Audit                | Checks        HD1 ok  HD2 ok  HD3 ok  HD4 ok  HD5 ok  HD6 ok  HD7 warning  |
|   Statistics           |               HD8 ok  HD9 ok  HD10 ok  HD11 failed: bothy-s-alex           |
|   Instance             | Profiles      shell, claude                                                |
|                        | Grants        (RO) Robin  terminal   effective (assignee)                  |
|                        |               (SA) Sam    terminal   not effective: not assignee (SR10)    |
|                        | Session users (RO) Robin: bothy-s-robin   Work folders: server, apps       |
|                        |               (AX) Alex (Owner): bothy-s-alex   Work folders: none         |
|                        |               reported, not mapped: bothy-s-sam            [Map]           |
|                        |               without a session user no terminal (SR12)                    |
|                        | [Re-pair]  [Sessions (2)]  [Retire]                                        |
+------------------------+----------------------------------------------------------------------------+
```

Pairing a new host:

```text
+----------------------------------------------------------------------------------------------+
| Pair host                                                                Step 2 of 3         |
+----------------------------------------------------------------------------------------------+
| Run on the host as the user of the host service (one-time code, expires soon):               |
|   bothy-host pair --server https://chat.example.com --code bothy_hpc_EXAMPLE...              |
| [Copy]                                                           Expires in 14:12            |
+----------------------------------------------------------------------------------------------+
| Step 3: Type the fingerprint from the host (first characters) [7KQ2 9XTA]                    |
|            LLM login on this host  ( ) None  ( ) Personal  ( ) Organization                  |
|            Does the server run on this host?  ( ) yes  ( ) no                                |
|            Session user per person: [Person v] -> [reported name v]  [+]                     |
|            (only names that the host reported during pairing)                                |
+----------------------------------------------------------------------------------------------+
| [Back]                                           [Confirm with passkey / device]             |
+----------------------------------------------------------------------------------------------+
```

### 5.3 iPhone

Navigation stack: spaces -> channel or post list -> history; a thread row in the spaces list opens
the history directly. Back by swiping from the edge.
Input at the bottom with spacing to the safe areas. Touch targets `density.target-min` (context
`touch`). A push opens the thread directly at the message.

**M1 Spaces.** Open posts and threads nested under their channel with the rules of 6.8 and the
"+" action at forum rows (addendum of 2026-10-27, D-SB-13). A tap on a thread row opens M3
directly; a tap on the forum name opens the post list M2.

```text
+--------------------------------------+
| [^] Bothy                    [Lens]  |
| (o) Running now               (2)    |
| v [H] Support                        |
|      # general                       |
|    | = requests           * (3)      |
|      # alerts                        |
| v [<>] Engineering                   |
|      {} server     [+]      *        |
|        '- New chat (draft)           |
|        '- (o) Gateway: resume ..     |
|        '- [!] Check migration ..     |
|        '- All posts                  |
|    > {} apps                *        |
| > [M] Marketing             *        |
|                                      |
+--------------------------------------+
| [Spaces] [Mentions] [Running] [Me]   |
+--------------------------------------+
```

**M2 Post list.**

```text
+--------------------------------------+
| < Support      = requests       [+]  |
| [Open][Waiting for me][Tags]         |
+--------------------------------------+
| (o) Gateway: resume after netw..     |
|     dev-runner [sync]      4 min     |
| [!] Check migration 0012             |
|     [Waiting for you]      12:41     |
| (@) Review: grant wizard             |
| (v) Costs per space         2 h      |
|                                      |
| > Done (14)                          |
+--------------------------------------+
```

**M3 History with compact live box.** Stop is always reachable at the top right of the live box
row; tapping opens the full live box as a sheet. Approval buttons stacked at full width. The sheet
has a button "Graph" that opens M12 (addendum of 2026-10-24).

```text
+--------------------------------------+
| < server   Gateway: resume ...  ...  |
+--------------------------------------+
| (o) Running 4:12 62 %      [STOP]    |
|     go test ./internal/sync/...      |
+--------------------------------------+
| [A] dev-runner 10:16                 |
|  Approval needed: go test ...        |
|  [         Allow once         ]      |
|  [      For this session      ]      |
|  [           Deny             ]      |
| (AL) Alex 10:22 {..}                 |
|  Please include expired threads.     |
+--------------------------------------+
| [+] Follow-up to dev-runner  [Mic]   |
+--------------------------------------+
```

**M4 Large components.** Like M3: buttons at full width, forms as a sheet with keyboard; selection
as the system picker.

**M5 Record voice message.** Press and hold the microphone; swiping left discards, swiping up
locks the recording; releasing sends. Maximum duration `voice.max_duration` ([12-files.md](12-files.md), delivered by `instance.read`); a
notice before it runs out.

```text
+--------------------------------------+
| < server   Gateway: resume ...       |
|                                      |
|                                      |
+--------------------------------------+
|   ( * )  0:17   Recording            |
|   ||||||||||| ||||| ||||||           |
|                                      |
|   <- swipe to discard                |
|   ^  swipe up to lock                |
|   [Release = send]                   |
+--------------------------------------+
| [Press and hold Mic]                 |
+--------------------------------------+
```

**M6 Share from other apps (choose target).** Separate sheet of the Share Extension: preview,
comment, target from recently used threads and channels with write permission, search for more.
Sending goes directly; without network the entry lands in the app's queue.

```text
+--------------------------------------+
| Cancel      Share to       [Send]    |
+--------------------------------------+
| [Image] screenshot.png  1,2 MB       |
| Comment [See error above         ]   |
+--------------------------------------+
| Target                               |
| > = server > Gateway: resume ..      |
|   = requests (new post)              |
|   # general                          |
|   [Lens] Search others ...           |
+--------------------------------------+
| Only targets with write permission.  |
+--------------------------------------+
```

**M7 Offline.** Banner at the top; waiting messages with "sending"; permanent errors with
reason and buttons; nothing is discarded silently.

```text
+--------------------------------------+
| < server   Gateway: resume ...       |
| [Cloud] Offline. 1 message waiting   |
+--------------------------------------+
| (AL) Alex 10:22 {..}                 |
|  Please include expired threads.     |
| (AL) Alex                            |
|  And summarize afterwards.           |
|  {z} sending                         |
| (AL) Alex                            |
|  Attachment too large. {x} failed    |
|  [Retry] [Edit] [Discard]            |
+--------------------------------------+
| [+] Message (will be sent later)     |
+--------------------------------------+
```

**M8 Terminal with extra keys bar.** Full-screen view of a session. Above the system keyboard
sits the extra keys bar: Esc, Tab, Ctrl, Alt, arrows, `|`, `~`, `/`, `-`. Ctrl and Alt
act once for the next key; tapping twice holds them until the next tap (indicator
filled). With a hardware keyboard the app hides the bar. Swiping with two fingers scrolls
the scrollback; a long press selects text for copying. When the app goes to the background, it
detaches the session (it keeps running on the host) and hides the view in the app switcher; back
in the app it reattaches within `terminal.quick_reattach_window` without a new confirmation,
after that with Face ID or fingerprint.

```text
+--------------------------------------+
| < Hosts     Build host      [Org.]   |
| shell 1  v   Left 11:42   [Detach]   |
+--------------------------------------+
| ~/src/server $ git status            |
| On branch main                       |
| nothing to commit, working tree clean|
| ~/src/server $ tail -f run.log       |
| 12:04:31 sync: resume ok             |
| 12:04:32 sync: catch-up 14 events    |
| _                                    |
+--------------------------------------+
| [Esc][Tab][Ctrl][Alt] [<][^][v][>]   |
| [ | ] [ ~ ] [ / ] [ - ]     [Keybd.] |
+--------------------------------------+
|  q  w  e  r  t  z  u  i  o  p        |
|   a  s  d  f  g  h  j  k  l          |
|  [^]  y  x  c  v  b  n  m  [<-]      |
|  [123]   [    Space     ]   [Enter]  |
+--------------------------------------+
```

**M9 Hosts and sessions.** List of the hosts that the person may see, with connection,
badge of the LLM login and own sessions; tapping a session attaches.

```text
+--------------------------------------+
| < Me         Hosts                   |
+--------------------------------------+
| (o) Build host           [Org.]      |
|     shell 1     detached  3 min ago  |
|     claude: server   active          |
|     [+ New session]                  |
| ( ) Linux VM             [None]      |
|     offline since 08:12              |
+--------------------------------------+
| Fingerprint and login: [i]           |
+--------------------------------------+
```

**M10 Live.** The same views as D12 from `bothy_viz` in density `touch`, one tab per view:
topology, backend, delivery, executive, timeline. The topology stacks the lanes vertically;
tapping a run card opens the thread. Every view has the table view. The live stream
runs only in the foreground. No TV mode. Normative: [25-visualization.md](25-visualization.md),
section 6.

```text
+--------------------------------------+
| < Me    Live      12:04:31   live    |
| [Topo][Backend][Deliv.][Exec][Time]  |
+--------------------------------------+
| Engineering                          |
|  = server                            |
|   +- dev-runner -------------------+ |
|   | (o) running 4:12   [.. Bash]   | |
|   | [Org.] 0,62 USD estimate       | |
|   +--------------------------------+ |
| Support                              |
|  = requests                          |
|   +- support-agent ----------------+ |
|   | [!] waiting_input              | |
|   | Approval waiting  [Personal]   | |
|   +--------------------------------+ |
+--------------------------------------+
| Last minute: runs 3 | errors 0       |
| [Table]                              |
+--------------------------------------+
```

**M11 Running now (control room).** Addendum of 2026-10-24 (run visualization). D10 in density
`touch` from the tab "Running": cards in one column, the switch "Cards" and "List", filters as a
sheet. A tap on a card opens M12; Stop where allowed. Visible cards are live only while the app is
in the foreground. Data: [09-agents.md](09-agents.md), section 5.6.

```text
+--------------------------------------+
| Running now (3)      [Cards | List]  |
| [Space v] [Agent v] [mine only]      |
+--------------------------------------+
| +- dev-runner ---------------------+ |
| | (o) running 4:12  0,62 USD est.  | |
| | server > Gateway: resume ...     | |
| | [.. Bash] go test ./...          | |
| | 2 subagents, 1 background job    | |
| |                         [STOP]   | |
| +----------------------------------+ |
| +- research-bot -------------------+ |
| | (~) running 31:40                | |
| | no movement since 12:41          | |
| +----------------------------------+ |
+--------------------------------------+
| [Spaces] [Mentions] [Running] [Me]   |
+--------------------------------------+
```

**M12 Run graph.** Addendum of 2026-10-24 (run visualization). D14 in density `touch`: the same
rows, indented, with a narrow time bar below each label; in landscape the bars stand beside the
labels on the full time axis. A tap on a row collapses or expands it; "As list" as in D14.

```text
+--------------------------------------+
| < Running   Run graph   [As list]    |
| (o) dev-runner  4:12  0,62 USD est.  |
+--------------------------------------+
| (o) dev-runner                       |
|     [==============================] |
|   (o) Subagent  Write test cases     |
|       [   ===================>     ] |
|     (o) Bash  go test ./...          |
|       [          ==========>       ] |
|   (o) Background job  npm test       |
|       [      ==================>   ] |
|   (x) Subagent  Check migration      |
|       [ =====                      ] |
|   3 earlier steps [show]             |
+--------------------------------------+
| [STOP]                               |
+--------------------------------------+
```

**M13 Emergency.** Addendum of 2026-10-25 (M5-09; R-PR-07). The entry "Emergency" under Me
shows only for an owner or for a person who manages at least one space (the owner flag of
`getMe` and the capability `manage` the server reported for a space). It lists people with a
search; a person shows their devices with "Revoke" and "Disable person" (owners only) and their
access grants with "Revoke" (the grants the server lists for the caller). Each action opens a
sheet that says what happens and that it takes effect at once; the button names the step-up
("Confirm with Face ID", on Android the fingerprint or screen lock), and a cancelled step-up
sends nothing. Stopping a run stays in the live box (M3) and in Running now (M11). Nothing else
of the administration is on the phone.

```text
+--------------------------------------+
| < Emergency                          |
| (SA) Sam  @sam  [Active]             |
| [        Disable person            ] |
| Devices                              |
| Pixel  Android  today 08:12 [Revoke] |
| Access grants                        |
| Space Support                        |
| Write, until Dec 31, 2026            |
|                             [Revoke] |
+--------------------------------------+
| Revoke the access grant for Sam?     |
| [!] The access grant of Sam on Space |
|     Support is revoked at once.      |
| [Reason                            ] |
| [      Confirm with Face ID        ] |
| [             Cancel               ] |
+--------------------------------------+
```

### 5.4 Android

Same screens as iPhone, with Android controls: system back, top app bar, notification
channels of the system, sharing via the system share sheet into the same target selection. In
the terminal (M8), system back detaches the session instead of ending it; the extra keys bar is
the same.

```text
+--------------------------------------+
| [<-] Gateway: resume ...   [:]       |
+--------------------------------------+
| (o) Running 4:12 62 %      [STOP]    |
| ...                                  |
+--------------------------------------+
| System back: to the post list        |
| Notification channels in the         |
| system settings: Mentions,           |
| Questions, All messages, Runs        |
| Share: system sheet -> app ->        |
|   same target sheet as iOS           |
+--------------------------------------+
```

### 5.5 Web console (administration, statistics and terminal)

Same administration screens as A1 to A8, the views from D12 and D13 and the terminal from D11,
navigation on the left. No chat (ADR 0028); links to threads open the app via deep link. The run
graph D14 and the control room D10 are not part of the console (addendum of 2026-10-24): both
show run content that the console, without chat, has no place for; D12 stays its live view. The terminal
opens only after confirmation with a passkey with user verification; other step-up methods of the
console do not count for it ([ADR 0070](../adr/0070-terminal-in-the-web-console.md)).

```text
+--------------------------+------------------------------------------------------------------------+
| [^] Bothy console        | Statistics                    Logged in: Alex (passkey)                |
+--------------------------+------------------------------------------------------------------------+
|   Live                   | (like desktop statistics, full width)                                  |
| > Statistics             |                                                                        |
|   Hosts and terminal     | No chat in the browser. Links in threads open the app:                 |
|   People                 | [Open in the app]  (deep link)                                         |
|   Groups, grants         |                                                                        |
|   Devices                | Terminal: opens only with passkey; pinned host keys are kept in        |
|   Agents, costs          | the storage of this browser and are deleted at logout.                 |
|   Apps, webhooks         |                                                                        |
|   Audit                  |                                                                        |
|   Import                 |                                                                        |
|   Instance, branding     |                                                                        |
|   Status                 |                                                                        |
+--------------------------+------------------------------------------------------------------------+
```

### 5.6 Onboarding and sign-in (all apps)

Screens E1 to E7 apply equally to desktop, iPhone and Android; the flows behind them are in
[08-identity.md](08-identity.md). On the phone they fill the screen; on the desktop they appear
as a card in the center of an empty window with the branding of the instance. The wireframes show
the phone form, E2 also the desktop form. The web console uses only E2 and E7: it has
no device confirmation, but requires a step-up after the login (08, section 7.5).

Rules for all onboarding screens:

- No content of the instance before the login, only name, logo and accent from
  `getDiscovery` (`branding`).
- Every code is set in groups, in a mono font (`font.family.mono`), can be copied and
  is read character by character by the screen reader. Input fields for codes accept pasting and
  ignore case, spaces and hyphens (08, section 3.2).
- Remaining times appear as text, not only as bars; the screen reader announces them at most once
  per `a11y.live_announce_min_interval`.
- Error texts name the cause and the next step; the response to a wrong code is
  always the same (08, section 3.2).
- Keyboard: Enter triggers the main action, Esc goes back one step; the focus order follows the
  reading direction.

**E1 Server address (first start).** One field for the address of the instance. "Connect" calls
`getDiscovery`. Results: on to E2; "No instance at this address" (no
valid response); "Certificate invalid" (no continue, no exception button); "Instance not set up
yet" on `setup_required` with a pointer to the setup wizard on the host
([16-operations.md](16-operations.md), section 3.1); "App too old" below `min_client_versions` with the
update path ([18-release.md](18-release.md)). The app remembers the address; a second server is
not planned (B13).

```text
+--------------------------------------+
|                                      |
|              [Logo]                  |
|   Connect to your instance           |
|                                      |
|   Server address                     |
|   [https://chat.example.com      ]   |
|                                      |
|   [          Connect            ]    |
|                                      |
|   Instance not set up yet:           |
|   The owner sets it up on the        |
|   host (install.sh). [More]          |
+--------------------------------------+
```

**E2 Login.** Name and logo of the instance at the top. Below them, one button per provider from
`auth_providers` with its `name`: OIDC opens the system browser (08, section 14.1); a proxy provider
(`trusted_header`) logs in without input (08, section 2.4). If a `local` driver is active: a field
for the login name, "Log in with passkey" (native apps only with a public domain, 08,
section 2.5) and, if `local_password_enabled`, "Password and one-time code" with a second step for
the TOTP code (autofill for the system's one-time codes on). At the bottom always "I have an
enrollment code" (E3) and "Pair with another device" (E5). If no method exists, it says
"No login is set up for this instance" with a pointer to the owner.

```text
+--------------------------------------+
| < Server        chat.example.com     |
+--------------------------------------+
|              [Logo]                  |
|        Log in to Team Chat           |
|                                      |
|   [   Log in with Company SSO    ]   |
|   -------------- or --------------   |
|   Login name  [alex             ]    |
|   [     Log in with passkey      ]   |
|   [  Password and one-time code  ]   |
|                                      |
|   I have an enrollment code          |
|   Pair with another device           |
+--------------------------------------+
```

Desktop: the same card in the center; the sidebar stays empty until the login succeeds.

```text
+------------------------------------------------------------------------------------+
|                                                                                    |
|                    +----------------------------------------+                      |
|                    |                [Logo]                  |                      |
|                    |        Log in to Team Chat             |                      |
|                    |   [     Log in with Company SSO      ] |                      |
|                    |   ---------------- or ---------------- |                      |
|                    |   Login name [alex                   ] |                      |
|                    |   [       Log in with passkey        ] |                      |
|                    |   [    Password and one-time code    ] |                      |
|                    |   Enrollment code   Pair device        |                      |
|                    |   Server: chat.example.com  [Change]   |                      |
|                    +----------------------------------------+                      |
|                                                                                    |
+------------------------------------------------------------------------------------+
```

**E3 Redeem enrollment code.** For the first device and for recovery (08, section 3.2).
First the code, then the method that the instance offers: "Create passkey", "Set up password and
one-time code" (two steps with a QR code of the one-time code and confirmation, 08, section 2.5) or
"Link with login service" (OIDC or proxy). A redeemed code makes this device
`active` immediately; the app goes straight to the main view.

```text
+--------------------------------------+
| < Login        Enrollment code       |
+--------------------------------------+
|   Code from the owner                |
|   [en_ 7K2M-9QXA-...            ]    |
|                                      |
|   How do you want to log in?         |
|   (x) Create passkey                 |
|   ( ) Password and one-time code     |
|   ( ) Link with Company SSO          |
|                                      |
|   [          Continue           ]    |
|   The code is valid once.            |
+--------------------------------------+
```

**E4 Device waiting for confirmation.** After a login on a new device (08, section
5.2). The confirmation code appears large in the center (`auth.device_verification_code_digits`
digits), below it the instructions: type in the code on an active device under Settings, Devices,
or ask an owner. The remaining time until `auth.device_pending_ttl` appears as text.
"Cancel" logs out and deletes the keys of the device. The app polls the state; after the
confirmation it goes to the main view, after rejection or expiry to E7.

```text
+--------------------------------------+
|     Device waiting for confirmation  |
+--------------------------------------+
|                                      |
|           4 8 1   2 0 7              |
|                                      |
|   Type this code on one of your      |
|   active devices: Settings,          |
|   Devices and sessions, Confirm.     |
|   No active device: ask the owner.   |
|                                      |
|   Valid for 23 more minutes          |
|   [          Cancel             ]    |
+--------------------------------------+
```

**E5 Pair with another device.** The new device shows a pairing code with
`auth.pairing_code_chars` characters and the remaining time until `auth.pairing_code_ttl` (08, section
5.3). On the active device, "Add device" (D8, Devices and sessions) opens a field for the
code and the step-up. After that the new device is `active`.

```text
+--------------------------------------+
| < Login        Pair device           |
+--------------------------------------+
|   On an active device:               |
|   Settings > Devices >               |
|   Add device, then enter:            |
|                                      |
|           HQ7P - 3MXR                |
|                                      |
|   Valid for 4 more minutes           |
|   [          New code           ]    |
+--------------------------------------+
```

**E6 App lock.** After `client.app_lock_after` without use and at start, if the person has turned
it on or `security.app_lock_required` applies (08, section 9). The content is
hidden, also in the app switcher. A button calls the system's biometrics or device PIN; after
a failure the lock stays with "Try again". "Log out" leads to E7 and deletes the
local data. The lock detaches terminal sessions (they keep running on the host, 5.3, M8).

```text
+--------------------------------------+
|                                      |
|              [Logo]                  |
|           Team Chat is locked        |
|                                      |
|   [    Unlock with Face ID      ]    |
|                                      |
|              Log out                 |
+--------------------------------------+
```

**E7 Logged out.** After logout, revocation of the device, expiry of the session
(`auth.session_max_age`), disabling of the person or rejection in E4. The reason appears as text
(`devices.revoke_reason` or the code of the session, without internal names), together with the
sentence "The data of this device has been deleted" (04, section 8.5, R-SY-14) and "Log in again"
(E2). For a disabled person there is no login button, only "Contact the owner".

```text
+--------------------------------------+
|                                      |
|   You are logged out.                |
|   Reason: This device was revoked    |
|   by another device.                 |
|                                      |
|   The data of this device has        |
|   been deleted.                      |
|                                      |
|   [         Log in again        ]    |
+--------------------------------------+
```

## 6. Interaction rules

### 6.1 Done and reopening

- "Done" (`thread.close`) in the thread header, in the context menu of the post list and via shortcut.
- Done posts are at the bottom in the section "Done (n)", collapsed by default; the
  collapse state is personal per forum.
  The collapse state is kept per forum on each device in the local database and is not
  synchronized, because 11 has no setting key for it (docs/DECISIONS.md D-M4-19-4).
- A new message in a done thread reopens it (`thread.reopen` implicitly). The
  history shows a system line "Reopened by message from Sam". A system line itself does not
  reopen a done thread ([03-data-model.md](03-data-model.md), R-DM-42).
- The word "done" as a message is not a command; the way is the button or a command that an
  agent registers.

### 6.2 Live box

- While a run is active, the live box is pinned at the top of the thread; at the end it collapses
  into one line at its place in the history ("Ran 28 min · 3 agents · USD 1.20 estimate").
- Expanding loads the run document; during the run, changes arrive only while the
  thread is visible (focus, [04](04-sync-protocol.md) section 11).
- Stop asks for no confirmation; it is harmless and must be fast.
- The button "Graph" opens the run graph (D14, M12) with the same document; it follows the same
  focus rule (addendum of 2026-10-24).
- A stall after `run.stall_after` shows a notice in the live box, no push.
- Follow-ups: during an active run, the input is labeled "Follow-up to <agent>". The trigger state
  of the message changes to "Follow-up delivered" as soon as the adapter has handed it over.
- Retry (addendum of 2026-10-30, [09](09-agents.md) 5.7): the line of a `failed` or `cancelled` run
  that is the newest run of its thread has a button "Retry" (German "Erneut versuchen") on the
  desktop apps (macOS, Windows) and the mobile apps (iOS, Android). It starts a new run of the
  same agent for the same message through the outbox (`retryRun`), without writing the message
  again; one activation sends once, and the button stays disabled while the request runs and
  after it was accepted. A refusal shows its reason (for example "The agent of this thread no
  longer works in this channel"). After `interrupted` and `stale` the line keeps "Send again"
  for the person who triggered (09 5.5) instead.

### 6.3 Trigger state as icons

Instead of status emoji, the triggering message shows an icon with a label in the tooltip and in
the screen reader label. Icons and colors: [components.md](../design/components.md) section 5.

| `trigger_state` | Meaning for the person |
|---|---|
| `received` | The server has accepted the message. |
| `queued` | A run has been created and is waiting. |
| `working` | The agent is working. |
| `done` | The run is finished. |
| `failed` | Triggering or the run has failed; the reason is shown with it. |
| `followup_delivered` | The follow-up has reached the running agent. |

### 6.4 Long replies

Messages over `ui.message_collapse_lines` lines are collapsed with "Show more";
code blocks wrap long lines by default and can be switched to a horizontal scroll, and a code
block higher than `ui.code_block_collapse_lines` lines shows collapsed with "Show all" (6.14,
addendum of 2026-10-31; this replaces the earlier rule that code blocks scroll horizontally
instead of wrapping). Long replies are never split across several
messages; one fenced code block renders as one block however long it is. A text larger than
`message.max_content_bytes` goes out as one message whose rest is one text attachment (6.13).

### 6.5 Components

Buttons apply once and only for those authorized. Whoever is not authorized sees them disabled
with a reason ("Only the triggering person or an owner"). After use, the card shows person, choice
and time. Expired components (`component.default_ttl`) show "Expired". Double clicks do not create
a second interaction.

### 6.6 Offline and queue

- Banner "Offline" at the top of the content; connecting runs in the background with backoff.
- Sent messages appear immediately with "sending" at the end of the thread.
- Permanent errors show the reason with "Send again" (new ID), "Edit", "Discard".
- If the person loses access, open views close with "No access anymore"; waiting
  messages remain as text for copying.
- After reconnecting, the app first catches up and then sends ([04](04-sync-protocol.md)
  section 13.4).

### 6.7 Other rules

- **Not found instead of forbidden.** A link to something invisible shows "Not found or
  no access", regardless of whether it exists (B7).
- **Mention suggestions** show only people and agents that the person may see.
- **Titles:** automatically set titles carry a small "automatic" mark; editing makes the title
  manual.
- **Operator:** The channel header names the operator. Without an operator it says "No agent
  answers here"; so an agent never answers in the wrong channel unnoticed.
- **Costs:** in Organization mode with "Estimate" until reconciled; in Personal mode with
  "Information only".
- **Destructive actions** (delete a message, revoke a grant, revoke a device) ask for confirmation
  once; administration actions with step-up ask for the confirmation of the device.
- **Move:** The dialog shows only channels with the same operator; the thread ID stays.
- **Each device navigates on its own** (addendum of 2026-10-29, R-PR-24). What a device shows
  is its own: the route (open space, channel, thread or view), the selected sidebar row, the open
  and hidden columns, which spaces and thread groups of the sidebar are collapsed and the scroll
  position are never written to the server and never follow another device of the person. Only
  data crosses devices (04 14): messages, read state and the counters it gives, drafts, settings,
  the personal order of the spaces and notification preferences. Expanding a space on the phone
  to open a chat leaves the space as it was on the desktop; an upgraded app keeps the collapse
  state it showed last, a new device starts with every space expanded. Opening a thread on one
  device marks it read there; the other device updates its
  counters and stays where it is. Only an action of the person on that device changes its route:
  a click in the app, a clicked notification or push, a link.

### 6.8 Threads in the sidebar

Addendum of 2026-10-27 (R-PR-17, rulings D-SB-1 to D-SB-13,
[design note](../design/sidebar-sessions.md)). Applies to the desktop sidebar (D1) and to the
spaces screen on the phone (M1).

Addendum of 2026-10-29 (R-PR-23, rulings D-M4-46-1 to D-M4-46-5): a forum lists every open post,
like Discord, ordered by last activity, with "Show more" beyond a limit (items 2 to 4); text
channels keep their rule. Reason: owner decision of 2026-10-29; after an import most open posts of
a forum had no activity within `ui.sidebar_thread_window`, so a forum with 24 open posts showed
none of them.

1. **Group.** Under each forum and each text channel the sidebar shows a group of thread rows
   (forum posts, threads from messages of the main thread), indented with a tree line. The main
   thread of a text channel is the channel row itself.
2. **Which threads.** In a forum every open post shows (every post the client holds: the active
   threads of the snapshot, 04 6.3 item 4, the open posts the client lists for each forum after a
   full snapshot, the posts that events brought and the pages of the forum overview). In a text channel a thread shows when it is open and at least one holds: it
   is selected; it has an active run; it waits for the person ("Waiting for me": a run the person
   triggered in `waiting_input`, or an unread mention of the person); it is unread; its
   `last_activity_at` lies within `ui.sidebar_thread_window`. A done thread shows only while it
   is selected.
3. **Bound.** A forum group shows the newest `ui.sidebar_forum_posts_visible` open posts and the
   selected one; when the forum has more open posts, the row "Show more" above "All posts" lists
   all of them in place and then reads "Show less". Whether a forum group shows all is kept per
   device like the collapse state (item 7). A text channel has at most
   `ui.sidebar_threads_per_channel` thread rows. When more qualify, they are chosen in this
   order: selected, waiting for the person, active run, unread mentions, unread, newest activity.
4. **Order.** In a forum by last activity (`last_activity_at`), newest first; in a text channel by
   creation time, newest first. A row that no longer qualifies disappears, and the rows of a forum
   change their order, only while the pointer is outside the sidebar and the keyboard focus is
   not in it; until then the rows stay in place, and a post new to the group appears at the top.
   "Show more" and "Show less" apply at once.
5. **"All posts".** An expanded forum group ends with the row "All posts", which opens the forum
   overview D15; it carries the unread mark and the mention counter of the posts not shown in the
   group, never a total. Text channels have no such row.
6. **Markers.** A thread row has the states of a channel row (components.md 7) and, in front of
   the title, the run state icon (running, waiting for an answer, failed) instead of the run dot. A
   title that is still empty reads "New chat" with the "automatic" mark (6.7, 09 12). The channel
   row of an expanded group counts unread and mentions only of its threads that are not shown and
   of its main thread; a collapsed group counts all; the run dot of a channel covers all its
   threads.
7. **Collapse.** Each channel with thread rows has its own arrow; groups are expanded by default.
   The collapse state of a group, whether a forum group shows all its open posts (item 3) and
   whether the forum overview is pinned as a column are kept per device in the local database and
   are not synchronized (like the Done section in 6.1).
8. **New chat.** Every forum row has the action "+" (visible on hover, on keyboard focus and on the
   selected forum; always on touch), labelled "New chat in <forum>" in a forum with a project
   binding and "New post in <forum>" otherwise; it is also in the context menu of the forum, and
   Cmd+N (Ctrl+N) starts it in the forum of the focused or selected row. It opens the new chat view
   in the content column without a dialog: the input has the focus; the title is optional when the
   forum has an operator agent (the agent names the chat, 09 12) and required otherwise; tags are
   optional. At the top of the forum's group a draft row "New chat" appears and is selected. Its
   text is the person's draft of a new post in this forum (03 `drafts` with `thread_id` NULL, one
   per person and forum), so it survives a restart and appears on the person's other devices; "+"
   on a forum that has such a draft opens it instead of a second one. Sending creates the post with
   its first message through the outbox (`thread.create`); the draft row becomes the post row in
   place. An empty draft is discarded when the person leaves the view.
9. **No worktree per post.** A new chat in a project forum is a new agent session in the same
   project folder; the app and the runner create no git worktree for it (09 18.1).
10. **Dragging.** Thread rows and channel rows cannot be dragged; the personal order of spaces stays
    (components.md 7). Moving a thread stays the dialog of 6.7.
11. **Kind of a channel.** The channel icon (`channel.text`, `channel.forum`, `channel.project`) has
    a tooltip and a screen reader text that explain the kind in one sentence (section 4); the
    channel header shows the same sentence as its subtitle, and an empty forum shows it as its
    empty state.

### 6.9 New project

Addendum of 2026-10-28 (R-PR-20, rulings D-M4-43-1 to D-M4-43-12,
[design note](../design/new-project.md)). Applies to the desktop sidebar (D1); the phone shows the
result like any forum.

1. **Entry.** A space row has the action "New project" (visible on hover, on keyboard focus and on
   the selected space), and the space menu has the same entry. Both show only for an owner, because
   binding a project is an owner action with step-up (05 `channel.project.bind`).
2. **Dialog.** A short dialog with four modes: "Empty project" (a new folder with `git init`),
   "Clone repository" (an `https` URL), "Existing folder" (a folder name under the runner's
   projects folder) and "Start from an idea" (a multi-line text; the project is created empty and
   the idea becomes its first chat). Fields: the name (suggested from the URL or the idea, at most
   `ui.new_project_name_max_chars` characters, editable; used for the forum and the folder), the
   runner (the active agents, default the space's default operator) and the mode's own field. The
   dialog checks the name (letters, digits, `.`, `_`, `-`, starting with a letter or digit) and the
   URL (`https` with a host) before it sends.
3. **Steps.** "Create" runs, in order: create the forum (`channel.create`), assign the runner when
   it is not the space's default operator (`channel.operator.set`), bind the project
   (`channel.project.bind` with `provision` `init`, `clone` or `none`), wait until the runner
   reports the project ready (`getChannelBehavior`, every `ui.new_project_poll_interval`), and for
   an idea post it (`thread.create`). The dialog lists the steps with the states waiting, running,
   done and failed; while the runner works the folder step reads "Creating folder" or "Cloning".
   A step-up prompt appears where an action needs one.
4. **Idea.** The idea is first stored as the person's draft of a new post in the new forum (6.8
   item 8) and sent with an empty title once the project is ready, so the agent answers in the new
   chat and names it. If the dialog closes before, the idea stays as that draft.
5. **Failure.** A failed step shows its name and one sentence for its cause: a taken name, a
   folder already bound to another forum, a refused step-up, a folder that already exists, a
   failed clone, a missing folder, a folder that is not a Git repository (only from a runner
   before the addendum of 2026-10-30, which treats Git as optional), a runner that has not
   answered. The fields of the steps not done yet stay editable (for example the URL after a
   failed clone, the folder name after a taken folder); "Try again" resumes at the failed step with
   them and creates nothing twice; "Close" keeps what exists.
6. **Waiting.** When the project is not ready after `ui.new_project_wait`, the dialog says that the
   runner has not finished and that the forum becomes ready by itself; the person may close it.
7. **Done.** The dialog closes and opens the new forum, for an idea the new chat; the forum shows
   as a project in the sidebar (6.8 item 11).

### 6.10 Forums for existing folders

Addendum of 2026-10-29 (R-PR-21, rulings D-M4-44-1 to D-M4-44-7,
[design note](../design/new-project.md) section 6). Applies to the desktop sidebar (D1), owners
only, like 6.9.

1. **Entry.** The space menu has the entry "Add forums for existing folders" below "New project
   in".
2. **List.** The dialog has the runner field of 6.9 (the active agents, default the space's
   default operator) and lists the folders that this runner reported under its projects folder
   and that no forum is bound to (`listAgentProjectFolders`, [09-agents.md](09-agents.md) 18.13),
   in name order, each with a checkbox, all checked. A changed runner loads its list again. An
   empty list says that every folder already has a forum; a runner that never reported its
   folders is named so (an older runner or one not connected yet). Nothing is created before the
   person presses "Add forums"; the list is the preview of what will be created.
3. **Steps.** For each checked folder in list order: create a forum named like the folder
   (`channel.create`), assign the runner when it is not the space's default operator
   (`channel.operator.set`), bind the folder (`channel.project.bind` with `provision` `none` and
   the folder name as `workspace_ref`). Then the dialog reads the readiness of all bound forums
   every `ui.new_project_poll_interval` until each is ready or names a reason. Every row shows its
   state (waiting, running, done, failed) and, on a failure, the sentence of 6.9 item 5; the other
   folders go on. A step-up prompt appears once, where the first action needs it.
4. **Resume.** "Try again" runs the failed rows again from their failed step with the forum they
   already have, so no folder gets a second forum; "Close" keeps what exists. A folder that got a
   forum since the list was loaded fails with "already bound" and its new forum stays a plain
   forum, as in 6.9.
5. **Waiting.** Rows not ready after `ui.new_project_wait` say that the runner has not finished;
   the person may close the dialog and the forums become ready by themselves.

### 6.11 Rename project

Addendum of 2026-10-29 (R-PR-21, D-M4-44-6, D-M4-44-7). The menu of a forum with a project binding
(`Channel.project_bound`) has "Rename project" for a person with `manage` on the forum
(`channel.update`). It opens a short dialog with the current name; "Rename" changes the forum's
name (`updateChannel` with the forum's version as `If-Match`). The folder, the binding and the
sessions stay as they are. The name is trimmed and not empty; the server checks its length
(`channel.name_max_chars`, 03) and a refusal shows its sentence in the dialog, and a changed version
(`version_conflict`) reads the forum again and asks the person to confirm the new name once more.

### 6.12 Copying blocks of a message

Addendum of 2026-10-29 (R-PR-25, rulings D-M4-48-1 to D-M4-48-7). Reason: owner request of
2026-10-29; in Discord a tap on a code block on the phone never copied it and "Copy text" always
copied the whole message, so a command or a quoted passage had to be cut out of it by hand.
Applies to every message the renderer draws, on the desktop (macOS, Windows) and on the phone
(iOS, Android).

1. **Desktop.** Every code block (fenced or indented) and every quote block has a small copy
   button at its top right ("Copy code", "Copy quote"); it copies exactly that block and confirms
   with "Copied". It is a button for the keyboard and the screen reader, and its label is never
   part of a text selection. A click on the text of a block copies nothing.
2. **Phone.** A tap on a code block or a quote block copies exactly that block, with a short
   "Copied" and a light haptic; the block has no button. A long press on the message opens the
   message menu (the menu of a right click on the desktop), where "Copy text" copies the whole
   message. A tap on a link inside a
   block opens the link, a horizontal drag scrolls a code block; neither copies. A screen reader
   copies a block through its action "Copy code" or "Copy quote".
3. **Code block.** The copy is the raw content between the fences, byte for byte: indentation,
   tabs, trailing spaces, blank lines, every Unicode character and long lines stay as they are;
   the fence lines and the language tag are not part of it, and nothing of the rendering (soft
   wraps, horizontal scroll) is. Line breaks are LF. Trailing line break rule: only the line break
   that ends the last content line is dropped, so a one-line block copies without a line break
   and a block that ends in blank lines keeps them; an unclosed fence copies to the end of the
   message. An indented fence removes only its own indentation (CommonMark).
4. **Quote block.** The copy is the quote without its `>` markers. As plain text it keeps the
   line breaks of the source and gives inline Markdown as its text (emphasis, inline code and
   links as their text, an image as its alt text); blocks are separated by an empty line, a list
   item starts with "- " or its number, a task with "[x] " or "[ ] ", a code block keeps its raw
   content and a table row is its cells separated by a tab. Where the platform clipboard holds
   rich text (all four apps: the HTML flavor of the pasteboard on macOS and iOS, "HTML Format"
   on Windows, an HTML clip on Android) the same copy also carries an HTML form, so a paste into
   a rich editor keeps emphasis, code, lists, tables and the `https`, `http` and `mailto` links;
   any other link and every image are text only, so a paste loads nothing. If the rich form
   cannot be written, the plain text still is.
5. **Nesting.** A block inside another copies only itself: the code block or quote inside a
   quote copies its own content, the outer quote copies all of its content as text.
6. **Selection.** Selecting part of a message with the system selection, where a view offers it,
   is not changed by the copy targets: a tap is the block's, a long press and a drag stay with
   the selection and the message.

### 6.13 Long texts

Addendum of 2026-10-29 (R-PR-26, rulings D-M4-47-1 to D-M4-47-6). Reason: owner decision of
2026-10-29; a person must never hit a length limit, unlike the 2000 characters of the former chat
tool that forced splitting a text or a paid plan.

1. **Long paste.** In the composer of a thread and of a new post (desktop and phone), a paste of
   more than `ui.composer_paste_file_chars` characters or more than `ui.composer_paste_file_lines`
   lines does not go into the field: it becomes the attachment `paste.txt`, shown as a file chip
   above the field. A paste that is exactly one fenced code block with a known language becomes
   `paste.<extension>` with the code of the block (for example `paste.py`); text that is clearly a
   file (a script with `#!`, JSON, XML, HTML, a diff) gets that extension. The chip has "Insert as
   text", which removes the file and puts the pasted text into the field at the cursor. Typed
   text, templates and quotes never become a file. A paste the attachment rules refuse (count,
   size) stays text and the refusal shows its reason. Text that an input source outside the app
   puts into the field (a dictation app or its keyboard extension, the system dictation or voice
   typing, a paste of the system menu or of an input method, the paste key of a hardware
   keyboard) counts as typed or pasted by what it inserts at once: it shows at once, enables
   send and is saved as the draft; inserted at once over either bound it becomes `paste.txt`,
   while a dictation that grows in small steps stays text as typing does (DICTATION-MOBILE).
2. **Text over the limit.** While the text of the composer is larger than
   `message.max_content_bytes` (UTF-8 bytes), a note above the field says that the rest goes out
   as `message.txt` with this message. Sending never fails with a length error: the message
   carries the head (cut at a line break, never inside a character, never inside a fenced code
   block, which moves whole into the rest) and the rest as the one attachment `message.txt` of the
   same message; head and attachment together are the text. A message is never split into
   several. Only when that attachment is refused (the count of attachments is full) the composer
   shows the limit as before and keeps the text.
3. **Agents.** The SDKs (Python `bothy_sdk.long_answers`, TypeScript `sendLongMessage`), the
   Hermes adapter and the Claude Code runner send a long answer the same way: one message, the
   rest above the limit as one attachment `answer.txt`; they never split an answer into several
   messages. When the attachment cannot be made, the head goes out with a short note, still one
   message ([09-agents.md](09-agents.md) 17.3, 18.10).
4. **Edits.** Editing a message keeps the limit of `message.max_content_bytes`: an edit carries no
   attachments ([06-api.md](06-api.md) `MessageUpdate`).

### 6.14 Readable code blocks

Addendum of 2026-10-31 (R-PR-28, rulings D-M4-49-1 to D-M4-49-6). Reason: owner decision of
2026-10-31, which overrides the former rule of 6.4 that code blocks scroll horizontally instead of
wrapping. An agent sent a long German sentence as a fenced block "to copy"; the macOS app showed it
as one cut-off line that could not be scrolled sideways (no visible scrollbar, the trackpad did not
move it) and could not be expanded, so the owner could not read what he would copy. Applies to
every code block the renderer draws, on the desktop (macOS, Windows) and on the phone (iOS,
Android).

1. **Wrap by default.** A code block (fenced or indented) wraps its long lines, so its whole
   content is visible without scrolling. A small toggle "Wrap lines" in the header of every block,
   beside the copy button on the desktop, switches to no wrap: every line stays one line and the
   block scrolls horizontally, with a visible scrollbar on the desktop that can be dragged, shift
   and the mouse wheel, a sideways trackpad gesture, and a drag on the phone. The choice is kept per
   device and applies to every code block of that device (local view state, never synchronized);
   without a local database (before the sign-in) the toggle switches only its own block. A tap on
   the toggle on the phone never copies the block (6.12 item 2).
2. **Long blocks.** A code block higher than `ui.code_block_collapse_lines` lines of code (the
   rendered height, so wrapped lines count) shows collapsed to that height with a fade at its
   bottom and a control "Show all" below it, which expands the block in place;
   "Show less" collapses it again. The control is a button for the keyboard and the
   screen reader with its expanded state. The state lasts while the block is on screen.
3. **Indentation.** A wrapped line that starts with spaces or tabs keeps that indentation on every
   wrapped line (hanging indent), up to 16 columns, so indented code stays readable. Code blocks
   have no line numbers.
4. **Display only.** Wrapping, the horizontal scroll and the collapse change only the display: the
   copy of a block is its raw content as 6.12 item 3 defines, also while it is collapsed, and a
   system selection over a wrapped block keeps its line breaks and indentation (6.12 item 6).

## 7. Keyboard shortcuts

Shortcuts apply on desktop. They avoid system assignments (Cmd+H, Cmd+M, Cmd+Q, Alt+F4,
Windows key). There is no shortcut that triggers an approval.

| Action | macOS | Windows |
|---|---|---|
| Quick switcher | Cmd+K | Ctrl+K |
| Search | Cmd+Shift+F | Ctrl+Shift+F |
| Search in thread | Cmd+F | Ctrl+F |
| New post or new chat in the current forum (the forum of the focused or selected row, 6.8) | Cmd+N | Ctrl+N |
| Focus the input | Esc then Tab, or Cmd+L | Esc then Tab, or Ctrl+L |
| Next or previous channel or thread row in the sidebar | Option+Arrow down / up | Alt+Arrow down / up |
| Next or previous unread channel or thread row | Option+Shift+Arrow down / up | Alt+Shift+Arrow down / up |
| Pin or unpin the forum overview as a column | Cmd+Shift+O | Ctrl+Shift+O |
| Switch column (sidebar, pinned overview, history, session) | F6 / Shift+F6 | F6 / Shift+F6 |
| Mark thread as read | Esc | Esc |
| Mark as unread (from a message) | Option+Click | Alt+Click |
| Mark thread as done or reopen it | Cmd+Shift+E | Ctrl+Shift+E |
| Stop the run in the current thread | Cmd+. | Ctrl+. |
| Expand and collapse the live box | Cmd+Shift+L | Ctrl+Shift+L |
| Show and hide the session column | Cmd+Shift+I | Ctrl+Shift+I |
| Show and hide the channel next to an open thread | Cmd+Shift+H | Ctrl+Shift+H |
| Edit the last own message | Arrow up in the empty field | Arrow up in the empty field |
| Send / new line | Enter / Shift+Enter (reversible) | Enter / Shift+Enter (reversible) |
| Running now | Cmd+Shift+R | Ctrl+Shift+R |
| Mentions | Cmd+Shift+M | Ctrl+Shift+M |
| Settings | Cmd+, | Ctrl+, |
| List of shortcuts | Cmd+/ | Ctrl+/ |
| Open hosts and terminal | Cmd+Shift+T | Ctrl+Shift+T |

**In the terminal** all keys go to the shell, including Ctrl combinations such as Ctrl+C, Ctrl+D
and Ctrl+K. App shortcuts work there only with the platform modifier: on macOS with Cmd, on
Windows with Ctrl+Shift. Reason: shells and coding agents use Ctrl and Alt themselves; an app
shortcut must not swallow any input (R-PR-10).

| Action in the terminal | macOS | Windows |
|---|---|---|
| Copy / paste | Cmd+C / Cmd+V | Ctrl+Shift+C / Ctrl+Shift+V |
| Quick switcher | Cmd+K | Ctrl+Shift+K |
| New session on this host | Cmd+T | Ctrl+Shift+N |
| Next or previous session | Cmd+Shift+] / [ | Ctrl+Shift+Arrow right / left |
| Detach session (keeps running) | Cmd+Shift+D | Ctrl+Shift+D |
| Search in the scrollback | Cmd+F | Ctrl+Shift+F |
| Font larger / smaller | Cmd+Plus / Cmd+Minus | Ctrl+Shift+Plus / Ctrl+Shift+Minus |

## 8. Accessibility

Goal: WCAG 2.2 level AA (https://www.w3.org/TR/WCAG22/, retrieved 2026-10-10 [verified]) in apps
and console, checked by tests, not by intention.

| Topic | Rule | Check |
|---|---|---|
| Contrast | Every text and icon pairing reaches AA (text 4.5:1, UI 3:1) in light and dark | Gate `tokens.py check` ([contrast.md](../design/contrast.md)); widget tests with `textContrastGuideline` |
| Screen reader | VoiceOver, TalkBack, Narrator; in the console NVDA and VoiceOver. Every component has a role and a label; live box as a container with a summary; trigger state as its own label | Widget tests with `labeledTapTargetGuideline`; semantics snapshots per component |
| Live content | Only state changes and questions are announced, at most once per `a11y.live_announce_min_interval`; patches never | Test with a simulated patch stream |
| Font size | System scaling up to 200 percent without loss (SC 1.4.4); the layout wraps into the stack | Golden tests with text scaling 1 and 2 |
| Focus | Order per column (sidebar, pinned overview, history, input, session); F6 switches columns; focus ring always visible; in the sidebar tree, arrow keys reach thread rows (6.8) | Integration test of a keyboard pass |
| Targets | Touch target at least `density.target-min` (desktop SC 2.5.8, touch platform value) | `androidTapTargetGuideline`, `iOSTapTargetGuideline` |
| Motion | The system setting "Reduce motion" and the own setting switch to `motion.reduced` | Golden test in the context `reduced` |
| Color | States never only through color; icons differ in shape | Review list in components.md |
| Language | Language of the app set in the semantics tree; agent text without a language tag is not switched | |
| Terminal | The line at the cursor and, on command, the visible screen are readable for screen readers; running output is not read aloud; extra keys bar with labels; the font size follows the system setting and can be set separately in the terminal. How much semantics the renderer `xterm` provides is not verified [assumed: little]; the build ticket checks it and adds its own semantics layer | Widget test of the semantics; checklist with VoiceOver, TalkBack, Narrator |
| Visualizations | Every view has a table view with the same values; states with icon and text ([25-visualization.md](25-visualization.md), section 7.4) | Golden and widget tests of `bothy_viz` |
| Run graph and control room | "As list" shows the run graph as an outline with tree semantics (level, position, state as text, duration, cost) and the control room as the rows of D10; the end of a node and state changes are announced politely under the rule "Live content"; reduced motion: nodes appear without animation (addendum of 2026-10-24) | Semantics snapshots and widget tests of `bothy_viz`; golden test in the context `reduced` |

Known risk: according to an internal preliminary study, the accessibility of Flutter on macOS was
being reworked [assumed, not checked ourselves]; it is checked with VoiceOver before the first
macOS release (ticket in 15-client-architecture.md).

## 9. Languages

- German (default) and English. The language follows `users.locale`; the app starts in the
  system language as long as the person has not chosen one.
- Texts in ARB files in `packages/l10n` (`app_de.arb`, `app_en.arb`), generated with
  `flutter gen-l10n`; plurals and select through ICU messages. A missing translation breaks the
  build.
- A test over the ARB files rejects en and em dashes (U+2013, U+2014) and emoji.
- Errors come from the server as a code (RFC 9457, [06-api.md](06-api.md)); the app translates the
  code. Unknown codes show a generic text plus the code.
- Agent content is not translated.
- Numbers, dates and currencies follow the locale of the app (for example "1,84 USD" (de) or
  "USD 1.84" (en)).
- Terms are fixed so that DE and EN mean the same:

<!-- language-check: off -->

| German | English | Code |
|---|---|---|
| Bereich | Space | `space` |
| Kanal | Channel | `channel` |
| Forum, Beitrag | Forum, Post | `forum`, `thread` |
| Thread | Thread | `thread` |
| Erledigt | Done | `closed` |
| Live-Box | Live box | `run` |
| Lauf | Run | `run` |
| Nachtrag | Follow-up | `followup` |
| Bediener | Operator | `operator` |
| Freigabe | Access grant | `grant` |
| Agenten auslösen (Stufe) | Trigger agents | `trigger` |
| Aktionen freigeben | Approve actions | `approve_actions` |
| Persönlich, Organisation | Personal, Organization | `personal`, `organization` |
| Schätzung | Estimate | `cost_is_estimate` |
| Rechner | Host | `host` |
| Host-Dienst | Host service | `bothy-host` |
| Kopplung, Kopplungs-Code | Pairing, pairing code | `host.pair` |
| Fingerabdruck | Fingerprint | `key_fingerprint` |
| Terminal-Sitzung | Terminal session | `terminal_session` |
| Trennen, Wiederanhängen | Detach, reattach | `detached`, `terminal.attach` |
| Sondertasten-Leiste | Extra keys bar | |
| LLM-Anmeldung | LLM login | `llm_credentials` |
| Live-Topologie | Live topology | |
| TV-Modus | TV mode | |
| Eigene Messung | Own measurement | `stats_minutes` |

<!-- language-check: on -->

Implementation in `packages/l10n` (M4-06, docs/DECISIONS.md D-M4-06-1 to D-M4-06-7):

- `app_en.arb` is the template of `flutter gen-l10n` and German is the default locale
  (`supportedLocales` lists `de` first). Every message has an `@key` with a `description`.
- Problem codes: each code of `x-problem-codes` in `docs/api/openapi.yaml` has the message
  `problem<Code>` (for example `problemNotFound`); a reason that 07 names selects
  `problem<Code><Reason>` (for example `problemForbiddenCapabilityMissing`). Any other code
  shows `problemFallback` with the code: "Something went wrong ({code})" or "Etwas ist
  schiefgegangen ({code})".
- The table above is the source of `packages/l10n/lib/src/term_table.dart`, generated by
  `packages/l10n/tool/term_table.dart`; a test fails when the generated file is stale and when an
  English message uses a term of the table while the German message of the same key lacks the
  German term (direction English to German).
- The language gate of the repository excludes the German product strings by path:
  `packages/l10n/lib/l10n/app_de.arb` and the generated
  `packages/l10n/lib/src/generated/app_localizations_de.dart` in `tools/docs/language-allow.txt`.

## 10. Notifications

### 10.1 When a notification arrives

Normative for kinds, recipients, effective level and quiet hours is
[13-push-notifications.md](13-push-notifications.md), sections 2 to 4. For the
interface, in short:

| Event | Level `all` | Level `mentions` | Level `mute` |
|---|---|---|---|
| New message in a visible thread (not your own) | yes | no | no |
| Mention of me, message in my direct message | yes | yes | no |
| A run that I triggered is waiting for me (`waiting_input`) | yes | yes | yes |
| A run that I triggered is finished or has failed | yes | yes | no |
| Messages from agents without a mention | yes | no | no |
| Imported or silent messages (`silent`) | no | no | no |

During quiet hours notifications arrive silently (no sound, no waking up); counters and badges
keep counting. Only a question from one's own run stays loud, and only if the person has turned on
`notify.input_needed_breaks_quiet_hours`. Every notification passes through `push.deliver`
beforehand.

### 10.2 Content

The push itself carries only IDs and a neutral placeholder ("New message"). The app fetches the
text itself ([15](15-client-architecture.md), background behavior) and then shows
"<Channel> · <Thread title>" as the title and "<Author>: <Text, shortened>" as the text. If fetching
fails (no network to the server, time expired), the placeholder stays. What the preview may show
is controlled by `notify.preview_mode` (`full`, `names`, `none`); with `none` the app never fetches
text. A local notification of the running app then shows the placeholder with the channel name
the app knows locally; a push on iOS and macOS shows the placeholder without a channel name,
because its Notification Service Extension cannot read the encrypted database and a list of
names in its shared container would be content outside it (docs/decisions.d/M5-04.md).

### 10.3 Platform

| Platform | Behavior |
|---|---|
| iOS, macOS | Grouping per thread; tapping opens the thread at the message; badge = unread mentions plus questions |
| Android | Notification channels "Mentions", "Questions", "All messages", "Runs", changeable in the system settings; grouping per thread |
| Windows | The app in the notification area keeps the connection and shows local notifications; a closed app gets no notifications (Baseline R-Push) |
| all | If the thread is currently open and the app is in the foreground, there is no notification, only the history |

Reading on one device removes notifications on the others as soon as that app is active the next
time; a separate push only for cleanup is to be checked later.

## 11. Personal settings

Synchronized via `user_settings` ([04](04-sync-protocol.md) section 14). The view state of a
device is no setting and has no key here: which spaces, thread groups and Done sections are
collapsed, which columns show and the route stay on the device (6.7, R-PR-24). This table is
normative for all personal keys except `notify.*` ([03-data-model.md](03-data-model.md),
`user_settings`); the keys `notify.*` are in
[13-push-notifications.md](13-push-notifications.md), section 3.3. The server rejects
unknown keys and values outside the list.

| Key | Values | Default | Meaning |
|---|---|---|---|
| `appearance.theme` | `system`, `light`, `dark` | `system` | Theme; `system` follows the device |
| `appearance.density` | `comfortable`, `compact` | `comfortable` | desktop only; mobile fixed to `touch` |
| `appearance.reduce_motion` | `system`, `on`, `off` | `system` | Reduce motion |
| `composer.enter_sends` | `true`, `false` | without a stored value `true` on desktop and `false` on mobile; a stored value applies on all devices | Enter sends |
| `privacy.crash_reports` | `true`, `false` | `true` if the instance has set up a crash service | Send crash reports |
| `privacy.client_metrics` | `true`, `false` | `true` | Send performance data without content |
| `layout.column_widths` | Object: names `^[a-z][a-z0-9_]*$` (column, for example `sidebar`, `post_list`, `main_history`, `session`), values non-negative integers in logical pixels | empty | Column widths on desktop; the client interprets the names |

## 12. Branding per instance

Two layers (ADR 0044):

| Layer | What | Where defined | Effect |
|---|---|---|---|
| Runtime | Product name, logo, accent | `instance_settings`: `branding.product_name`, `branding.logo_file_id`, `branding.accent_color` (action `instance.branding.update`) | App and console show name and logo; the accent replaces `color.accent.*` |
| Build | App name in the system, bundle and package IDs, app icons, URL scheme for deep links and login, push topics (APNs topic = bundle ID, Firebase project), update feeds (Sparkle, `.appinstaller`), signing identities, default server address | File `branding.yaml` of the instance (lives in the instance operator's repo, not in the product), read by the build | Own signed app per instance |

Rules:

- `branding.accent_color` is a key from the category colors (`spruce`, `loch`, `heather`,
  `rowan`, `amber`, `gorse`, `moss`), not a free color value; only these combinations are in the
  contrast test (03, `instance_settings`; schema in the configuration reference).
  Every variant of these accents, in light and dark, is in the contrast table
  ([contrast.md](../design/contrast.md)); the design generator fails when one falls below AA.
- Without its own build configuration, the repo builds a neutral version with the working name
  and placeholder IDs under `example.com`.
- The logo is uploaded as a file and served only through the server; the app falls back to the
  product's mark as long as the logo is missing.
- `branding.product_name` has at most `branding.product_name_max_chars` characters (03) and no
  control or bidi characters ([19-security.md](19-security.md), SEC-IN-4).

## 13. Decisions of this chapter

| No. | Decision | Reason |
|---|---|---|
| R-PR-01 | Superseded by R-PR-17 (2026-10-27). Was: four columns on desktop; post list only for forums; the right column shows the session instead of members. | Study B; member lists are of no use in agent work, session data is. |
| R-PR-02 | Presence only for agents, never for people. | Privacy and co-determination; for agents, presence is operational information. |
| R-PR-03 | No read receipts for others; the read state stays personal. | Avoid performance monitoring; not in the baseline scope. |
| R-PR-04 | No keyboard shortcut triggers an approval. | Operating errors on approvals are expensive. |
| R-PR-05 | Questions from one's own runs notify even at level "mute" (13, kind `input_needed`). | A waiting agent blocks the work of the person who started it. |
| R-PR-06 | Quiet hours make notifications silent; only a question from one's own run becomes loud, if the person turns this on themselves (13). | Reliability before urgency; the only exception is in the hands of the person. |
| R-PR-07 | On mobile there are emergency actions of the administration, not the whole administration. | Small screens for complex permissions are error-prone; but disabling and revoking must work on the go. |
| R-PR-08 | Accent per instance only from the checked palette. | Contrast AA is a test, not a hope. |
| R-PR-09 | Link preview in the client only later and only with opt-in. | The server loads no foreign URLs; the client would contact foreign servers. |
| R-PR-10 | In the terminal all keys go to the shell; app shortcuts only with Cmd (macOS) or Ctrl+Shift (Windows). | Shells and coding agents use Ctrl and Alt; a swallowed Ctrl+C would be an operating error with consequences. |
| R-PR-11 | On mobile, an extra keys bar with one-shot modifiers instead of a custom keyboard. | The system keyboard stays (dictation, languages, accessibility); only the keys that a shell needs are missing. |
| R-PR-12 | The app detaches the terminal when it moves to the background and hides it in the app switcher. | Mobile systems end connections in the background anyway; the image in the app switcher would show terminal content. |
| R-PR-13 | Product description "Command center for AI agents in chat form" with six pillars (chat, live view, grants, terminal, costs, statistics). | Requirement of the owner (addendum of 2026-10-10); it determines the home page, docs and showcase. |
| R-PR-14 | Onboarding screens E1 to E7 shared by all apps; before login only the branding of the instance; with an invalid certificate no continue and no exception button. | One flow for all platforms keeps the security rules from 08 in one place; an exception button would let an attacker in the middle defeat DPoP and device binding. |
| R-PR-15 | The run graph is an indented trace view on a time axis, not a node-link diagram, on desktop, iPhone and Android; not in the web console (addendum of 2026-10-24). | Tree and time stay readable together, the layout does not jump when nodes arrive, it scales with a virtualized list and works on a phone; the console has no run views (ADR 0028). |
| R-PR-16 | The control room is "Running now" with cards and stays in its place; cards visible on the screen are live through `focus`, the others show the summary (addendum of 2026-10-24). | One place and one permission for active runs; live data only where a person looks, within the existing focus limit. |
| R-PR-17 | Desktop has three columns by default (sidebar, history, session column instead of members); open posts and threads are nested under their channel in the sidebar (6.8); the post list is the forum overview D15 in the content column and can be pinned as its own column (addendum of 2026-10-27, replaces R-PR-01). | Owner feedback 2026-10-27: the sessions of a project belong in the sidebar as in Discord; a fixed list column costs a quarter of a laptop screen. |
| R-PR-18 | A new chat starts from the forum row ("+") or Cmd+N without a dialog; the title is optional where an operator agent names the chat; no git worktree per post (addendum of 2026-10-27). | Starting a session is the most frequent action in a dev forum; the owner opens worktrees through the agent when he wants them. |
| R-PR-19 | On desktop the channel column next to an open thread (the main history of a text channel or the pinned forum overview) can be hidden from the thread header and with Cmd+Shift+H, kept per device; every column border, also the one between main history and thread, can be dragged (addendum of 2026-10-28). | Owner feedback on the Mac app (2026-10-28): the thread where the person writes stayed small next to the channel history, which could neither be hidden nor resized. |
| R-PR-20 | A new project starts from the space row or the space menu ("New project", owners only) in a dialog with four modes: empty, clone, existing folder, idea; it shows every step and resumes after a failure; for an idea the first post starts when the folder is ready (6.9, addendum of 2026-10-28). | Owner feedback 2026-10-28: a space had no way to add a project; his earlier setup created one per request (D-M4-43-1 to D-M4-43-12). |
| R-PR-21 | The space menu adds forums for the folders of a runner's projects folder that have none, as a checked list the owner confirms, one forum and binding per folder; a project forum's menu renames the forum, never the folder (6.10, 6.11, addendum of 2026-10-29). | Owner feedback 2026-10-29: his former setup synced a forum for every project folder and renamed project forums; the list is the preview, and a folder with a forum never gets a second one (D-M4-44-1 to D-M4-44-7). |
| R-PR-22 | On desktop the session column can be hidden and shown from the header of a thread or main history ("Hide session", "Show session"), with Cmd+Shift+I and from a slim rail at the right edge while it is hidden, kept per device and per layout (addendum of 2026-10-29). | Owner feedback on the Mac app (2026-10-29): the session column read as not collapsible, because the header button only said "Column" and the choice was lost on restart. |
| R-PR-23 | A forum group in the sidebar lists every open post by last activity, newest first, the newest `ui.sidebar_forum_posts_visible` and then "Show more" in place (kept per device); text channels keep the rule of 6.8 item 2 (addendum of 2026-10-29). | Owner decision 2026-10-29 ("like Discord"): after an import most open posts had no recent activity and vanished from the sidebar; a forum with 24 open posts showed none. |
| R-PR-24 | Each device navigates on its own: route, selected row, open columns, collapsed spaces and groups and scroll position stay on the device and never follow another device of the person; only data (messages, read state, counters, drafts, settings, sidebar order, notification preferences) syncs (6.7, 04 14; addendum of 2026-10-29). | Owner report 2026-10-29 (iPhone and Mac app of one person): opening a chat on the phone appeared to open the same chat on the Mac. The cause was the synchronized collapse of a space: the space a device expanded to open a chat expanded on the other device at the same moment, while the route never followed. The owner wants to do different things on phone and desktop. Two-device tests over the fake server hold the rule (docs/decisions.d/FIX-device-mirroring.md, docs/decisions.d/FIX-device-mirroring-2.md). |
| R-PR-25 | Every code block and quote block of a message copies on its own: on the desktop through a copy button at its top right, on the phone through a tap with "Copied" and a haptic, while a long press keeps the message menu; a code block copies its raw content byte for byte, a quote its text without markers and, where the clipboard holds rich text, also as HTML (6.12, addendum of 2026-10-29). | Owner request 2026-10-29: in Discord a tap on a code block on the phone never copied it and "Copy text" always copied the whole message. |
| R-PR-26 | A paste over `ui.composer_paste_file_chars` characters or `ui.composer_paste_file_lines` lines becomes a text file attachment with "Insert as text"; a text over `message.max_content_bytes` goes out as one message with its rest as one attachment, from people and agents alike; a message is never split (6.13, addendum of 2026-10-29). | Owner decision 2026-10-29: a person never hits a length limit, as in the former chat tool where 2000 characters forced splitting or a paid plan; one message keeps a reply readable and quotable as one. |
| R-PR-27 | Every forum the Claude Code runner operates is a Claude Code chat: its folder need not be a Git repository, a forum without a project binding works in the runner's default folder when one is set, and a failed run shows its cause in the thread next to the reason (section 4, addendum of 2026-10-30). | Owner expectation 2026-10-30: three forums bound to folders without Git stayed "not ready", and a forum without a binding failed after 0 s with only "Error in the adapter" (docs/decisions.d/RUNNER-NO-GIT.md). |
| R-PR-28 | A code block wraps its long lines by default, keeps the indentation of a wrapped line, and switches per device to one line per line with a working horizontal scroll through the toggle "Wrap lines"; a block higher than `ui.code_block_collapse_lines` lines shows collapsed with a fade and "Show all" / "Show less"; the copy stays the raw content (6.14, addendum of 2026-10-31). | Owner decision 2026-10-31: a long sentence sent as a fenced block "to copy" showed as one cut-off line that could neither be scrolled nor expanded on macOS, so the owner could not read what he would copy (docs/decisions.d/M4-49.md). |

## 14. Configuration values of this chapter

| Name | Default | Meaning |
|---|---|---|
| `ui.message_group_window` | `5m` | messages of the same author within this time get no header of their own |
| `ui.message_collapse_lines` | `24` | from this many lines on, a message is collapsed |
| `ui.livebox_visible_steps` | `6` | visible steps in the running live box |
| `ui.composer_max_lines` | `12` | the input grows up to this many lines |
| `ui.composer_paste_file_chars` | `4000` | a paste of more characters becomes a text file attachment in the composer (6.13) |
| `ui.composer_paste_file_lines` | `100` | a paste of more lines becomes a text file attachment in the composer (6.13) |
| `ui.code_block_collapse_lines` | `20` | a code block higher than this many lines of code shows collapsed with a fade and "Show all" (6.14) |
| `ui.autocomplete_debounce` | `150ms` | waiting time before `command.autocomplete` |
| `ui.quick_switch_max_results` | `20` | hits in the quick switcher |
| `ui.desktop_min_width_four_columns` | `1280px` | below this width the session column collapses |
| `ui.desktop_min_width_three_columns` | `960px` | below this width a pinned forum overview returns to the content column (navigation stack) |
| `ui.sidebar_thread_window` | `72h` | an open thread of a text channel with activity within this window shows in the sidebar (6.8) |
| `ui.sidebar_threads_per_channel` | `8` | at most this many thread rows per text channel in the sidebar (6.8) |
| `ui.sidebar_forum_posts_visible` | `15` | a forum group in the sidebar shows this many open posts before "Show more" (6.8) |
| `ui.new_project_wait` | `2m` | after this time without readiness the new project dialog says that the runner has not finished (6.9) |
| `ui.new_project_poll_interval` | `2s` | interval at which the new project dialog reads the readiness of the binding (6.9) |
| `ui.new_project_name_max_chars` | `40` | longest suggested and accepted name of a new project (6.9) |
| `a11y.live_announce_min_interval` | `10s` | the live box announces a change at most this often |
| `client.share.recent_targets` | `8` | recently used targets in the share sheet |
