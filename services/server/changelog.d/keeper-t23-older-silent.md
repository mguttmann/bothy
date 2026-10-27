### Fixed

- The terminal scenario TM-T23 `older_silent` checks the stable end state after the paused host
  is resumed: it reconnects while the newer connection still answers, so the host ends `disabled`
  with `duplicate_connection` and both connections close with `4112`. The old check for `active`
  held only until that reconnect (flaky `TestScenarios/tm-t23/older_silent`).
