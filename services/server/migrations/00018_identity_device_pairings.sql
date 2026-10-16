-- SPDX-License-Identifier: Apache-2.0
-- Device pairings of the identity module (M1-38): device_pairings exactly as
-- docs/spec/03-data-model.md section 4.2 states it. Retention class: transient (03 section 3).
-- A pairing code is stored only as HMAC-SHA-256 with the token pepper (03 Grundsatz 9); the code
-- itself appears only on the new device.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update pairings but
-- not delete them; a pairing ends by its status (completed, rejected, expired), set on access and
-- by the job identity.auth_cleanup. Deleting old rows belongs to the retention job, which grants
-- its delete in its own migration.

-- +goose Up
CREATE TABLE device_pairings (
    id                    uuid        NOT NULL,
    pairing_code_hash     bytea       NOT NULL,
    device_registration   jsonb       NOT NULL,
    status                text        NOT NULL DEFAULT 'requested'
                                      CHECK (status IN ('requested', 'approved', 'completed', 'rejected', 'expired')),
    user_id               uuid,
    approved_by_device_id uuid,
    device_id             uuid,
    created_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    completed_at          timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (approved_by_device_id) REFERENCES devices (id) ON DELETE SET NULL,
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE SET NULL,
    -- An approved or completed pairing names the approving person; a completed one its device.
    CONSTRAINT device_pairings_user_check CHECK (status NOT IN ('approved', 'completed') OR user_id IS NOT NULL),
    CONSTRAINT device_pairings_device_check CHECK (status <> 'completed' OR device_id IS NOT NULL)
);
-- A code matches at most one open request; a closed pairing frees its code.
CREATE UNIQUE INDEX device_pairings_code_hash_requested_key ON device_pairings (pairing_code_hash) WHERE status = 'requested';
-- The cleanup job finds the expired pairings.
CREATE INDEX device_pairings_expires_at_idx ON device_pairings (expires_at);

GRANT SELECT, INSERT, UPDATE ON device_pairings TO bothy_app;

-- +goose Down
DROP TABLE device_pairings;
