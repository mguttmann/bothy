-- SPDX-License-Identifier: Apache-2.0
-- The personal data of one person in push_registrations (M7-06; docs/spec/20-privacy.md sections
-- 4.4 and 5.2). Only the module notify reads and writes the table (03 principle 12). The privacy
-- job selects the person; nothing here authorizes.

-- name: RegistrationsOfUser :many
-- Every registration of the person, active and disabled, ascending by id, for the export. Never
-- the token or the preview hash (20 section 5.2: no secrets, no hashes, no keys).
SELECT id, device_id, provider, environment, app_id, created_at, last_success_at, disabled_at, disabled_reason
FROM push_registrations
WHERE user_id = @user_id
ORDER BY id;

-- name: DeleteRegistrationsOfUser :execrows
-- Removes every registration of the person (step security of the account deletion, 20 4.4 item 2).
DELETE FROM push_registrations
WHERE user_id = @user_id;
