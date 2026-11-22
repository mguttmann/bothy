-- SPDX-License-Identifier: Apache-2.0
-- team_channels, team_members, team_inputs and team_mirrors (docs/spec/03-data-model.md section
-- 4.8, FEAT-TEAM-CHANNEL-SERVER, docs/spec/09-agents.md 3.9). Every read names the team of one
-- channel whose team.read, team.update or team.stop authorize allowed in the same unit of work, the
-- member of one work thread or input that the job runs.team_input or a delivery check reads without
-- a principal (tools/lint/sql_visibility.yaml lists each read). Lock order (03 section 6.3): the
-- team_channels row before the runs of the work threads.

-- name: UpsertTeamChannel :one
-- Switches a channel to team mode with the sponsor, or makes the sponsor the sponsor of an existing
-- team; members and brake state stay.
INSERT INTO team_channels (channel_id, sponsor_id, created_at, updated_at)
VALUES (sqlc.arg(channel_id), sqlc.arg(sponsor_id), sqlc.arg(at), sqlc.arg(at))
ON CONFLICT (channel_id) DO UPDATE SET sponsor_id = EXCLUDED.sponsor_id, updated_at = EXCLUDED.updated_at,
    version = team_channels.version + 1
RETURNING *;

-- name: GetTeamChannel :one
-- The team of one channel without a lock.
SELECT * FROM team_channels WHERE channel_id = sqlc.arg(channel_id);

-- name: LockTeamChannel :one
-- The team of one channel with its row lock: every change of the brake state and every input.
SELECT * FROM team_channels WHERE channel_id = sqlc.arg(channel_id) FOR UPDATE;

-- name: DeleteTeamChannel :execrows
-- Switches team mode off; the members go with it (ON DELETE CASCADE).
DELETE FROM team_channels WHERE channel_id = sqlc.arg(channel_id);

-- name: UpdateTeamBrake :one
-- The brake state of one team (09 3.9.6).
UPDATE team_channels SET
    member_turns = sqlc.arg(member_turns),
    pingpong_from = sqlc.narg(pingpong_from),
    pingpong_to = sqlc.narg(pingpong_to),
    pingpong_count = sqlc.arg(pingpong_count),
    pause_reason = sqlc.narg(pause_reason),
    updated_at = sqlc.arg(at),
    version = version + 1
WHERE channel_id = sqlc.arg(channel_id)
RETURNING *;

-- name: InsertTeamMember :one
-- A new member; a taken handle or the same agent and home channel twice is a unique violation the
-- repository maps.
INSERT INTO team_members (id, channel_id, agent_id, home_channel_id, work_thread_id, handle, created_by, created_at)
VALUES (sqlc.arg(id), sqlc.arg(channel_id), sqlc.arg(agent_id), sqlc.arg(home_channel_id), sqlc.arg(work_thread_id),
    sqlc.arg(handle), sqlc.arg(created_by), sqlc.arg(created_at))
RETURNING *;

-- name: TeamMembers :many
-- The members of one team in the order they were added.
SELECT * FROM team_members WHERE channel_id = sqlc.arg(channel_id) ORDER BY created_at, id;

-- name: TeamMemberByWorkThread :one
-- The member whose work thread is the thread.
SELECT * FROM team_members WHERE work_thread_id = sqlc.arg(work_thread_id);

-- name: DeleteTeamMember :execrows
-- Removes one member of one team; its work thread stays.
DELETE FROM team_members WHERE channel_id = sqlc.arg(channel_id) AND id = sqlc.arg(id);

-- name: InsertTeamInput :execrows
-- Records an input; a source that already reached the member stores nothing.
INSERT INTO team_inputs (message_id, member_id, source_message_id, source, source_member_id, authority_id, created_at)
VALUES (sqlc.arg(message_id), sqlc.arg(member_id), sqlc.arg(source_message_id), sqlc.arg(source), sqlc.narg(source_member_id),
    sqlc.arg(authority_id), sqlc.arg(created_at))
ON CONFLICT DO NOTHING;

-- name: GetTeamInput :one
-- The input of one message in a work thread.
SELECT * FROM team_inputs WHERE message_id = sqlc.arg(message_id);

-- name: TeamInputDelivered :one
-- Whether the source already reached the member.
SELECT EXISTS (SELECT 1 FROM team_inputs WHERE source_message_id = sqlc.arg(source_message_id) AND member_id = sqlc.arg(member_id));

-- name: InsertTeamMirror :execrows
-- Records the mirror of a reply; a reply already mirrored stores nothing.
INSERT INTO team_mirrors (source_message_id, mirror_message_id, member_id, created_at)
VALUES (sqlc.arg(source_message_id), sqlc.arg(mirror_message_id), sqlc.narg(member_id), sqlc.arg(created_at))
ON CONFLICT DO NOTHING;

-- name: GetTeamMirror :one
-- The mirror of one reply.
SELECT * FROM team_mirrors WHERE source_message_id = sqlc.arg(source_message_id);

-- name: TeamHistory :many
-- The newest history of a team channel's main thread that a new member gets as context (09 3.9.4,
-- FEAT-TEAM-CONTEXT-ALL): text messages with text that are not deleted and are either a person's
-- or the mirror of a member reply (team_mirrors), newest first, at most the limit. The use case
-- keeps only active owners' messages and unedited mirrors.
SELECT m.id, m.origin, m.author_principal_id, m.version, (m.edited_at IS NOT NULL)::boolean AS edited,
    tm.member_id AS mirror_member_id, (tm.mirror_message_id IS NOT NULL)::boolean AS mirror
FROM messages m
LEFT JOIN team_mirrors tm ON tm.mirror_message_id = m.id
WHERE m.thread_id = sqlc.arg(thread_id) AND m.deleted_at IS NULL AND m.kind = 'text' AND m.content_md <> ''
    AND (m.origin = 'human' OR tm.mirror_message_id IS NOT NULL)
ORDER BY m.thread_seq DESC
LIMIT sqlc.arg(max_rows);
