### Fixed

- The terminal tests that let an opening time out (`TestAuditTerminate_Details`,
  `TestEventsValidate`, `TestCloseValuesOnlyFromLists` in `test/terminal`) no longer hang until
  the test patience ends: the Fake-Host opened its data connection right after `session.open`, and
  when the relay accepted it while the open deadline ran, that lone side got its own timer from
  the advanced fake clock, so the opening waited for a second `terminal.open_timeout`. The one
  helper `openTimedOut` now opens such a session with a host that never dials, so the deadline of
  the grant alone decides inside the clock advance, and it checks the end right after the advance.
