-- SPDX-License-Identifier: Apache-2.0
-- Deletion journal (M7-07): validates the check of event_consumer_cursors that 00080 added
-- NOT VALID (upgrade.md rule M5). VALIDATE CONSTRAINT takes only SHARE UPDATE EXCLUSIVE, so the
-- worker consumers keep moving their cursors meanwhile.

-- +goose Up
ALTER TABLE event_consumer_cursors VALIDATE CONSTRAINT event_consumer_cursors_name_check;

-- +goose Down
-- A validated constraint cannot become NOT VALID again; the down step of 00080 replaces it.
SELECT 1;
