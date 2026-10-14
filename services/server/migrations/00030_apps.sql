-- SPDX-License-Identifier: Apache-2.0
-- Apps of the apps module (M2-24): apps exactly as docs/spec/03-datenmodell.md section 4.4 states
-- it. An app is a principal of kind app (ADR 0022); this table holds its manifest
-- (app_manifest.v1 as jsonb), the SemVer of the manifest and the SHA-256 of the manifest bytes as
-- received. Retention class: account (03 section 3).
--
-- Grants (migrations/README.md, grant rule): bothy_app may read, insert and update, not delete.
-- Rows are never deleted (03 Grundsatz 7): an uninstalled app keeps its row and its principal,
-- because its messages keep their author (docs/spec/11-erweiterungen.md section 4.3).

-- +goose Up
CREATE TABLE apps (
    principal_id     uuid        NOT NULL,
    slug             text        NOT NULL,
    manifest         jsonb       NOT NULL,
    manifest_version text        NOT NULL,
    manifest_sha256  bytea       NOT NULL,
    publisher        text        NOT NULL DEFAULT '',
    homepage_url     text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (principal_id),
    FOREIGN KEY (principal_id) REFERENCES principals (id) ON DELETE RESTRICT
);
-- The slug is unique in the instance without regard to case (11 section 3.1).
CREATE UNIQUE INDEX apps_slug_lower_key ON apps (lower(slug));

-- An apps row belongs to a principal of kind app (03 apps: "kind = 'app' per Trigger geprüft"),
-- the same pattern as identity_users_kind_check. principals.kind never changes, so a check at
-- insert and at a change of principal_id suffices. SQLSTATE 23514 (check_violation) lets the
-- adapter map it like a check.
-- +goose StatementBegin
CREATE FUNCTION apps_principal_kind_check() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog, pg_temp
AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.principals WHERE id = NEW.principal_id AND kind = 'app') THEN
        RAISE EXCEPTION 'apps row for a principal that is not of kind app'
            USING ERRCODE = 'check_violation', TABLE = 'apps', COLUMN = 'principal_id';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER apps_principal_kind_check
    BEFORE INSERT OR UPDATE OF principal_id ON apps
    FOR EACH ROW EXECUTE FUNCTION apps_principal_kind_check();

-- Trigger functions fire without EXECUTE; nobody needs to call it directly.
REVOKE ALL ON FUNCTION apps_principal_kind_check() FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON apps TO bothy_app;

-- +goose Down
DROP TABLE apps;
DROP FUNCTION apps_principal_kind_check();
