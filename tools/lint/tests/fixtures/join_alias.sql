-- SPDX-License-Identifier: Apache-2.0
-- A join of two content tables; the predicate names the alias of one of them.

-- name: ThreadsWithChannel :many
SELECT t.id, c.name
  FROM threads AS t
  JOIN channels c ON c.id = t.channel_id
 WHERE t.channel_id = ANY (@visible::uuid[])
 ORDER BY t.id;
