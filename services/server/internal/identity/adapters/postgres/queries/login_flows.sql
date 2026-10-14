-- SPDX-License-Identifier: Apache-2.0
-- Queries on auth_login_flows (identity, docs/spec/03-datenmodell.md section 4.2, M1-41). state,
-- nonce and login_code are stored only as their HMAC; no query reads or writes them.

-- name: InsertLoginFlow :exec
INSERT INTO auth_login_flows (id, provider_id, mode, user_id, oidc_state_hash, oidc_nonce_hash, client_challenge,
    app_redirect_uri, enrollment_code_id, created_at, expires_at)
VALUES (@id, @provider_id, @mode, sqlc.narg(user_id), @oidc_state_hash, @oidc_nonce_hash, @client_challenge,
    @app_redirect_uri, sqlc.narg(enrollment_code_id), @created_at, @expires_at);

-- name: LoginFlowByStateHash :one
-- The flow of a callback. It is read without a lock, because the callback talks to the provider
-- before it writes and no outgoing call runs inside a transaction (02 section 8.5); the write is
-- conditional on the status instead (IssueLoginCode, FailLoginFlow).
SELECT * FROM auth_login_flows WHERE oidc_state_hash = @oidc_state_hash;

-- name: LockLoginFlowByCodeHash :one
-- The flow of a completion, locked until the end of its unit of work, so a login_code is redeemed
-- once.
SELECT * FROM auth_login_flows WHERE login_code_hash = @login_code_hash FOR UPDATE;

-- name: IssueLoginCode :execrows
-- The callback verified the ID token: the external identity and the login_code, only from started.
UPDATE auth_login_flows
SET external_identity = @external_identity, login_code_hash = @login_code_hash, login_code_expires_at = @login_code_expires_at,
    status = 'code_issued'
WHERE id = @id AND status = 'started';

-- name: FailLoginFlow :execrows
-- Every failure ends the flow; the external identity is dropped (data minimisation).
UPDATE auth_login_flows SET status = 'failed', error = @error, external_identity = NULL
WHERE id = @id AND status IN ('started', 'code_issued');

-- name: CompleteLoginFlow :execrows
-- The login_code was redeemed; the external identity is dropped (data minimisation).
UPDATE auth_login_flows SET status = 'completed', external_identity = NULL
WHERE id = @id AND status = 'code_issued';

-- name: DeleteExpiredLoginFlows :execrows
-- The job identity.auth_cleanup: flows past expires_at.
DELETE FROM auth_login_flows WHERE expires_at < @now;
