-- SPDX-License-Identifier: Apache-2.0
--
-- The texts of runs in the job retention.content (docs/spec/20-privacy.md 4.1,
-- AUD-PKG-R2-owner-server): a run, its steps and its wake-ups keep their rows (they go with their
-- thread, 03 principle 7) and lose their texts once the end time lies before the cutoff. Each
-- statement empties at most row_limit rows that still hold a text, so a repeated run finds only
-- the rest.

-- name: ClearExpiredRunTexts :execrows
-- Status line, text excerpt, summary and plan of runs that finished before the cutoff.
UPDATE runs SET status_line = NULL, last_text_excerpt = NULL, summary = NULL, plan = '{}'::jsonb
WHERE id IN (
    SELECT r.id FROM runs r
    WHERE r.finished_at < @cutoff
      AND (r.status_line IS NOT NULL OR r.last_text_excerpt IS NOT NULL OR r.summary IS NOT NULL
           OR r.plan <> '{}'::jsonb)
    ORDER BY r.finished_at, r.id
    LIMIT @row_limit
);

-- name: ClearExpiredRunStepTexts :execrows
-- Title and detail of the steps of runs that finished before the cutoff.
UPDATE run_steps SET title = '', detail = '{}'::jsonb
WHERE id IN (
    SELECT s.id FROM run_steps s
    JOIN runs r ON r.id = s.run_id
    WHERE r.finished_at < @cutoff AND (s.title <> '' OR s.detail <> '{}'::jsonb)
    LIMIT @row_limit
);

-- name: ClearExpiredRunWakeTexts :execrows
-- Label, condition and result of wake-ups that ended before the cutoff.
UPDATE run_wakes SET label = '', condition = '{}'::jsonb, result_text = ''
WHERE id IN (
    SELECT w.id FROM run_wakes w
    WHERE w.ended_at < @cutoff AND (w.label <> '' OR w.condition <> '{}'::jsonb OR w.result_text <> '')
    ORDER BY w.ended_at, w.id
    LIMIT @row_limit
);
