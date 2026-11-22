-- SPDX-License-Identifier: Apache-2.0
-- messages.team_handle (FIX-TEAM-MEMBER-ATTRIBUTION, docs/spec/03-data-model.md section 4.6,
-- docs/spec/09-agents.md 3.9.2): the handle of the team member whose reply a team copy carries,
-- set by messaging in the INSERT of the mirror of a member's reply in the team channel and of a
-- member's input in another member's work thread; NULL for every other message. One agent can be
-- several members of a team channel, so the author alone does not name the member.
--
-- Expand only (upgrade.md rules M1 and M3): a nullable column without a default, which the
-- previous binary never reads, added without a table rewrite.
--
-- Backfill: the copies written before this migration get the handle of the source member that
-- team_mirrors.member_id and team_inputs.source_member_id name, while that member still exists.
-- This deviates from upgrade.md rule M7 (backfills as jobs) on purpose (docs/decisions.d/
-- FIX-TEAM-MEMBER-ATTRIBUTION.md D-TMA-3): the rows are only the team copies, reached through the
-- primary keys of the two team tables of 00106, which came with the same release line, so the
-- step stays one short statement each.

-- +goose Up
ALTER TABLE messages ADD COLUMN team_handle text;

UPDATE messages m SET team_handle = tm.handle
FROM team_mirrors r JOIN team_members tm ON tm.id = r.member_id
WHERE m.id = r.mirror_message_id AND m.team_handle IS NULL;

UPDATE messages m SET team_handle = tm.handle
FROM team_inputs i JOIN team_members tm ON tm.id = i.source_member_id
WHERE m.id = i.message_id AND i.source = 'member' AND m.team_handle IS NULL;

-- +goose Down
ALTER TABLE messages DROP COLUMN team_handle;
