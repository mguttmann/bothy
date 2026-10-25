-- SPDX-License-Identifier: Apache-2.0
-- Retention per class (M7-05): the rights the jobs retention.<class> need, no table definition.
--
-- bothy_app may delete the expired rows of the classes security and cost (docs/spec/20-privacy.md
-- section 4.1) and the closed device pairings of the job identity.auth_cleanup; every other table
-- of these jobs already had DELETE (push_registrations, webhook_deliveries) or is only emptied or
-- tombstoned (messages, audit_ip_keys). The participant of each module deletes only its own
-- tables (03 principle 12).
--
-- bothy_maint is the role of the audit trim (03 audit_events, 16 7.1): in one transaction it
-- writes the anchor entry through the audit hook (reserves audit_seq, reads the hash of the head,
-- inserts the row) and deletes the trimmed rows of audit_events and the checkpoints before them.
-- The triggers of 00004 and 00015 already let a DELETE by bothy_maint through and refuse every
-- other change by every role; bothy_maint gets no UPDATE on either table and no right on any other
-- table except the counter row it advances.

-- +goose Up
GRANT DELETE ON server_signing_keys, passkey_credentials, devices, device_keys, auth_sessions,
    refresh_token_families, refresh_tokens, api_tokens, enrollment_codes TO bothy_app;
GRANT DELETE ON hosts, host_session_users, terminal_sessions TO bothy_app;
-- identity.auth_cleanup deletes the closed pairings past expires_at (02 12.3, class transient).
GRANT DELETE ON device_pairings TO bothy_app;
GRANT DELETE ON price_table, cost_entries TO bothy_app;

GRANT SELECT, INSERT, DELETE ON audit_events TO bothy_maint;
GRANT SELECT, DELETE ON audit_checkpoints TO bothy_maint;
GRANT SELECT ON sequence_counters TO bothy_maint;
GRANT UPDATE (value) ON sequence_counters TO bothy_maint;

-- +goose Down
REVOKE UPDATE (value) ON sequence_counters FROM bothy_maint;
REVOKE SELECT ON sequence_counters FROM bothy_maint;
REVOKE SELECT, DELETE ON audit_checkpoints FROM bothy_maint;
REVOKE SELECT, INSERT, DELETE ON audit_events FROM bothy_maint;
REVOKE DELETE ON price_table, cost_entries FROM bothy_app;
REVOKE DELETE ON device_pairings FROM bothy_app;
REVOKE DELETE ON hosts, host_session_users, terminal_sessions FROM bothy_app;
REVOKE DELETE ON server_signing_keys, passkey_credentials, devices, device_keys, auth_sessions,
    refresh_token_families, refresh_tokens, api_tokens, enrollment_codes FROM bothy_app;
