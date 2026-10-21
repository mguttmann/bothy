-- SPDX-License-Identifier: Apache-2.0
-- Push targets of devices (M3-19): push_registrations exactly as docs/spec/03-datenmodell.md
-- section 4.10 states it, without deviation. A push carries no content (ADR 0031): the row holds
-- the provider token of one device of one session and the delivery state the job
-- notify.push_send writes (last_success_at, failure_count, disabled_at, disabled_reason). The
-- token and preview_token_hash never leave the adapters of notify. Creation, removal and the
-- deactivation on logout and device revocation are M3-22. Retention class: security.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert, update and delete
-- (M3-22 removes registrations; the retention job of class security deletes old rows).

-- +goose Up
CREATE TABLE push_registrations (
    id                 uuid        NOT NULL,
    user_id            uuid        NOT NULL,
    device_id          uuid        NOT NULL,
    auth_session_id    uuid        NOT NULL,
    provider           text        NOT NULL CHECK (provider IN ('apns', 'fcm', 'unifiedpush', 'wns')),
    token              text        NOT NULL,
    environment        text        NOT NULL DEFAULT 'production' CHECK (environment IN ('production', 'sandbox')),
    app_id             text        NOT NULL,
    preview_token_hash bytea,
    created_at         timestamptz NOT NULL DEFAULT now(),
    last_success_at    timestamptz,
    failure_count      integer     NOT NULL DEFAULT 0,
    disabled_at        timestamptz,
    disabled_reason    text        CHECK (disabled_reason IN ('token_invalid', 'device_revoked', 'logout', 'topic_mismatch',
                                                              'sender_mismatch', 'repeated_failures')),
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE CASCADE,
    FOREIGN KEY (auth_session_id) REFERENCES auth_sessions (id) ON DELETE CASCADE,
    -- A deactivated row and only a deactivated one has its reason.
    CHECK ((disabled_at IS NULL) = (disabled_reason IS NULL)),
    UNIQUE (provider, token)
);
-- The preview endpoint finds the registration by the HMAC of its preview token (13 section 6.4).
CREATE UNIQUE INDEX push_registrations_preview_token_hash_key ON push_registrations (preview_token_hash)
    WHERE preview_token_hash IS NOT NULL;
-- The consumer push and its jobs read the active registrations of people.
CREATE INDEX push_registrations_user_id_active_idx ON push_registrations (user_id) WHERE disabled_at IS NULL;
-- Ending a session deactivates its registrations (M3-22).
CREATE INDEX push_registrations_auth_session_id_idx ON push_registrations (auth_session_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON push_registrations TO bothy_app;

-- +goose Down
DROP TABLE push_registrations;
