-- SPDX-License-Identifier: Apache-2.0
-- The participant of runs in the privacy export (M7-06, docs/spec/20-privacy.md section 5.2): the
-- runs a person triggered and the interactions a person submitted, read by the job privacy.export
-- without a principal. runs and interactions have no channel_id; the rows are named by the
-- person, and only IDs, states, times, costs and the person's own values leave (no agent text).

-- name: PersonalDataRuns :many
SELECT id, thread_id, agent_id, trigger_kind, state, queued_at, started_at, finished_at,
       cost_micros, currency, cost_is_estimate, metrics
FROM runs WHERE triggered_by = @subject ORDER BY queued_at, id;

-- name: PersonalDataInteractions :many
SELECT id, component_id, message_id, state, created_at, "values"
FROM interactions WHERE principal_id = @subject ORDER BY created_at, id;
