-- SPDX-License-Identifier: Apache-2.0
-- Grant model tables of the authz module (M1-27): groups, group_memberships and grants exactly
-- as docs/spec/03-datenmodell.md section 4.3 states them. IDs are UUIDv7 from kernel/ids, so no
-- id has a database default (R-DM-01); enumerations are text with CHECK (R-DM-02).
--
-- Deviations: foreign keys to tables that do not exist yet. The migration of the ticket that
-- creates each table adds its key:
--   M1-36: ALTER TABLE groups ADD CONSTRAINT groups_provider_id_fkey
--            FOREIGN KEY (provider_id) REFERENCES identity_providers(id) ON DELETE RESTRICT;
--   M1-52: ALTER TABLE grants ADD CONSTRAINT grants_target_space_id_fkey
--            FOREIGN KEY (target_space_id) REFERENCES spaces(id) ON DELETE RESTRICT;
--   M1-53: ALTER TABLE grants ADD CONSTRAINT grants_target_channel_id_fkey
--            FOREIGN KEY (target_channel_id) REFERENCES channels(id) ON DELETE RESTRICT;
--   M2-39: ALTER TABLE grants ADD CONSTRAINT grants_target_host_id_fkey
--            FOREIGN KEY (target_host_id) REFERENCES hosts(id) ON DELETE RESTRICT;
-- Until then no code path creates space, channel or host grants, because no such target exists.
--
-- grants.subject_kind is not checked against principals.kind: that would need a trigger reading
-- an identity table. The use case validates it through PrincipalSource (M1-48), and the rights
-- snapshot caps level and capabilities by the principal's real kind (M1-28), so a wrong
-- subject_kind can never widen rights.
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update the three
-- tables but not delete from them. Groups are soft deleted, memberships are removed by
-- removed_at and grants are revoked by revoked_at; the rows are history (R-DM-09). A retention
-- job that must delete rows (M7) grants DELETE in its own migration; the immutability trigger
-- does not block deletion.

