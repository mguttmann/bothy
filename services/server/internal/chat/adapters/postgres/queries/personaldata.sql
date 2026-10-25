-- SPDX-License-Identifier: Apache-2.0
-- The personal data of chat (M7-06, docs/spec/20-privacy.md sections 4.4 and 5.2): the sidebar of
-- one person. Only this module reads and writes user_space_states (03 principle 12).

-- name: ListPersonalSidebar :many
-- Every sidebar row of the person for the export, archived spaces included, by space.
SELECT space_id, collapsed, sort_key, updated_by_device_id, updated_at FROM user_space_states
WHERE user_id = @user_id
ORDER BY space_id;

-- name: DeletePersonalSidebar :execrows
-- Step 3 of the account deletion (20 section 4.4): the sidebar goes with the account.
DELETE FROM user_space_states WHERE user_id = @user_id;
