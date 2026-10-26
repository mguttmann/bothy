-- SPDX-License-Identifier: Apache-2.0
-- The participant of apps in the privacy export (M7-06, docs/spec/20-privacy.md section 5.2): the
-- app installations a person made.

-- name: PersonalDataInstallations :many
SELECT id, app_id, state, installed_at, uninstalled_at
FROM app_installations WHERE installed_by = @subject ORDER BY installed_at, id;
