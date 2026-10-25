-- SPDX-License-Identifier: Apache-2.0
-- The personal data of messaging (M7-06, docs/spec/20-privacy.md sections 4.4 and 5.2): the rows
-- of one person for the export and the account deletion. The privacy jobs run without a principal
-- and without a reader of other channels: each statement selects by the person's own ID, so no
-- visible-channel predicate applies (tools/lint/sql_visibility.yaml, category system_job). Only this
-- module reads and writes these tables (03 principle 12).

-- name: ListOwnMessages :many
-- A batch of the messages the person wrote, tombstones included, after the keyset position, by
-- id. System lines carry no content of the person.
SELECT id, thread_id, channel_id, kind, content_md, has_attachments, created_at, edited_at, deleted_at FROM messages
WHERE author_principal_id = @author_id AND kind <> 'system' AND id > @after_id
ORDER BY id
LIMIT @row_limit;

-- name: ListOwnRevisions :many
-- The revisions of the given messages that the person made, by message and number.
SELECT message_id, revision, content_md, created_at FROM message_revisions
WHERE message_id = ANY (@message_ids::uuid[]) AND edited_by = @author_id
ORDER BY message_id, revision;

-- name: ListOwnReactions :many
-- A batch of the reactions of the person after the keyset position (message, symbol).
SELECT message_id, emoji, created_at FROM reactions
WHERE principal_id = @principal_id AND (message_id, emoji) > (@after_message_id::uuid, @after_emoji::text)
ORDER BY message_id, emoji
LIMIT @row_limit;

-- name: ListMentionsOf :many
-- A batch of the mentions of the person after the keyset position: message and time only.
SELECT message_id, created_at FROM mentions
WHERE principal_id = @principal_id AND message_id > @after_message_id
ORDER BY message_id
LIMIT @row_limit;

-- name: ListAllPersonalTemplates :many
-- Every personal template of the person, deleted ones that are still stored included, by id.
SELECT id, name, content_md, created_at, deleted_at FROM message_templates
WHERE owner_user_id = @owner_user_id
ORDER BY id;

-- name: DeletePersonalTemplates :execrows
-- Step 3 of the account deletion (20 section 4.4): the personal templates go with the account;
-- the templates of a channel the person wrote stay (00079 grants the DELETE).
DELETE FROM message_templates WHERE owner_user_id = @owner_user_id;

-- name: ListLiveOwnMessages :many
-- Step 5 with privacy.account_deletion_content = delete: a batch of the messages of the person not
-- deleted yet, with their channel, by id; each is deleted through the deletion path of 20 4.2.
SELECT id, channel_id FROM messages
WHERE author_principal_id = @author_id AND deleted_at IS NULL AND kind <> 'system'
ORDER BY id
LIMIT @row_limit;

-- name: ListLiveOwnReactions :many
-- Step 5 with privacy.account_deletion_content = delete: a batch of the reactions of the person on
-- messages not deleted yet, with the channel of the message; each is removed like reaction.remove.
-- Reactions on deleted messages are gone with the message (20 section 4.2).
SELECT r.message_id, m.channel_id, r.emoji FROM reactions r
JOIN messages m ON m.id = r.message_id
WHERE r.principal_id = @principal_id AND m.deleted_at IS NULL
ORDER BY r.message_id, r.emoji
LIMIT @row_limit;
