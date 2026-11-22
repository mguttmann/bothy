# FEAT-PIN-CHANNELS Pinning channels and posts in the own sidebar

Owner request of 2026-11-22: a team channel sat far down in its space; it should stay at the top
while "Arrange channels and projects automatically" stays on, with a right click and "Pin", in
every space. Normative text: 01 6.24, R-PR-39; 03 `user_channel_states.pinned_at` and
`pinned_threads`; 04 14 (sidebar channel); 06 R-API-31.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-PIN-1 | 2026-11-22 | A pin is personal, per person, and follows the person to every device through the server; it is not a state of the space or the channel. | A person who uses several devices (desktop and phone) expects the same sidebar on each; the sidebar order (6.16) and the archive (6.22) are personal and synchronized for the same reason, while the pin of a message (6.7) is moderation for everyone and needs `manage`. A device-local pin would show the team channel at the top on one device only. |
| D-PIN-2 | 2026-11-22 | The channel pin is `user_channel_states.pinned_at`, the pinned posts of a forum are `user_channel_states.pinned_threads` (in pin order), both written with the existing `putMySidebarChannel` (`pinned`, `pinned_threads`, right `sidebar.update`, people only, only the caller's own row) and synchronized with the existing `sidebar_channel.updated`, the snapshot member `sidebar_channels` and `getMySidebar`. | The row, the write, the right, the event and the snapshot member already carry the personal state of a channel (M4-51, PROJ-ARCHIVE); no new endpoint, right or event is needed, and the rights stay "only own pins" by construction (the row key is the caller). |
| D-PIN-3 | 2026-11-22 | Channels and forums are pinned by a time (`pinned_at`, oldest first, equal times by channel ID); posts by a list (`pinned_threads`) that the client replaces as a whole. | A time per channel row merges pins of two devices without losing one; posts have no personal row of their own, and a list in the forum's row keeps the pin order with the same last-write-wins rule as `thread_order`. |
| D-PIN-4 | 2026-11-22 | Pins apply on top of every order: the manual order (6.16 items 2 and 3) and the arrangement by activity (6.16 item 4) stay unchanged for the rest of the level, and a pin or unpin applies at once, also while the sidebar holds its rows. | Owner wording: the automatic arrangement stays on, only the one channel stays on top. The hold of 6.8 item 4 exists so that activity does not move rows under the pointer; a pin is the person's own action in that very menu. |
| D-PIN-5 | 2026-11-22 | In a manual level a pinned row does not lift and its "Move up" and "Move down" are disabled; the other rows move only among themselves below the pins. A move still writes the whole manual order. | A drag or move of a pinned row would change an order that its display ignores; the manual order stays the place an unpinned row returns to. |
| D-PIN-6 | 2026-11-22 | The entry stands in the sidebar menus of channels, forums and forum posts on all four platforms and in the More menu of the header (main history, chat history) and the post list menu. Direct messages, channels archived for everyone, main threads, done posts that are not pinned and rows in the section "Archived" have none. The server accepts `pinned` for every channel the caller can read, as for `hidden` (D-PROJ-ARCHIVE-5). | Direct messages and channels archived for everyone are no rows of a space; the sidebar shows no done post; an archived row's pin waits for its restore. |
| D-PIN-7 | 2026-11-22 | Archiving a channel for oneself keeps its pin; it applies again after "Restore". | Archive and pin are independent members of the same row; neither write touches the other (an absent member keeps the stored value). |
| D-PIN-8 | 2026-11-22 | The client database gets schema version 18 with both columns, a cleared cursor and every `user_channel_states` row at version 0 (`MigrationRules.reloadRows`). | A cleared cursor alone loads a snapshot, but its rows arrive at the version an older app already stored without the pins, and the version rule skips them (found in review; the earlier step for `hidden_at` had the same gap, which this step also closes for devices that upgrade now). |
| D-PIN-9 | 2026-11-22 | A move in a manual level and the hold of 6.8 item 4 use the order without the pins; pinned rows are left out of the rows a move counts. | A pinned row must keep its place in the manual or held order, so an unpin returns it there at once (found in review). |

## Follow-ups (minor, not fixed in this change)

- An older server rejects a body with only `pinned` or `pinned_threads` (`additionalProperties: false`)
  with 400; the local pin then stays on that device without a notice (same pattern as `hidden`).
- A post moved to another forum or deleted stays in `pinned_threads` and counts toward
  `sidebar.order_max_items`; the next toggle sends it again.
- The local pin time is the device clock until the server's time arrives; a write the server
  answers as "no change" keeps the local time, so the order of several pins can differ between
  devices for a while.
