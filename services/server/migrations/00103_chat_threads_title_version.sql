-- SPDX-License-Identifier: Apache-2.0
-- The version of the last title change of a thread (FIX-TITLE-EDIT-412, docs/spec/03-data-model.md
-- section 4.5 `threads`, docs/spec/06-api.md section 8 point 2): title_version is the value of
-- version at which title or title_source last changed. setThreadTitle accepts an If-Match from
-- that version on, because every message raises version without a thread event (03 section 6.2),
-- so a client in an active chat never holds the current one.
--
-- Expand only (upgrade.md rules M1 and M3): a column with a constant default is added without a
-- table rewrite, and the previous binary neither reads nor writes it. Every existing thread gets 1,
-- the version of a new thread: its precondition accepts every version it ever had until its next
-- title change (docs/decisions.d/FIX-TITLE-EDIT-412.md).

-- +goose Up
ALTER TABLE threads ADD COLUMN title_version bigint NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE threads DROP COLUMN IF EXISTS title_version;
