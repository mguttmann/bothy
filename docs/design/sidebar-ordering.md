# Sidebar ordering

Status: design of 2026-10-31 after an owner request; a short design with the decisions, then one
build ticket. This note explains and justifies; the normative text is in the chapters it changes
([01-product.md](../spec/01-product.md) 6.7, 6.8, 6.15, 11, 13 R-PR-29;
[03-data-model.md](../spec/03-data-model.md) `user_space_states`, `user_channel_states`;
[04-sync-protocol.md](../spec/04-sync-protocol.md) 14; [06-api.md](../spec/06-api.md) Preferences).
Rulings: [decisions.d/M4-51.md](../decisions.d/M4-51.md) (`D-M4-51-1` to `D-M4-51-9`). Ticket:
M4-51 in the [plan](../PLAN.md), brief [M4-51](../briefs/M4-51-sidebar-ordering.md).

## 1. Goal

Owner request (2026-10-31, translated): nothing in the sidebar can be moved. Not the spaces (the
top level, for example five imported spaces), not the channels and forums inside a space (a dev
space with 25 project forums), not the chats inside a forum. He wants to drag them into his own
order, and he wants an optional automatic order by activity, switchable per level.

Done means:

1. The person can put the spaces, the channels and forums of a space, and the open posts of a
   forum into an order of their own, by dragging on macOS, Windows, iOS and Android and by the
   keyboard on the desktop.
2. Three switches in the settings arrange each level by the last activity instead.
3. The order is the person's own: it is kept across restarts and never changes the order of
   anyone else.

## 2. What exists

| Part | Today | Gap |
|---|---|---|
| Space order (M4-19, 04 14) | personal `user_space_states.sort_key`, synchronized; the desktop list drags a space by its row, screen readers have "Move up" and "Move down" | no keyboard entry in the space menu; on a touch screen the row starts a drag at once, which competes with scrolling; no test drags a space |
| Channel order | the global `channels.sort_key` of the space (owners) | no personal order (D-SB-7 forbade dragging) |
| Post order in a forum (M4-46) | by last activity, newest first, held while the person is in the sidebar | no manual order |
| Settings (01 11) | personal keys synchronized through `user_settings` | no key for an arrangement |

## 3. Decisions

**D-M4-51-1 Three levels, each manual or automatic.** The levels are the spaces, the channels and
forums of one space, and the open posts of one forum. Each level is either in its manual order or
arranged by activity, chosen by one switch per level. Text channel threads keep their rule (01 6.8
items 2 to 4): they are few, bounded and derived from messages of the main history. Reason: the
owner asked for exactly these three levels and three independent switches.

**D-M4-51-2 The order follows the person, not the device.** The manual order of every level is
personal data of 04 14, synchronized to every device of the person and never visible to anyone
else: spaces keep `user_space_states.sort_key`; the channel order of a space is
`user_space_states.channel_order`; the post order of a forum is
`user_channel_states.thread_order`. Reason: 01 6.7 and R-PR-24 name the sidebar order as data that
crosses devices, while the collapse state, the route and the columns stay per device; the space
order already syncs, and a person with 25 forums arranges them once, not on every device. The
collapse of a space stays per device as fixed in R-PR-24.

**D-M4-51-3 An order is a list; rows not in it come first.** The channel order of a space and the
post order of a forum are lists of IDs. The rows in the list show in its order; the rows not in it
(a forum created later, a new post) show first, in the default order of the level (channels: the
global order; posts: last activity, newest first). Moving a row writes the whole list of the rows
of the level that the client holds, in the new order, as one write; the list therefore holds only rows that exist on
the device and stays within `sidebar.order_max_items`. Reason: one write per move, the last write
wins as for every sidebar row (04 14), and a new forum or chat appears where the person sees it.
The spaces keep their fractional keys: their order already works that way and old clients read it.

**D-M4-51-4 Automatic order by activity.** A level that is automatic is ordered by its last
activity, newest first: a post by `last_activity_at` (a message, a run step); a channel by the
newest activity of its threads that the client holds (its main history included), else its creation
time; a space by the newest activity of its channels. Ties keep the manual order. While the pointer
or the keyboard focus is in the sidebar (on a touch screen: a finger, a scroll, the screen reader)
an automatic level keeps the order it showed last, as forum posts do since M4-46 (01 6.8 item 4), so
no row moves under the pointer. Writing a message therefore moves its post to the top of its forum
and the forum to the top of its space, once the person is back in the history.

**D-M4-51-5 Three personal settings.** `sidebar.auto_arrange_spaces` (default `false`),
`sidebar.auto_arrange_channels` (default `false`) and `sidebar.auto_arrange_chats` (default `true`)
are personal settings of 01 11, synchronized like the theme. The German interface calls them
"Bereiche automatisch anordnen", "Kanäle und Projekte automatisch anordnen" and "Chats automatisch
anordnen". Reason: 01 11 makes personal settings synchronized; the manual order they switch
between follows the person, so the switch does too. `chats` on is the order of M4-46, so nothing
changes for a person who never opens the settings.

**D-M4-51-6 Dragging.** On macOS and Windows a row is dragged with the mouse; a line between two
rows shows where it lands. On iOS and Android a long press lifts the row, then it is dragged; a
short touch still opens the row and scrolling still scrolls. A row moves only within its level and
its parent: a channel stays in its space, a post in its forum (moving a post to another forum stays
the dialog of 6.7). At an automatic level the row does not lift; a note says that the level is
arranged by activity and where to switch it off. D-SB-7 is replaced for channels and forum posts.

**D-M4-51-7 Keyboard and screen readers.** The menu of a space, a channel and a forum post in the
sidebar (right click, the menu key, Shift+F10) has "Move up" and "Move down", disabled at the ends;
at an automatic level both are disabled and the menu says why. Screen readers get the same two
actions on the row, as the spaces have since M5-10.

**D-M4-51-8 The server stores what the person writes.** `PUT /me/sidebar/spaces/{space_id}` takes
an optional `channel_order`; `PUT /me/sidebar/channels/{channel_id}` (new, action
`sidebar.update`) takes the `thread_order` of a channel. Both check the form (unique IDs, at most
`sidebar.order_max_items`), not whether each ID names a visible row, so the answer reveals nothing
about rows the person cannot see; the client ignores IDs it does not hold. The event
`sidebar_channel.updated` (audience self) and the snapshot member `sidebar_channels` carry the
channel rows like `sidebar.updated` and `sidebar` carry the space rows.

**D-M4-51-9 Hidden rows keep their place.** The "Show more" limit of a forum (01 6.8 item 3) applies
to the manual order as well: the first `ui.sidebar_forum_posts_visible` posts of the order show.
A move writes the order of every open post of the forum the client holds, including those behind
"Show more", so a move never drops a hidden post from the order.

## 4. Out of scope

- Sharing an order with other people, or an owner setting the order for everyone (the global
  `sort_key` of spaces and channels stays the owners' tool).
- Dragging a row into another parent (a channel to another space, a post to another forum).
- An order of text channel threads and of direct messages.
