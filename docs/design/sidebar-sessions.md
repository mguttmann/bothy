# Sessions in the sidebar

Status: design of 2026-10-27 after the owner's first use of the desktop app; a short design with
the decisions, then build tickets. This note explains and justifies; the normative text is in the
chapters it changes ([01-product.md](../spec/01-product.md) 3.1, 4, D1, D2, D15, M1, 6.8, 7, 8,
13, 14; [03-data-model.md](../spec/03-data-model.md) `project_bindings`;
[04-sync-protocol.md](../spec/04-sync-protocol.md) 6.3 item 4, 17, 18;
[05-actions.md](../spec/05-actions.md) event `channel.updated`;
[09-agents.md](../spec/09-agents.md) 18.1; [components.md](components.md) 6, 7). Rulings:
[decisions.d/sidebar-sessions.md](../decisions.d/sidebar-sessions.md) (`D-SB-1` to `D-SB-13`).
Tickets: M2-52 (server), M4-40 and M4-41 (desktop), M5-15 (iPhone and Android) in the
[plan](../PLAN.md).

## 1. Goal

Owner feedback (2026-10-27, translated): "Left I have the sidebar with the areas and the projects,
but I have no sessions. Each project should be able to have several chats at the same time. In
Discord I see them in the sidebar. Here the posts open in an extra column that takes far too much
space. What is the difference between the ones with # and the ones with the chat bubble?"

His Discord setup: one forum channel per project in the sidebar, its active posts nested directly
below it, indented with a tree line, clickable there.

Clarification of the same day: several agents in one project are fine as they are; he can tell an
agent himself to open an extra git worktree. What matters is that starting another chat in a
project is quick and visible in the sidebar.

Done means:

1. Open posts of a forum (in a dev forum each post is one agent session) and open threads of a
   text channel appear nested under their channel in the sidebar, with state and unread markers,
   and open with one click.
2. The separate post list column is closed by default; the forum overview with search, tags and
   the Done section is still one click or one shortcut away.
3. A new chat in a forum starts from the sidebar with one click or Cmd+N, without a dialog, and
   appears in the sidebar at once.
4. The interface explains the difference between a text channel (`#`) and a forum, and shows which
   forums are projects.
5. No automatic git worktree per post; the runner is unchanged.

## 2. What exists

| Part | Today | Gap for the goal |
|---|---|---|
| Sidebar (01 D1, components.md 7, M4-19) | spaces and channels with counters, run dot, collapse and personal order of spaces | no thread rows; the sessions of a project are not visible without opening the forum |
| Post list (01 D1, components.md 6, M4-19) | its own column between sidebar and history, always shown for a forum | takes a fixed column for something the sidebar can show; on a laptop the history gets narrow |
| New post (M4-19) | button in the post list header, dialog with title, first message and tags | two steps and a modal dialog for the most frequent action in a dev forum; not reachable from the sidebar |
| Snapshot (04 6.3 item 4) | spaces, channels, sidebar state, unread threads, active runs | no thread titles: the client knows the posts of a forum only after loading its list (`listChannelThreads` per channel) |
| Channel (API `Channel`) | `kind` `text`, `forum` or `dm` | no sign that a forum is bound to a project, so the app cannot show "project" or say "New chat" |
| Auto-title (09 12) | an empty or placeholder title stays `auto`; the reference runners set the title after the first run | already enough for a chat without a title |
| Claude Code runner (09 18.1, 18.2) | one Claude Code session per thread, working folder of every turn is the project folder, locks per thread and session, `runner.max_parallel_runs` | none: several sessions in one project already run in parallel |

## 3. Decisions

Each decision has a ruling ID in the fragment; the chapters carry the normative text.

**D-SB-1 Threads nested under their channel; the post list becomes optional.** The sidebar shows,
under each forum and each text channel, a group of thread rows (forum posts, threads from messages
of the main thread), indented with a tree line, as in Discord. The separate post list column is
closed by default. A click on a forum row opens the **forum overview** (D15) in the content column:
the post list with "New post", the filters, tags, the Done section and the search in the forum. The
overview can be pinned as its own column between sidebar and history (Cmd+Shift+O, button in the
forum header); then D1 looks as before. R-PR-17 replaces R-PR-01. Reason: the owner works from the
sidebar as in Discord; a fixed column for the list costs about a quarter of a laptop screen while
the sidebar has room for the few active sessions; the overview keeps every function of the list
(search, tags, Done) without a column of its own.

