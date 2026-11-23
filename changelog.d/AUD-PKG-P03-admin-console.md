### Changed

- The web console's browser tab names the page, and choosing a page with the keyboard keeps the
  focus on its navigation entry.
- In a narrow browser window or at a high zoom the console's navigation folds into a drawer behind
  a menu button.
- A run or chat on the console's "Live" page opens a dialog with the thread ID and a copy action.
- A release build takes `crash_reports.target` of `distribution.yaml` as the crash report target
  of the apps (`CRASH_DSN`); the apps then offer crash reports, still off until the person turns
  them on.
- The console's browser adapter tests run in CI (`make console-web-test`).

### Fixed

- The page "Live" queries the period up to now after a tab change and after TV mode, and TV
  mode moves the period with the clock at every rotation, so a screen in the office no longer
  shows the day it started on.
- A person who may read the statistics of a space but not of the instance starts the page
  "Live" in the first space instead of a refusal.
- The administration counts the waiting devices one pass at a time and only after a change on
  People or Devices, which keeps large instances within the read rate limit.
- A cancelled Touch ID, Face ID or PIN confirmation of a step-up no longer reads as "No
  connection".
- People and group lists drop the answer of an older search or filter, "Load more" asks once
  while its page loads, and the group filter offers the groups of every page.
- "Extend" keeps the person or group and the target of the grant it replaces.
- The status page in the administration leaves its loading state on every error of a read.
- An import source shows a failed refresh with a retry above its last state.
- People, groups and waiting devices show that they load.
- In the console's terminal, Ctrl+Shift+F6 (Cmd+Shift+F6 on Apple systems) moves the focus from
  the terminal to the buttons above it, and the view names these keys; before, the keyboard could
  not leave the terminal.
- A failed logout and a step-up return that did not complete now show a message in the signed-in
  console instead of nothing.
- When the console cannot read its session after a step-up or after starting "View as person", it
  says so instead of staying on the old screen.
- The confirmation of an action in the console uses a challenge created after the person answered
  the dialog, so a slow answer no longer fails a correct password.
- When the server ends a stored "View as person" before the browser's clock does, the console
  reads its session instead of asking for a step-up.
- The console reads the profile and instance view again after a failed read, ignores a late
  answer for a previous person, and reads its session when the tab becomes visible again.
- Opening a terminal on a path whose host ID is invalid no longer leaves an empty terminal view
  and an unhandled error.
- Ending a console session no longer waits forever when another tab holds the terminal pins
  database open, and a pin write the browser aborts reports an error.
