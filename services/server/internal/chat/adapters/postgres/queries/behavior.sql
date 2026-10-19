-- SPDX-License-Identifier: Apache-2.0
-- Queries on the behaviour of channels (chat module, M2-02; docs/spec/03-datenmodell.md section
-- 4.5 channels and channel_skill_bindings). A behaviour is read in one statement: the switches,
-- the context window, the prompt, settings_version, the skill references in their order and the
-- project binding of a forum (M2-03), so a
-- concurrent change never mixes two states. The locks of a change (the space row FOR SHARE, then
-- the channel row FOR UPDATE) are ShareSpace and LockChannel.

-- name: GetChannelBehavior :one
-- The behaviour of one channel, read after authorize allowed channel.behavior.read,
-- channel.behavior.update or agent.config.read on it.
SELECT c.id, c.space_id, c.require_mention, c.thread_require_mention, c.free_response, c.auto_thread,
       c.allow_mentioned_agents, c.history_backfill_limit, c.prompt, c.settings_version,
       ARRAY(SELECT b.skill_ref FROM channel_skill_bindings b WHERE b.channel_id = c.id ORDER BY b.position)::text[] AS skills,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, pb.workspace_ref, pb.repository_url, pb.default_branch,
       pb.runner_config, pb.ready, pb.version AS binding_version
FROM channels c
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.id = @id;

-- name: ListServedBehaviors :many
-- listServedChannels with the predicate of 07 section 7.4 rule 1: the channels and spaces of
-- Scope of agent.config.read that are not archived, ascending by channel id; authorize then
-- checks each row.
SELECT c.id, c.space_id, c.require_mention, c.thread_require_mention, c.free_response, c.auto_thread,
       c.allow_mentioned_agents, c.history_backfill_limit, c.prompt, c.settings_version,
       ARRAY(SELECT b.skill_ref FROM channel_skill_bindings b WHERE b.channel_id = c.id ORDER BY b.position)::text[] AS skills,
       (pb.channel_id IS NOT NULL)::boolean AS project_bound, pb.workspace_ref, pb.repository_url, pb.default_branch,
       pb.runner_config, pb.ready, pb.version AS binding_version
FROM channels c
LEFT JOIN project_bindings pb ON pb.channel_id = c.id
WHERE c.id = ANY (@channel_ids::uuid[]) AND c.space_id = ANY (@space_ids::uuid[]) AND c.archived_at IS NULL
ORDER BY c.id;

-- name: SaveChannelBehavior :exec
-- Writes the switches, the context window and the prompt of the locked channel; settings_version
-- is raised by RaiseSettingsVersion, channels.version stays (M1-53 design note D1).
UPDATE channels
SET require_mention = @require_mention, thread_require_mention = @thread_require_mention, free_response = @free_response,
    auto_thread = @auto_thread, allow_mentioned_agents = @allow_mentioned_agents,
    history_backfill_limit = @history_backfill_limit, prompt = @prompt
WHERE id = @id;

-- name: RaiseSettingsVersion :one
-- Raises settings_version of the locked channel by one: the own counter of behaviour, prompt,
-- skills and project (03 channels).
UPDATE channels SET settings_version = settings_version + 1 WHERE id = @id RETURNING settings_version;

-- name: DeleteChannelSkills :exec
DELETE FROM channel_skill_bindings WHERE channel_id = @channel_id;

-- name: InsertChannelSkills :exec
-- Inserts the skill references in their order; positions count from zero.
INSERT INTO channel_skill_bindings (channel_id, position, skill_ref)
SELECT @channel_id::uuid, (s.ord - 1)::integer, s.ref
FROM unnest(@skill_refs::text[]) WITH ORDINALITY AS s (ref, ord);
