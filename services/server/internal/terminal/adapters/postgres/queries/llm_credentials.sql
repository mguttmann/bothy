-- SPDX-License-Identifier: Apache-2.0
-- Queries of the declaration of the LLM login of a host (M2-40, 24 section 7.2, I6).

-- name: UpdateHostLlmCredentials :execrows
-- Writes the declaration with who declared it and when, and the next version; the caller holds
-- the lock of the row and checked the version (03 section 6.8).
UPDATE hosts
SET llm_credentials = @llm_credentials, credential_holder_id = sqlc.narg(credential_holder_id),
    llm_credentials_declared_by = @declared_by, llm_credentials_declared_at = @declared_at, version = @version
WHERE id = @id;

-- name: ListPersonalHosts :many
-- The hosts with a personal LLM login, ascending (the I6 check of every rights change).
SELECT * FROM hosts WHERE llm_credentials = 'personal' ORDER BY id;
