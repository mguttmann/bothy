-- SPDX-License-Identifier: Apache-2.0
-- The participant of agents in the privacy export (M7-06, docs/spec/20-privacy.md section 5.2):
-- the agents whose LLM credentials a person holds (credential_holder_id, billing mode personal).

-- name: PersonalDataAgentsHeld :many
SELECT principal_id FROM agents WHERE credential_holder_id = @subject ORDER BY principal_id;
