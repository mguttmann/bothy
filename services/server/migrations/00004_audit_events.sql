-- SPDX-License-Identifier: Apache-2.0
-- Module audit (M1-24): the append-only, hash-chained audit log audit_events and the keys of the
-- IP pseudonyms audit_ip_keys, exactly as docs/spec/03-data-model.md section 4.12 states them
-- (ADR 0021, retention class audit). audit_checkpoints comes with M1-50.
--
-- audit_events: seq comes from sequence_counters('audit_seq') in the same transaction, so the
-- chain is gapless and ordered by commit; there is no foreign key, because the log outlives
-- everything it names. deny_reason has no CHECK on purpose (R-DM-28): an entry must never fail on
-- a new reason code; a test holds the list closed against 07.
--
-- Rights (19 SEC-AUD-1, migrations/README.md grant rule): bothy_app may only read and insert.
-- UPDATE, DELETE and TRUNCATE are revoked and, for every role including the table owner, refused
-- by triggers; the only exception is a DELETE by bothy_maint, the retention trim of M7 that also
-- writes an anchor (03). bothy_maint gets its grants with that ticket, none here.
--
-- audit_ip_keys: one HMAC key per privacy.audit_ip_key_period, AEAD-encrypted under the key of
-- privacy.audit_ip_kek_ref (20 "Audit log and IP addresses"). bothy_app reads and inserts rows and
-- may update only key_ciphertext and destroyed_at (the destruction job of M7-05).

-- +goose Up
CREATE TABLE audit_events (
    seq                bigint      NOT NULL,
    id                 uuid        NOT NULL,
    occurred_at        timestamptz NOT NULL DEFAULT now(),
    actor_principal_id uuid,
    actor_device_id    uuid,
    actor_session_id   uuid,
    action             text        NOT NULL,
    decision           text        NOT NULL CHECK (decision IN ('allow', 'deny', 'error')),
    deny_reason        text,
    resource_kind      text,
    resource_id        uuid,
    details            jsonb       NOT NULL DEFAULT '{}',
    correlation_id     uuid,
    prev_hash          bytea       NOT NULL,
    hash               bytea       NOT NULL,
    PRIMARY KEY (seq),
    CONSTRAINT audit_events_id_key UNIQUE (id),
    CONSTRAINT audit_events_hash_key UNIQUE (hash)
);
-- The filterable log view (03): by actor, by resource, by action, newest first.
CREATE INDEX audit_events_actor_idx ON audit_events (actor_principal_id, occurred_at DESC);
CREATE INDEX audit_events_resource_idx ON audit_events (resource_kind, resource_id);
CREATE INDEX audit_events_action_idx ON audit_events (action, occurred_at DESC);
-- Retention trims by time.
CREATE INDEX audit_events_occurred_at_brin ON audit_events USING brin (occurred_at);

-- +goose StatementBegin
CREATE FUNCTION audit_events_append_only() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog
AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_user = 'bothy_maint' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'audit_events is append-only: % refused', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION audit_events_append_only() FROM PUBLIC;

CREATE TRIGGER audit_events_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();

CREATE TRIGGER audit_events_no_truncate
    BEFORE TRUNCATE ON audit_events
    FOR EACH STATEMENT EXECUTE FUNCTION audit_events_append_only();

REVOKE ALL ON audit_events FROM PUBLIC;
GRANT SELECT, INSERT ON audit_events TO bothy_app;
-- No default privileges exist (16 7.1 rule 3); the revoke keeps the rule even if they ever do.
REVOKE UPDATE, DELETE, TRUNCATE ON audit_events FROM bothy_app;

CREATE TABLE audit_ip_keys (
    id             uuid        NOT NULL,
    valid_from     timestamptz NOT NULL,
    valid_until    timestamptz NOT NULL,
    key_ciphertext bytea,
    key_ref        text        NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    destroyed_at   timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT audit_ip_keys_valid_from_key UNIQUE (valid_from),
    CONSTRAINT audit_ip_keys_period_check CHECK (valid_until > valid_from),
    CONSTRAINT audit_ip_keys_destroyed_check CHECK ((key_ciphertext IS NULL) = (destroyed_at IS NOT NULL))
);

REVOKE ALL ON audit_ip_keys FROM PUBLIC;
GRANT SELECT, INSERT ON audit_ip_keys TO bothy_app;
GRANT UPDATE (key_ciphertext, destroyed_at) ON audit_ip_keys TO bothy_app;

-- +goose Down
DROP TABLE audit_ip_keys;
DROP TABLE audit_events;
DROP FUNCTION audit_events_append_only();
