-- SPDX-License-Identifier: Apache-2.0
-- run_answers (docs/spec/03-data-model.md section 4.8, FEAT-TEAM-CHANNEL-SERVER): the run an
-- agent's message answers. The acceptance writes it, the run context reads it. Every statement
-- names one message the use case is handling (07 section 7.4 rule 1); the table holds no content
-- and is no content table of tools/lint/sql_visibility.yaml.

-- name: InsertRunAnswer :exec
-- Links the message to the run; a link already stored changes nothing.
INSERT INTO run_answers (message_id, run_id) VALUES (sqlc.arg(message_id), sqlc.arg(run_id))
ON CONFLICT DO NOTHING;

-- name: RunAnswer :one
-- The run the message answers.
SELECT run_id FROM run_answers WHERE message_id = sqlc.arg(message_id);
