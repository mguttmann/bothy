-- SPDX-License-Identifier: Apache-2.0
-- Queries on auth_challenges and the step-up columns of auth_sessions (identity,
-- docs/spec/03-data-model.md section 4.2, docs/spec/08-identity.md section 6.2, M1-35).
-- Challenges are stored only as HMAC; a completion locks the session rows first, then the
-- challenge (03 "Lock order").

-- name: InsertChallenge :exec
-- Every purpose (03 auth_challenges): session_id for step_up and passkey_register,
-- enrollment_code_id for passkey_enroll, webauthn_session for the WebAuthn ceremonies (M1-40).
INSERT INTO auth_challenges (id, purpose, session_id, enrollment_code_id, challenge_hash, webauthn_session, status,
                             created_at, expires_at)
VALUES (@id, @purpose, sqlc.narg(session_id), sqlc.narg(enrollment_code_id), @challenge_hash, sqlc.narg(webauthn_session),
        'issued', @created_at, @expires_at);

-- name: LockChallenge :one
SELECT * FROM auth_challenges WHERE id = @id FOR UPDATE;

-- name: FinishChallenge :exec
-- Ends an issued challenge as used or failed; a finished one stays unchanged.
UPDATE auth_challenges SET status = @status, finished_at = @finished_at
WHERE id = @id AND status = 'issued';

-- name: FailedStepUpTimes :many
-- finished_at of the failed step-up challenges of a session after since (the start of
-- auth.login_fail_window), over the partial index auth_challenges_session_failed_idx.
SELECT finished_at::timestamptz AS finished_at FROM auth_challenges
WHERE session_id = @session_id AND status = 'failed' AND purpose = 'step_up' AND finished_at > @since
ORDER BY finished_at;

-- name: SetSessionStepUp :exec
UPDATE auth_sessions SET step_up_at = @step_up_at, step_up_method = @step_up_method WHERE id = @id;

-- name: DeleteExpiredChallenges :execrows
-- The job identity.auth_cleanup: every challenge past expires_at, except failed step-up challenges
-- whose finished_at still lies inside auth.login_fail_window (keep_failed_since), because they
-- count toward auth.step_up_fail_limit (08 section 13).
DELETE FROM auth_challenges
WHERE expires_at <= @now
  AND NOT (purpose = 'step_up' AND status = 'failed' AND finished_at > @keep_failed_since);

-- name: LockSessionForStepUpConsumption :one
-- The login session of a terminal opening, locked as the first row lock of its transaction, so a
-- second opening with the same confirmation waits and sees the consumption (08 section 6.3 point
-- 4, 03 section 6.3; M2-40).
SELECT * FROM auth_sessions WHERE id = @id AND user_id = @user_id FOR UPDATE;

-- name: ConsumeStepUp :execrows
-- Consumes the step-up of a session: one confirmation opens exactly one terminal session.
UPDATE auth_sessions SET step_up_consumed_at = @consumed_at WHERE id = @id AND step_up_at IS NOT NULL;