**D-SB-2 Which threads show.** A thread row shows when the thread is open and at least one of these
holds: it is selected; it has an active run; it waits for the person ("Waiting for me" as in the
post list, D-M4-19); it is unread; its last activity lies within `ui.sidebar_thread_window`. At most
`ui.sidebar_threads_per_channel` rows per channel; when more qualify, the selected thread, then
"Waiting for me", then an active run, then unread mentions, then unread, then the newest activity
win. Done threads never show, except the selected one. Reason: the sidebar shows what is going on
now; a fixed bound keeps a busy forum from pushing the other channels out of sight; the priority
keeps every thread that needs the person visible.

**D-SB-3 Order by creation, newest first, and no jumping rows.** The rows of a group are ordered by
creation time, newest first, not by activity. A row that no longer qualifies leaves the sidebar
only when the pointer is outside the sidebar and the keyboard focus is not in it. Reason: agents
write constantly; ordering by activity would move rows under the pointer while the person aims at
one. A new chat appears at the top of its forum, where the person expects it.

**D-SB-4 Entry "All posts".** An expanded forum group ends with the row "All posts", which opens the
forum overview; it carries the mention counter and the unread mark of the posts that are not shown
in the group. It names no total: the client knows the full number only after loading every page of
the list. Text channels have no such row; their other threads stay reachable through the main
thread, the search and the quick switcher. Reason: the owner asked for a "show all" entry; a wrong
count would be worse than none.

**D-SB-5 Markers.** A thread row has the states of a channel row (components.md 7): unread bold with
the bar, mention counter, selected, hover, focus, muted; in front of the title the run state icon
(`run.running`, `run.waiting`, `run.failed`) instead of the run dot. The channel row of an expanded
group counts unread and mentions only of the threads that are not shown below it (and of the main
thread of a text channel); a collapsed group counts all; the run dot of the channel stays an
aggregate of all its threads. Reason: a number must not appear twice on screen, and nothing unread
may vanish from view.

**D-SB-6 Local view state.** Whether a channel's group is collapsed (default: expanded) and whether
the overview column is pinned (default: not pinned) are kept per device in the local database and
are not synchronized. Reason: 01 11 is normative and has no key for them, like the Done section
(D-M4-19-4); a new personal setting key would need a server change for a view preference.

**D-SB-7 Dragging.** Thread rows and channel rows cannot be dragged; the personal order of spaces
stays as it is. Moving a thread stays the dialog with only valid targets (6.7). Reason: thread order
is derived (D-SB-3) and channel order is global; a drop onto another channel would be a move, which
needs the target check and a reason, and an accidental drag must not move a session.

**D-SB-8 Keyboard.** The sidebar is one tree: arrow up and down move through the visible rows
including thread rows, arrow right expands a group or enters its first row, arrow left goes to the
parent or collapses, Enter opens. "Next or previous channel" (Option or Alt with arrows) also stops
at thread rows; "next unread" too. Cmd+N (Ctrl+N) starts a new chat in the current forum, which is
the forum of the focused or selected row. Cmd+Shift+O (Ctrl+Shift+O) pins or unpins the overview
column. Reason: the sessions are the main navigation targets now; the shortcuts that existed keep
their meaning, extended to the rows that are now in the tree.

**D-SB-9 New chat without a dialog.** Every forum row has a "+" action (visible on hover, on focus
and on the selected forum, always on touch) labelled "New chat in <forum>" for a project forum and
"New post in <forum>" otherwise; it also is in the context menu. It selects a draft row "New chat"
at the top of the forum's group and opens the new chat view in the content column: the composer has
the focus, the title field is optional in a forum with an operator agent (the agent names the chat,
09 12) and required in a forum without one, tags are optional. Sending creates the post through the
outbox with the first message (`createThread`); the draft row becomes the post row in place. The
draft is the existing draft of a new forum post (03 `drafts`, `thread_id` NULL, one per person and
forum), so it survives a restart and appears on the person's other devices; a second "+" on the same
forum opens that draft instead of a second one. Reason: starting a chat is the most frequent action
in a dev forum; a dialog and a title are friction the auto-title already removes.

**D-SB-10 Explaining text channel and forum.** Three icons: `channel.text` (`#`, a channel is one
shared conversation, threads come from its messages), `channel.forum` (several bubbles, every post
is its own conversation), and new `channel.project` for a forum with a project binding (every post
is one agent session in the project). The icon has a tooltip and a screen reader text with that
sentence; the channel header shows it as a subtitle; an empty forum shows it as its empty state.
For this the API `Channel` carries `project_bound` (D-SB-11). Reason: the owner asked the question
in the first minutes; the showcase standard (01 1 item 8) requires that a person understands it at
first sight.

