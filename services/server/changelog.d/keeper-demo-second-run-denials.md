### Fixed

- `TestDemoSecondRunRefused` no longer fails with "audit events 61, before 60" on a slow runner
  (main run 40540821549). The first demo run gets step-up answers (`price.manage`,
  `step_up_required`), which the server aggregates and writes only when
  `audit.deny_aggregate_window` (60 s) ends; a run longer than that window counted the late row
  as a write of the second run. The test now checks the audit events after the second run
  started and ignores only aggregated denials whose first denial came before it.
