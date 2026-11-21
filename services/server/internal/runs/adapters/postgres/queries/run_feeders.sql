-- SPDX-License-Identifier: Apache-2.0
-- run_feeders (docs/spec/03-data-model.md section 4.8, FEAT-TEAM-CHANNEL-SERVER): who fed a run.
-- The triggers of migration 00108 record the trigger, the follow-ups and the interactions; the
-- runs module records the deliveries and the run context here. The table has no channel_id:
-- every statement names the one run whose delivery, context or reply the use case is handling
-- (07 section 7.4 rule 1, D-M2-05-9). It holds no content and is no content table of
-- tools/lint/sql_visibility.yaml, so the read needs no entry there.

-- name: InsertRunFeeders :exec
-- Records the principals of one source of a run; a NULL element is an input that speaks for no
-- person (another agent's or an app's message), a row already stored changes nothing, and a run that is gone (a late command.invoked of
-- a deleted thread) records nothing.
INSERT INTO run_feeders (run_id, principal_id, source)
SELECT r.id, p, sqlc.arg(source) FROM runs r, unnest(sqlc.arg(principal_ids)::uuid[]) AS p
WHERE r.id = sqlc.arg(run_id)
ON CONFLICT DO NOTHING;

-- name: RunFeeders :many
-- Every recorded feeder of one run.
SELECT principal_id, source FROM run_feeders WHERE run_id = sqlc.arg(run_id) ORDER BY source, principal_id;
