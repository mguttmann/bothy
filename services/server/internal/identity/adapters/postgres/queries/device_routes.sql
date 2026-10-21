-- SPDX-License-Identifier: Apache-2.0
-- The active devices of many people for the worker consumer push of notify (M3-19 design note 8,
-- identity.Devices.ActiveForUsers). No visibility predicate: the consumer selected the people and
-- decides every hint with authorize (07 section 7.4 rule 1).

-- name: ListActiveDevicesOfUsers :many
-- Uses devices_user_id_state_idx; ordered by person, then id, so the lists come out ascending.
SELECT d.id, d.user_id, d.platform
FROM devices d
WHERE d.user_id = ANY(@user_ids::uuid[]) AND d.state = 'active'
ORDER BY d.user_id, d.id;
