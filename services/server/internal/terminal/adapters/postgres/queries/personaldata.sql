-- SPDX-License-Identifier: Apache-2.0
-- The participant of terminal in the privacy export (M7-06, docs/spec/20-privacy.md section 5.2):
-- the hosts a person is the assignee or credential holder of (a host the person only created is
-- the host of someone else: its name and times are that person's data; creating it is in the
-- audit entries), the person's session users,
-- terminal sessions and computer sessions. Only names, states, times and counts leave: never keys, fingerprints,
-- pairing codes, reported data of the host, token IDs or the close detail.

-- name: PersonalDataHosts :many
SELECT id, name, state, platform, assigned_user_id IS NOT DISTINCT FROM @subject AS assignee, created_by = @subject AS creator,
       credential_holder_id IS NOT DISTINCT FROM @subject AS credential_holder, created_at, paired_at, last_seen_at, retired_at
FROM hosts
WHERE assigned_user_id = @subject OR credential_holder_id = @subject
ORDER BY created_at, id;

-- name: PersonalDataHostSessionUsers :many
SELECT id, host_id, os_user, created_at, revoked_at
FROM host_session_users WHERE user_id = @subject ORDER BY created_at, id;

-- name: PersonalDataTerminalSessions :many
SELECT id, host_id, device_id, profile, state, opened_at, started_at, closed_at, close_reason, bytes_in, bytes_out
FROM terminal_sessions WHERE user_id = @subject ORDER BY opened_at, id;

-- name: PersonalDataComputerSessions :many
-- The person's computer sessions (27, 20 section 2): never the token ID, the generations, the
-- close detail or who stopped the session.
SELECT id, host_id, agent_id, run_id, state, opened_at, started_at, closed_at, close_reason, paused_ms, action_count, front_app
FROM computer_sessions WHERE user_id = @subject ORDER BY opened_at, id;
