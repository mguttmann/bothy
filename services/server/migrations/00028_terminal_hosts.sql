-- SPDX-License-Identifier: Apache-2.0
-- Host records of the terminal module (M2-39): hosts, host_session_users and terminal_sessions
-- exactly as docs/spec/03-datenmodell.md section 4.14 states them, and the foreign key
-- grants.target_host_id that M1-27 deferred to this migration. IDs come from kernel/ids
-- (R-DM-01); enumerations are text with CHECK (R-DM-02). Retention class of all three tables:
-- security (03 section 3; the jobs of class security come with M7).
--
-- hosts.dev_environment_id is a plain uuid: the table dev_environments comes with V2-01, which
-- adds the foreign key, as M1-27 did for its deferred keys. The column stays NULL until then.
--
-- terminal_sessions holds metadata only, never content or commands (R-DM-36). Its session user
-- belongs to the same host and person through the composite foreign key on
-- host_session_users(id, host_id, user_id) (R-DM-40, D-W-66).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update, not delete.
-- A session user row is never deleted while its host is not retired, or its name would be free
-- again (ruling T12, R-DM-39); hosts and sessions are security history.

-- +goose Up
CREATE TABLE hosts (
    id                             uuid        NOT NULL,
    name                           text        NOT NULL,
    state                          text        NOT NULL DEFAULT 'pairing',
    platform                       text,
    assigned_user_id               uuid,
    dev_environment_id             uuid,
    llm_credentials                text        NOT NULL DEFAULT 'none',
    credential_holder_id           uuid,
    llm_credentials_declared_by    uuid,
    llm_credentials_declared_at    timestamptz,
    declared_server_host           boolean     NOT NULL DEFAULT false,
    pairing_code_hash              bytea,
    pairing_expires_at             timestamptz,
    noise_public_key               bytea,
    auth_public_jwk                jsonb,
    auth_jkt                       text,
    key_fingerprint                text,
    key_generation                 integer     NOT NULL DEFAULT 0,
    fingerprint_failures           integer     NOT NULL DEFAULT 0,
    paired_at                      timestamptz,
    confirmed_by                   uuid,
    confirmed_at                   timestamptz,
    reported_profiles              jsonb       NOT NULL DEFAULT '[]',
    reported_session_users         jsonb       NOT NULL DEFAULT '[]',
    reported_doctor                jsonb       NOT NULL DEFAULT '[]',
    reported_is_server_host        boolean,
    reported_require_local_confirm boolean,
    reported_os_version            text,
    reported_host_version          text,
    last_seen_at                   timestamptz,
    created_by                     uuid        NOT NULL,
    created_at                     timestamptz NOT NULL DEFAULT now(),
    disabled_at                    timestamptz,
    disabled_reason                text,
    retired_at                     timestamptz,
    version                        bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (assigned_user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (credential_holder_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (llm_credentials_declared_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (confirmed_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- The life cycle of 24 section 3.1.
    CONSTRAINT hosts_state_check CHECK (state IN ('pairing', 'confirming', 'active', 'disabled', 'retired')),
    CONSTRAINT hosts_platform_check CHECK (platform IN ('macos', 'linux', 'windows')),
    CONSTRAINT hosts_llm_credentials_check CHECK (llm_credentials IN ('none', 'personal', 'organization')),
    CONSTRAINT hosts_disabled_reason_check CHECK (disabled_reason IN ('owner', 'duplicate_connection')),
    -- X25519, exactly 32 bytes.
    CONSTRAINT hosts_noise_public_key_length_check CHECK (octet_length(noise_public_key) = 32),
    -- A credential holder exactly with personal (I6).
    CONSTRAINT hosts_personal_holder_check CHECK ((llm_credentials = 'personal') = (credential_holder_id IS NOT NULL)),
    -- A pairing code hash exactly in pairing.
    CONSTRAINT hosts_pairing_hash_check CHECK ((state = 'pairing') = (pairing_code_hash IS NOT NULL)),
    -- Keys in every paired state.
    CONSTRAINT hosts_keys_check CHECK (state NOT IN ('confirming', 'active', 'disabled')
        OR (noise_public_key IS NOT NULL AND auth_public_jwk IS NOT NULL AND auth_jkt IS NOT NULL AND key_fingerprint IS NOT NULL)),
    CONSTRAINT hosts_confirmed_check CHECK (state NOT IN ('active', 'disabled') OR confirmed_at IS NOT NULL),
    CONSTRAINT hosts_retired_check CHECK ((state = 'retired') = (retired_at IS NOT NULL)),
    CONSTRAINT hosts_disabled_check CHECK ((state = 'disabled') = (disabled_reason IS NOT NULL))
);
CREATE UNIQUE INDEX hosts_auth_jkt_key ON hosts (auth_jkt) WHERE auth_jkt IS NOT NULL;
CREATE UNIQUE INDEX hosts_pairing_code_hash_key ON hosts (pairing_code_hash) WHERE pairing_code_hash IS NOT NULL;
-- A name is unique among the hosts that are not retired, without regard to case.
CREATE UNIQUE INDEX hosts_name_live_key ON hosts (lower(name)) WHERE state <> 'retired';
-- SR10 reads the active hosts of a holder.
CREATE INDEX hosts_assigned_user_active_idx ON hosts (assigned_user_id) WHERE state = 'active';
-- I6 reads the personal hosts of a credential holder.
CREATE INDEX hosts_credential_holder_personal_idx ON hosts (credential_holder_id) WHERE llm_credentials = 'personal';
-- The sweep job terminal.sweep reads expired pairing codes (02 section 12.3).
CREATE INDEX hosts_pairing_expires_idx ON hosts (pairing_expires_at) WHERE state = 'pairing';

CREATE TABLE host_session_users (
    id         uuid        NOT NULL,
    host_id    uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    os_user    text        NOT NULL,
    created_by uuid        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_by uuid,
    revoked_at timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (host_id) REFERENCES hosts (id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (revoked_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- A name of the operating system, never a path; the length is host.os_user_max_chars.
    CONSTRAINT host_session_users_os_user_check CHECK (os_user ~ '^[A-Za-z0-9_][A-Za-z0-9_.-]*$'),
    CONSTRAINT host_session_users_revoked_check CHECK ((revoked_at IS NULL) = (revoked_by IS NULL)),
    -- The target of the composite foreign key of terminal_sessions (R-DM-40).
    CONSTRAINT host_session_users_id_host_user_key UNIQUE (id, host_id, user_id)
);
-- A name belongs to at most one person per host, also after the end of its assignment (T12).
CREATE UNIQUE INDEX host_session_users_host_os_user_key ON host_session_users (host_id, lower(os_user));
-- At most one valid assignment per person and host.
CREATE UNIQUE INDEX host_session_users_host_user_valid_key ON host_session_users (host_id, user_id) WHERE revoked_at IS NULL;
-- The end of the sessions when a person is disabled reads the valid assignments of the person.
CREATE INDEX host_session_users_user_valid_idx ON host_session_users (user_id) WHERE revoked_at IS NULL;

CREATE TABLE terminal_sessions (
    id                   uuid        NOT NULL,
    host_id              uuid        NOT NULL,
    user_id              uuid        NOT NULL,
    host_session_user_id uuid        NOT NULL,
    opened_by_device_id  uuid        NOT NULL,
    device_id            uuid        NOT NULL,
    auth_session_id      uuid        NOT NULL,
    profile              text        NOT NULL,
    state                text        NOT NULL DEFAULT 'opening',
    token_jti            uuid        NOT NULL,
    key_generation       integer     NOT NULL,
    opened_at            timestamptz NOT NULL DEFAULT now(),
    started_at           timestamptz,
    detached_at          timestamptz,
    last_input_at        timestamptz,
    closed_at            timestamptz,
    closed_by            uuid,
    close_reason         text,
    close_detail         text,
    exit_code            integer,
    bytes_in             bigint      NOT NULL DEFAULT 0,
    bytes_out            bigint      NOT NULL DEFAULT 0,
    version              bigint      NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    FOREIGN KEY (host_id) REFERENCES hosts (id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    -- The session user belongs to the host and the person of the session (R-DM-40).
    CONSTRAINT terminal_sessions_session_user_fkey FOREIGN KEY (host_session_user_id, host_id, user_id)
        REFERENCES host_session_users (id, host_id, user_id) ON DELETE RESTRICT,
    FOREIGN KEY (opened_by_device_id) REFERENCES devices (id) ON DELETE RESTRICT,
    FOREIGN KEY (device_id) REFERENCES devices (id) ON DELETE RESTRICT,
    FOREIGN KEY (auth_session_id) REFERENCES auth_sessions (id) ON DELETE RESTRICT,
    FOREIGN KEY (closed_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT terminal_sessions_token_jti_key UNIQUE (token_jti),
    -- The states of 24 section 6.6.
    CONSTRAINT terminal_sessions_state_check CHECK (state IN ('opening', 'active', 'detached', 'closed')),
    -- The closed list of 24 section 10.3.
    CONSTRAINT terminal_sessions_close_reason_check CHECK (close_reason IN ('client_closed', 'closed_by_owner',
        'process_exited', 'idle_timeout', 'max_duration', 'detach_expired', 'access_revoked', 'i6_violation',
        'host_disabled', 'host_retired', 'host_rekeyed', 'host_restarted', 'user_disabled', 'device_revoked',
        'session_revoked', 'logout', 'open_failed', 'host_policy_violation')),
    CONSTRAINT terminal_sessions_closed_check CHECK ((state = 'closed') = (closed_at IS NOT NULL)),
    CONSTRAINT terminal_sessions_reason_check CHECK ((state = 'closed') = (close_reason IS NOT NULL)),
    CONSTRAINT terminal_sessions_detail_check CHECK (close_detail IS NULL OR state = 'closed'),
    CONSTRAINT terminal_sessions_detached_check CHECK ((state = 'detached') = (detached_at IS NOT NULL))
);
-- terminal.max_sessions_per_person and the end at a revocation read the open sessions of a person.
CREATE INDEX terminal_sessions_user_open_idx ON terminal_sessions (user_id) WHERE state <> 'closed';
CREATE INDEX terminal_sessions_host_open_idx ON terminal_sessions (host_id) WHERE state <> 'closed';
CREATE INDEX terminal_sessions_auth_session_open_idx ON terminal_sessions (auth_session_id) WHERE state <> 'closed';
CREATE INDEX terminal_sessions_device_open_idx ON terminal_sessions (device_id) WHERE state <> 'closed';
-- The retention of class security reads by age.
CREATE INDEX terminal_sessions_opened_at_brin ON terminal_sessions USING brin (opened_at);

-- The key M1-27 deferred: a grant on a host references a real row. No path created a host
-- grant before this migration, so the constraint validates on the existing rows.
ALTER TABLE grants ADD CONSTRAINT grants_target_host_id_fkey
    FOREIGN KEY (target_host_id) REFERENCES hosts (id) ON DELETE RESTRICT;

GRANT SELECT, INSERT, UPDATE ON hosts, host_session_users, terminal_sessions TO bothy_app;

-- +goose Down
ALTER TABLE grants DROP CONSTRAINT grants_target_host_id_fkey;
DROP TABLE terminal_sessions;
DROP TABLE host_session_users;
DROP TABLE hosts;
