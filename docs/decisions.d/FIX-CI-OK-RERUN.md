# FIX-CI-OK-RERUN ci-ok of a rerun of failed jobs creates its own check run

| No | Date | Ruling | Reason |
|---|---|---|---|
| FCOR/1 | 2026-11-24 | `ci-ok` completes the pending check run of the job `plan` only in the attempt that created it (`plan` output `marker_attempt`); `ci-ok` of a later attempt leaves it alone and creates a new, completed check run `ci-ok` for the head commit with its own conclusion (supersedes the last sentence of PL/15 for later attempts). | "Re-run failed jobs" does not run `plan` again. In run 45380914447 (PR #1781) `ci-ok` of attempts 2 and 3 updated the mark of attempt 1, and GitHub left its previous conclusion behind as a newer check run `ci-ok` (138479737312, 138505634360), so the required check stayed red under a green job `ci-ok`. Tests: `test_ci_ok_of_the_attempt_that_marked_completes_the_mark`, `test_ci_ok_of_a_rerun_of_failed_jobs_creates_its_own_check_run`. |
