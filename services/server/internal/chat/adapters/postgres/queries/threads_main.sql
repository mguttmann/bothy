-- SPDX-License-Identifier: Apache-2.0
-- The main thread of a text channel (chat module, M1-53; docs/spec/03-data-model.md section
-- 4.5, R-DM-11): inserted in the transaction that creates the channel. The thread use cases on
-- the same table come with M1-55 (docs/DECISIONS.md M1-53/6).

-- name: InsertMainThread :one
INSERT INTO threads (id, channel_id, kind, created_by, created_at, last_activity_at)
VALUES (@id, @channel_id, 'main', @created_by, @created_at, @created_at)
RETURNING *;
