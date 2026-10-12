-- SPDX-License-Identifier: Apache-2.0
-- Devices and device keys of the identity module (M1-33): devices and device_keys exactly as
-- docs/spec/03-data-model.md section 4.2 states them. Retention class of both tables: security
-- (03 section 3). IDs are UUIDv7 from kernel/ids (the device id from the client, R-DM-01);
-- enumerations are text with CHECK (R-DM-02).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update both tables
-- but not delete. A device is revoked by state and revoked_at, a key by revoked_at, and the rows
-- are security history; the retention job of class security grants its delete in its own
-- migration (docs/DECISIONS.md M1-33).

-- +goose Up
CREATE TABLE devices (
    id            uuid        NOT NULL,
    user_id       uuid        NOT NULL,
    name          text        NOT NULL,
    platform      text        NOT NULL
                              CHECK (platform IN ('ios', 'android', 'macos', 'windows', 'web', 'cli')),
    state         text        NOT NULL DEFAULT 'pending'
                              CHECK (state IN ('pending', 'active', 'revoked')),
    app_version   text,
    approved_by   uuid,
    approved_at   timestamptz,
    revoked_by    uuid,
    revoked_at    timestamptz,
    revoke_reason text        CHECK (revoke_reason IN ('revoked_by_self', 'revoked_by_owner', 'user_disabled',
                                                       'inactive', 'verification_failed', 'pending_expired',
                                                       'pairing_expired')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (approved_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (revoked_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- A revoked device and only a revoked one has revoked_at; a reason only on a revoked device.
    CONSTRAINT devices_revoked_at_check CHECK ((state = 'revoked') = (revoked_at IS NOT NULL)),
    CONSTRAINT devices_revoke_reason_state_check CHECK (revoke_reason IS NULL OR state = 'revoked')
);
-- The device list of a person.
CREATE INDEX devices_user_id_state_idx ON devices (user_id, state);
-- The approval queue of the admin console.
CREATE INDEX devices_pending_idx ON devices (state) WHERE state = 'pending';

CREATE TABLE device_keys (
    id          uuid        NOT NULL,
    device_id   uuid        NOT NULL,
    purpose     text        NOT NULL CHECK (purpose IN ('dpop', 'step_up')),
    jkt         text        NOT NULL,
    public_jwk  jsonb       NOT NULL,
    alg         text        NOT NULL CHECK (alg IN ('ES256', 'EdDSA')),
    attestation jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE
);
-- A thumbprint belongs to one key of one device for good (cnf.jkt names exactly one key).
CREATE UNIQUE INDEX device_keys_jkt_key ON device_keys (jkt);
-- At most one active key per device and purpose (08 section 6.1).
CREATE UNIQUE INDEX device_keys_active_purpose_key ON device_keys (device_id, purpose) WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON devices, device_keys TO bothy_app;

-- +goose Down
DROP TABLE device_keys;
DROP TABLE devices;
