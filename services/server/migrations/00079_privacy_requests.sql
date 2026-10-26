-- SPDX-License-Identifier: Apache-2.0
-- Privacy requests (M7-06): gdpr_requests exactly as docs/spec/03-data-model.md states it, and the
-- rights the participants of the account deletion need (docs/spec/20-privacy.md section 4.4).
--
-- gdpr_requests is new and empty, so its keys, checks and indexes are created with it. Only the
-- module privacy reads and writes it (03 principle 12); bothy_app gets DELETE for the job
-- retention.privacy_job, which removes requests of the class privacy_job (20 section 4.1).
--
-- The account deletion deletes the rows of a person that 20 section 4.4 lists in steps 2 and 3:
-- linked identities and the local credential (identity), the read state (sync) and the personal
-- templates (messaging). Every other table of these steps already had DELETE. Each participant
-- deletes only the tables of its own module (03 principle 12).

-- +goose Up
CREATE TABLE gdpr_requests (
    id                uuid        NOT NULL,
    subject_user_id   uuid        NOT NULL,
    kind              text        NOT NULL CHECK (kind IN ('export', 'deletion', 'rectification')),
    state             text        NOT NULL DEFAULT 'requested'
                                  CHECK (state IN ('requested', 'approved', 'running', 'completed', 'failed', 'rejected', 'cancelled')),
    requested_by      uuid        NOT NULL,
    approved_by       uuid,
    reason            text        NOT NULL,
    due_at            timestamptz NOT NULL,
    result_file_id    uuid,
    result_expires_at timestamptz,
    progress          jsonb       NOT NULL DEFAULT '{}',
    error             text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    completed_at      timestamptz,
    PRIMARY KEY (id),
    FOREIGN KEY (subject_user_id) REFERENCES users (principal_id) ON DELETE RESTRICT,
    FOREIGN KEY (requested_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (approved_by) REFERENCES principals (id) ON DELETE RESTRICT,
    FOREIGN KEY (result_file_id) REFERENCES files (id) ON DELETE SET NULL
);
-- One open request per person and kind (03 gdpr_requests); a second one answers 409.
CREATE UNIQUE INDEX gdpr_requests_open_key ON gdpr_requests (subject_user_id, kind)
    WHERE state IN ('requested', 'approved', 'running');
-- The job worker, the reminder and the deadline warnings of the console.
CREATE INDEX gdpr_requests_state_due_at_idx ON gdpr_requests (state, due_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON gdpr_requests TO bothy_app;
GRANT DELETE ON user_identities, local_credentials, read_states, message_templates TO bothy_app;

-- +goose Down
REVOKE DELETE ON user_identities, local_credentials, read_states, message_templates FROM bothy_app;
DROP TABLE gdpr_requests;