-- +goose Up
CREATE TABLE groups (
    id           uuid        NOT NULL,
    name         text        NOT NULL,
    description  text        NOT NULL DEFAULT '',
    source       text        NOT NULL DEFAULT 'local'
                             CONSTRAINT groups_source_check CHECK (source IN ('local', 'oidc_claim')),
    provider_id  uuid,
    external_ref text,
    created_by   uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    deleted_at   timestamptz,
    PRIMARY KEY (id),
    CONSTRAINT groups_created_by_fkey FOREIGN KEY (created_by) REFERENCES principals (id) ON DELETE RESTRICT,
    -- A claim group is bound to exactly one provider and one claim value (03 groups, R-DM-33).
    CONSTRAINT groups_provider_shape_check CHECK ((source = 'oidc_claim') = (provider_id IS NOT NULL)),
    CONSTRAINT groups_external_ref_shape_check CHECK ((source = 'oidc_claim') = (external_ref IS NOT NULL))
);
-- Names are unique among live groups without regard to case; a deleted group frees its name.
CREATE UNIQUE INDEX groups_live_name_key ON groups (lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX groups_claim_key ON groups (provider_id, external_ref)
    WHERE source = 'oidc_claim' AND deleted_at IS NULL;

CREATE TABLE group_memberships (
    id                 uuid        NOT NULL,
    group_id           uuid        NOT NULL,
    -- Only people: the key points at users, so an agent or app has no row to point at (R-DM-07).
    user_id            uuid        NOT NULL,
    added_by           uuid        NOT NULL,
    added_at           timestamptz NOT NULL DEFAULT now(),
    expires_at         timestamptz,
    expiry_reminded_at timestamptz,
    expiry_reported_at timestamptz,
    removed_at         timestamptz,
    removed_by         uuid,
    PRIMARY KEY (id),
    CONSTRAINT group_memberships_group_id_fkey FOREIGN KEY (group_id) REFERENCES groups (id) ON DELETE RESTRICT,
    CONSTRAINT group_memberships_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    CONSTRAINT group_memberships_added_by_fkey FOREIGN KEY (added_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT group_memberships_removed_by_fkey FOREIGN KEY (removed_by) REFERENCES principals (id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX group_memberships_active_key ON group_memberships (group_id, user_id) WHERE removed_at IS NULL;
-- "Groups of a person" when the rights snapshot is built (07 section 7.2).
CREATE INDEX group_memberships_active_user_idx ON group_memberships (user_id) WHERE removed_at IS NULL;
-- Reminder and expiry report (M1-30).
CREATE INDEX group_memberships_expiry_idx ON group_memberships (expires_at)
    WHERE removed_at IS NULL AND expires_at IS NOT NULL;

CREATE TABLE grants (
    id                   uuid        NOT NULL,
    subject_kind         text        NOT NULL,
    subject_principal_id uuid,
    subject_group_id     uuid,
    target_kind          text        NOT NULL,
    target_space_id      uuid,
    target_channel_id    uuid,
    target_host_id       uuid,
    level                text,
    capabilities         text[]      NOT NULL DEFAULT '{}',
    expires_at           timestamptz,
    expiry_reminded_at   timestamptz,
    expiry_reported_at   timestamptz,
    reason               text        NOT NULL,
    granted_by           uuid        NOT NULL,
    granted_at           timestamptz NOT NULL DEFAULT now(),
    revoked_at           timestamptz,
    revoked_by           uuid,
    revoke_reason        text,
    PRIMARY KEY (id),
    CONSTRAINT grants_subject_principal_id_fkey FOREIGN KEY (subject_principal_id) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT grants_subject_group_id_fkey FOREIGN KEY (subject_group_id) REFERENCES groups (id) ON DELETE RESTRICT,
    CONSTRAINT grants_granted_by_fkey FOREIGN KEY (granted_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT grants_revoked_by_fkey FOREIGN KEY (revoked_by) REFERENCES principals (id) ON DELETE RESTRICT,
    CONSTRAINT grants_subject_kind_check CHECK (subject_kind IN ('user', 'group', 'agent', 'app')),
    CONSTRAINT grants_target_kind_check CHECK (target_kind IN ('instance', 'space', 'channel', 'host')),
    -- Levels do not apply on a host (07 section 2.12): NULL exactly there.
    CONSTRAINT grants_level_check CHECK (level IN ('read', 'write', 'trigger') AND (target_kind = 'host') = (level IS NULL)),
    CONSTRAINT grants_capabilities_check CHECK (capabilities <@ ARRAY['approve_actions', 'upload', 'export', 'manage', 'terminal']::text[]),
    CONSTRAINT grants_subject_shape_check CHECK (
        (subject_kind = 'group') = (subject_group_id IS NOT NULL AND subject_principal_id IS NULL)
        AND (subject_kind <> 'group') = (subject_principal_id IS NOT NULL AND subject_group_id IS NULL)),
    CONSTRAINT grants_target_shape_check CHECK (
        (target_space_id IS NOT NULL) = (target_kind = 'space')
        AND (target_channel_id IS NOT NULL) = (target_kind = 'channel')
        AND (target_host_id IS NOT NULL) = (target_kind = 'host')),
    -- Agents and apps: at most write, only upload (R-DM-08, R-DM-20).
    CONSTRAINT grants_bot_limits_check CHECK (
        subject_kind NOT IN ('agent', 'app') OR (level <> 'trigger' AND capabilities <@ ARRAY['upload']::text[])),
    -- A host grant carries exactly terminal, for people and groups only (R-DM-35).
    CONSTRAINT grants_host_shape_check CHECK (
        target_kind <> 'host' OR (capabilities = ARRAY['terminal'] AND subject_kind IN ('user', 'group'))),
    CONSTRAINT grants_terminal_only_on_host_check CHECK (target_kind = 'host' OR NOT ('terminal' = ANY (capabilities)))
);
-- No unique constraint: several grants on the same pair are allowed; the highest valid one wins.
-- Rights snapshot of a principal (07 section 7.2).
CREATE INDEX grants_active_subject_principal_idx ON grants (subject_principal_id) WHERE revoked_at IS NULL;
CREATE INDEX grants_active_subject_group_idx ON grants (subject_group_id) WHERE revoked_at IS NULL;
-- "Who has access?" and the I1 check.
CREATE INDEX grants_active_target_space_idx ON grants (target_space_id) WHERE revoked_at IS NULL;
CREATE INDEX grants_active_target_channel_idx ON grants (target_channel_id) WHERE revoked_at IS NULL;
-- I6 check and SR10.
CREATE INDEX grants_active_target_host_idx ON grants (target_host_id) WHERE revoked_at IS NULL;
-- Reminder and expiry report (M1-30).
CREATE INDEX grants_expiry_idx ON grants (expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL;

-- Grants never change (R-DM-09): allowed are only (a) the revocation columns revoked_at,
-- revoked_by and revoke_reason while the grant is not revoked, with revoked_at set, and (b)
-- expiry_reminded_at and expiry_reported_at from NULL to a value. Every other change, including
-- a second revocation and any change of level, capabilities, subject, target, expiry, reason or
-- grantor, raises. The comparison runs over the whole row minus the allowed columns, so a column
-- a later migration adds is immutable unless that migration says otherwise. The database is the
-- last line against a code path that updates in place. SQLSTATE 23514 (check_violation) with
-- the constraint name lets the adapter map it like a check.
-- +goose StatementBegin
CREATE FUNCTION authz_grants_immutable() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
DECLARE
    bookkeeping constant text[] := ARRAY['revoked_at', 'revoked_by', 'revoke_reason',
                                         'expiry_reminded_at', 'expiry_reported_at'];
BEGIN
    IF (to_jsonb(NEW) - bookkeeping) IS DISTINCT FROM (to_jsonb(OLD) - bookkeeping) THEN
        RAISE EXCEPTION 'a grant never changes; revoke it and create a new one'
            USING ERRCODE = 'check_violation', TABLE = 'grants', CONSTRAINT = 'authz_grants_immutable';
    END IF;
    IF (NEW.revoked_at, NEW.revoked_by, NEW.revoke_reason) IS DISTINCT FROM (OLD.revoked_at, OLD.revoked_by, OLD.revoke_reason)
       AND (OLD.revoked_at IS NOT NULL OR NEW.revoked_at IS NULL) THEN
        RAISE EXCEPTION 'a grant is revoked exactly once'
            USING ERRCODE = 'check_violation', TABLE = 'grants', CONSTRAINT = 'authz_grants_immutable';
    END IF;
    IF NEW.expiry_reminded_at IS DISTINCT FROM OLD.expiry_reminded_at
       AND (OLD.expiry_reminded_at IS NOT NULL OR NEW.expiry_reminded_at IS NULL) THEN
        RAISE EXCEPTION 'the expiry reminder of a grant is written exactly once'
            USING ERRCODE = 'check_violation', TABLE = 'grants', CONSTRAINT = 'authz_grants_immutable';
    END IF;
    IF NEW.expiry_reported_at IS DISTINCT FROM OLD.expiry_reported_at
       AND (OLD.expiry_reported_at IS NOT NULL OR NEW.expiry_reported_at IS NULL) THEN
        RAISE EXCEPTION 'the expiry report of a grant is written exactly once'
            USING ERRCODE = 'check_violation', TABLE = 'grants', CONSTRAINT = 'authz_grants_immutable';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER authz_grants_immutable
    BEFORE UPDATE ON grants
    FOR EACH ROW EXECUTE FUNCTION authz_grants_immutable();

-- Trigger functions fire without EXECUTE; nobody needs to call them directly.
REVOKE ALL ON FUNCTION authz_grants_immutable() FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON groups, group_memberships, grants TO bothy_app;

-- +goose Down
DROP TABLE grants;
DROP TABLE group_memberships;
DROP TABLE groups;
DROP FUNCTION authz_grants_immutable();
