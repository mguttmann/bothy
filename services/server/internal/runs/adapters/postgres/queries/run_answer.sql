-- SPDX-License-Identifier: Apache-2.0
-- The answer check of M2-19 (design notes 4 and 7): the run an agent names in run_id, read by
-- primary key inside the unit of work of an authorized message.create, after messaging holds the
-- thread row (thread, then run, as the acceptance of M2-05). FOR SHARE lets an end of the run
-- (finishRun, cancelRun, the cancel timeout, the stall check) wait for the commit of the answer,
-- or the answer wait for the end and then see the end state. Only the verdict leaves the module
-- (tools/lint/sql_visibility.yaml).

-- name: LockRunForAnswer :one
-- Thread, agent and state of the named run with a shared row lock for the rest of the unit.
SELECT id, thread_id, agent_id, state FROM runs WHERE id = sqlc.arg(id) FOR SHARE;
