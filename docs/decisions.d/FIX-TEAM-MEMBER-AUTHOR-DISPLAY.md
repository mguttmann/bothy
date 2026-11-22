# FIX-TEAM-MEMBER-AUTHOR-DISPLAY Team members write under their own name

Owner word of 2026-11-22: in a team channel every member wrote under the agent's name ("Claude
Code") with only a small line under the message naming the member, and several members' replies in
a row appeared as one block. Wanted: each member writes under its own name, and it is clear at a
glance who wrote what. Specification: [01-product.md](../spec/01-product.md) 6.23 item 3 and 14,
[components.md](../design/components.md) section 1, [13-push-notifications.md](../spec/13-push-notifications.md) 6.4.

| No | Date | Ruling | Reason |
|---|---|---|---|
| D-TMAD-1 | 2026-11-22 | A message with `team_handle` shows the handle as its author name; the agent's display name follows the badge in caption size ("via Claude Code", in German with the German word for "via"). The handle is the member's name: a member has no display name of its own (09 3.9.1). | The server already sends `team_handle` with every team copy (FIX-TEAM-MEMBER-ATTRIBUTION), so no server field was added for the history; the handle is what the owner typed for the member and what mentions use. |
| D-TMAD-2 | 2026-11-22 | Grouping (`isContinuation`) also needs the same `team_handle`, so replies of two members of one agent each get their own header, and a member's reply never continues a message of the agent without a handle. | Grouping by author alone joined the members, because one agent writes for several of them. |
| D-TMAD-3 | 2026-11-22 | The avatar of a team member picks its tile from eight category tiles (the six of every avatar plus gorse and granite, whose chip pairs pass contrast, contrast.md) with the same rune-sum rule; every other avatar keeps its six-tile color. | With six tiles three of the five handles of the owner's team shared one color; with eight all five differ. Changing every avatar would have changed the colors of every person and the golden images without need. Members can still share a tile; their initials and names then tell them apart. |
| D-TMAD-4 | 2026-11-22 | The "from @handle" line is shown only under a mirror stored without `team_handle`; with the handle the header names the member. This replaces the "from @handle" of D-TMA-2 for such a mirror. | The line repeated the header in small type, the complaint of the owner. |
| D-TMAD-5 | 2026-11-22 | The server names the member (`team_handle`) as `sender_display_name` in the preview of a notification (push preview and `notification.created`); `messaging.PreviewMessage` carries `TeamHandle`. | Push texts and the in-app notification are built from the server's preview, not by the client, so the member's name has to come from there. |

Not covered (open minors for the next run in this area): the search results still name the author
from the server's search hit (the agent); the mentions view (`describeMention`,
apps/client/lib/mentions/providers.dart) names the agent of a message with `team_handle`; the
screen reader label of a member's message reads the handle but not the "via" note;
the input copies in a member's work thread show the member as author too, since they carry
`team_handle`.
