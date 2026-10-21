-- SPDX-License-Identifier: Apache-2.0
-- A single resource loaded after authorize allowed it.

-- name: GetMessage :one
SELECT * FROM messages WHERE id = @id;

-- name: LockMessage :one
SELECT * FROM messages WHERE id = @id FOR UPDATE;