**D-SB-11 Server: threads in the snapshot and the project mark.** The snapshot (`ready` and
`getSnapshot`) gets `active_threads`: per visible channel every open thread with an active run, every
open thread unread for the person (so the threads the sidebar ranks first are always held), plus
the open threads with activity within `sync.snapshot_thread_window`, newest activity first, until
the channel has `sync.snapshot_threads_per_channel` entries. The client keeps them current from the
events it already receives (`thread.*`, `message.created`). `Channel.project_bound` is true exactly
when a project binding exists; creating or deleting a binding raises `channels.version` and writes
`channel.updated`. No new action, permission rule, event type or table. Reason: without titles in
the snapshot the client would call `listChannelThreads` once per channel on every connect; a flag
derived from an existing row is cheaper than exposing the binding, which needs `manage` to read.

**D-SB-12 No worktree per post; the runner stays.** A project forum holds several sessions at the
same time, each in the project folder with its own Claude Code session (09 18.1); the runner creates
no git worktree per post and its working folder per turn stays the project folder. An agent with
tool class `host` can create a worktree itself when the person asks it to; the runner neither helps
nor prevents that. Reason: the owner's clarification of 2026-10-27; worktrees per post would change
the runner contract, readiness and cleanup for a need he does not have. Locks per thread and session
and `runner.max_parallel_runs` already make parallel sessions safe for the runner itself; what two
agents do in the same folder is the person's call, as in his Discord setup.

**D-SB-13 Phone.** The spaces screen M1 shows the same groups with the same rules and the "+" action
at forum rows (touch target `density.target-min`); a tap on a thread row opens the history directly
(M3), a tap on the forum name opens the post list M2 as before. There is no pinned column on the
phone. Reason: the same mental model on every device; on the phone the post list is a page anyway.

## 4. Considered and rejected

| Option | Why not |
|---|---|
| Keep the post list column and only shrink it | still a column for a list the sidebar can hold; does not give the Discord view the owner asked for |
| Order thread rows by last activity | rows move under the pointer while agents work (D-SB-3) |
| Show every open post in the sidebar | a busy forum would push the rest of the sidebar away; the overview has them all (replaced for forums on 2026-10-29, section 6) |
| Load each forum's list on connect instead of a snapshot member | one request per channel on every connect and resync; rate class `rl.read` and start time grow with the number of channels |
| Synchronize collapse and pin state | needs a new personal setting key and a server change for a view preference (D-M4-19-4) |
| A worktree per post in the runner | not wanted (owner, 2026-10-27); would change readiness, cleanup and the working folder contract |
| Terminal sessions nested under hosts the same way | not asked; D11 already lists hosts with a session count and opens sessions as tabs; can follow later with the same tree rules |

## 5. Tickets

Defined in the [plan](../PLAN.md); dependencies there and in the brief index.

- M2-52 (server): `active_threads` in the snapshot, `Channel.project_bound`, `channel.updated` on
  binding create and delete; after M2-03.
- M4-40 (desktop): thread groups in the sidebar, forum overview in the content column, pinnable
  overview column, icons and explanations, keyboard; after M2-52 and M4-24.
- M4-41 (desktop): "+ New chat" at the forum, new chat view without a dialog, draft row; after
  M4-40.
- M5-15 (iPhone and Android): the same groups and "+" in M1; after M4-41 and M5-03.

## 6. Addendum of 2026-10-29: every open post of a forum (M4-46)

Owner decision of 2026-10-29: the sidebar shows every open forum post, as Discord does. After an
import most open posts of a forum had no activity within `ui.sidebar_thread_window`, so a forum
with 24 open posts showed none. The normative text is 01 6.8 items 2 to 4 and 7 (R-PR-23); the
rulings are `D-M4-46-1` to `D-M4-46-5` in [decisions.d/M4-46.md](../decisions.d/M4-46.md). They
replace D-SB-2 and D-SB-3 for forums; text channels keep both.

- A forum group lists every open post the client holds, by last activity, newest first; done
  posts only while selected. Unread, mentions and the run state icon stay on the rows.
- Beyond `ui.sidebar_forum_posts_visible` (15) the row "Show more" above "All posts" lists the
  rest in place and then reads "Show less"; the choice is kept per device next to the collapse
  state (`channel_view_states`), not synchronized. This answers the concern of section 4: a busy
  forum takes the room of 15 rows until the person asks for more.
- While the pointer or the focus is in the sidebar the rows keep their order (D-SB-3 still
  holds: no row moves under the pointer); a post new to the group appears at the top. "Show
  more" and "Show less" apply at once.
- The client lists what it holds: the snapshot's `active_threads` (04 6.3 item 4: unread, running
  and up to `sync.snapshot_threads_per_channel` posts active within `sync.snapshot_thread_window`),
  the open posts the client lists for each forum after a full snapshot (first connection,
  resync; `listChannelThreads` with `state=open`), the posts that events brought, and the pages
  of the forum overview. The snapshot rule is not changed here (D-M4-46-5).
