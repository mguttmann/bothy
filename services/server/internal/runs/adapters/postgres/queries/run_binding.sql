-- SPDX-License-Identifier: Apache-2.0
-- The acting-for binding of a request (07 guard rule SR9, brief M2-14 Design note 3). This read
-- carries no visible-channel predicate: the authentication of a request with the Invocation-Token
-- header loads the one run named by the verified token by primary key, before any authorize, to
-- bind the request to the person who triggered the run, and nothing of the row leaves the server
-- (07 section 7.4 rule 1, exception "resolution for authorize"; tools/lint/sql_visibility.yaml).
-- It runs on the pool, outside a unit of work and without a lock (Design note 3g).

-- name: GetRunBinding :one
SELECT id, agent_id, triggered_by, thread_id, state FROM runs WHERE id = sqlc.arg(id);
