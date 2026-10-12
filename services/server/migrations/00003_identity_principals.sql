-- SPDX-License-Identifier: Apache-2.0
-- Principal tables of the identity module (M1-26): principals, users and owners exactly as
-- docs/spec/03-data-model.md section 4.2 states them. IDs are UUIDv7 from kernel/ids, so no
-- id has a database default (R-DM-01); enumerations are text with CHECK (R-DM-02).
--
-- Deviation: principals.avatar_file_id has no foreign key yet, because files does not exist
-- before M3-03. The migration of M3-03 that creates files adds
--   ALTER TABLE principals ADD CONSTRAINT principals_avatar_file_id_fkey
--     FOREIGN KEY (avatar_file_id) REFERENCES files(id) ON DELETE SET NULL;
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update the three
-- tables but not delete from them. Principals are never hard deleted (03 Grundsatz 7), an owner
-- is revoked by revoked_at, and a users row lives as long as its principal; a later ticket that
-- needs a delete grants it in its own migration.

-- +goose Up
CREATE TABLE principals (
    id             uuid        NOT NULL,
    kind           text        NOT NULL CHECK (kind IN ('user', 'agent', 'app')),
    status         text        NOT NULL DEFAULT 'active'
                               CHECK (status IN ('pending', 'active', 'disabled', 'deleted')),
    handle         text        NOT NULL CHECK (handle ~ '^[a-z0-9][a-z0-9._-]*$'),
    display_name   text        NOT NULL,
    avatar_file_id uuid,
    created_at     timestamptz NOT NULL DEFAULT now(),
    disabled_at    timestamptz,
    deleted_at     timestamptz,
    PRIMARY KEY (id)
);
-- Handles are unique without regard to case (03 Grundsatz 6); the check already allows only
-- lower case, the index keeps the rule if the check ever widens.
CREATE UNIQUE INDEX principals_handle_lower_key ON principals (lower(handle));
-- Directory lists filter by kind and status.
CREATE INDEX principals_kind_status_idx ON principals (kind, status);

CREATE TABLE users (
    principal_id   uuid        NOT NULL,
    email          text,
    locale         text        NOT NULL DEFAULT 'de',
    time_zone      text        NOT NULL DEFAULT 'UTC',
    last_active_at timestamptz,
    PRIMARY KEY (principal_id),
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE owners (
    id         uuid        NOT NULL,
    user_id    uuid        NOT NULL,
    granted_by uuid,
    granted_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    revoked_by uuid,
    reason     text        NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (granted_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (revoked_by) REFERENCES principals (id) ON DELETE RESTRICT
);
-- At most one active owners row per person.
CREATE UNIQUE INDEX owners_active_user_key ON owners (user_id) WHERE revoked_at IS NULL;

-- A users row belongs to a principal of kind user (03 users: checked by a trigger). The kind
-- never changes (next trigger), so a check at insert and at a change of principal_id suffices.
-- SQLSTATE 23514 (check_violation) lets the adapter map it like a check.
-- +goose StatementBegin
CREATE FUNCTION identity_users_kind_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.principals WHERE id = NEW.principal_id AND kind = 'user') THEN
        RAISE EXCEPTION 'users row for a principal that is not of kind user'
            USING ERRCODE = 'check_violation', TABLE = 'users', COLUMN = 'principal_id';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER identity_users_kind_check
    BEFORE INSERT OR UPDATE OF principal_id ON users
    FOR EACH ROW EXECUTE FUNCTION identity_users_kind_check();

-- principals.kind never changes (03 principals: never changeable).
-- +goose StatementBegin
CREATE FUNCTION identity_principals_kind_immutable() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    IF NEW.kind IS DISTINCT FROM OLD.kind THEN
        RAISE EXCEPTION 'the kind of a principal never changes'
            USING ERRCODE = 'check_violation', TABLE = 'principals', COLUMN = 'kind';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER identity_principals_kind_immutable
    BEFORE UPDATE OF kind ON principals
    FOR EACH ROW EXECUTE FUNCTION identity_principals_kind_immutable();

-- Trigger functions fire without EXECUTE; nobody needs to call them directly.
REVOKE ALL ON FUNCTION identity_users_kind_check() FROM PUBLIC;
REVOKE ALL ON FUNCTION identity_principals_kind_immutable() FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON principals, users, owners TO bothy_app;

-- +goose Down
DROP TABLE owners;
DROP TABLE users;
DROP TABLE principals;
DROP FUNCTION identity_users_kind_check();
DROP FUNCTION identity_principals_kind_immutable();
