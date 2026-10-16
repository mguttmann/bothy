-- SPDX-License-Identifier: Apache-2.0
-- Module audit (M1-50): the signed checkpoints of the hash chain, audit_checkpoints exactly as
-- docs/spec/03-data-model.md section 4.12 states it (ADR 0021, 19 SEC-AUD-2 and SEC-AUD-7,
-- retention class audit). The job audit.chain_verify inserts a row after it recomputed the chain
-- and sets exported_at once the checkpoint lies at audit.checkpoint_sink; audit.verify never reads
-- this table, only the sink (SEC-AUD-3).
--
-- Rights as audit_events (03): bothy_app may read and insert, and update only exported_at. The
-- trigger refuses, for every role including the table owner, every UPDATE except setting
-- exported_at once with nothing else changed, every DELETE except one by bothy_maint (the
-- retention trim of M7, which brings its grant) and every TRUNCATE.

-- +goose Up
CREATE TABLE audit_checkpoints (
    seq                      bigint      NOT NULL,
    hash                     bytea       NOT NULL,
    previous_checkpoint_hash bytea,
    signature                bytea       NOT NULL,
    kid                      text        NOT NULL,
    created_at               timestamptz NOT NULL DEFAULT now(),
    exported_at              timestamptz,
    PRIMARY KEY (seq),
    CONSTRAINT audit_checkpoints_kid_fkey FOREIGN KEY (kid) REFERENCES server_signing_keys (kid) ON DELETE RESTRICT
);
-- The export reads the checkpoints that are not yet at the sink (03).
CREATE INDEX audit_checkpoints_unexported_idx ON audit_checkpoints (exported_at) WHERE exported_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION audit_checkpoints_append_only() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = pg_catalog
AS $$
BEGIN
    IF TG_OP = 'DELETE' AND current_user = 'bothy_maint' THEN
        RETURN OLD;
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.exported_at IS NULL AND NEW.exported_at IS NOT NULL
       AND NEW.seq = OLD.seq AND NEW.hash = OLD.hash
       AND NEW.previous_checkpoint_hash IS NOT DISTINCT FROM OLD.previous_checkpoint_hash
       AND NEW.signature = OLD.signature AND NEW.kid = OLD.kid AND NEW.created_at = OLD.created_at THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'audit_checkpoints is append-only: % refused', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION audit_checkpoints_append_only() FROM PUBLIC;

CREATE TRIGGER audit_checkpoints_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_checkpoints
    FOR EACH ROW EXECUTE FUNCTION audit_checkpoints_append_only();

CREATE TRIGGER audit_checkpoints_no_truncate
    BEFORE TRUNCATE ON audit_checkpoints
    FOR EACH STATEMENT EXECUTE FUNCTION audit_checkpoints_append_only();

REVOKE ALL ON audit_checkpoints FROM PUBLIC;
GRANT SELECT, INSERT ON audit_checkpoints TO bothy_app;
GRANT UPDATE (exported_at) ON audit_checkpoints TO bothy_app;

-- +goose Down
DROP TABLE audit_checkpoints;
DROP FUNCTION audit_checkpoints_append_only();
