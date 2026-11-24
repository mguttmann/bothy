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
- The confirmation of an action in the console takes a new challenge when the person needed
  more than half of the first one's lifetime, so a slow answer no longer fails a correct
  password.
- When the server ends a stored "View as person" before the browser's clock does, the console
  reads its session instead of asking for a step-up.
- The console reads the profile and instance view again after a failed read, ignores a late
  answer for a previous person, and reads its session when the tab becomes visible again.
- Opening a terminal on a path whose host ID is invalid no longer leaves an empty terminal view
  and an unhandled error.
- Ending a console session no longer waits forever when another tab holds the terminal pins
  database open (it gives up after a second), and a pin write the browser aborts reports an
  error.
- A change on the host or agent page that meets `412 version_conflict` reads the record again
  and says that it changed meanwhile, so the next attempt no longer fails the same way.
- The switch of several agents, the runner change and the pairing wizard stay open while their
  call runs; the pairing wizard keeps its one-time code after a failed read of the host.
- The agent page no longer listens anew to the presence of its agent on every rebuild.
- The built-in agent card shows a failed read after a save with Try again, a refused Disable as
  an error, says nothing after a cancelled step-up, and no longer calls a program error
  "offline".
- "Change consent" on the app page grants or withdraws optional scopes and events after the
  install, also after "Later" in the consent of an update.
- A failed step of the app install offers Close (what succeeded stays) and shows the token's
  fields again for the repeat.
- Installing an app and creating a grant, a group or an agent keep their idempotency key after
  a lost answer, so a repeat no longer creates twice.
- The interview of "New agent" offers Try again after a timeout or an offline builder, takes an
  already delivered draft after Back, and says when the account details did not load.
- The host page offers to read its audit again after a failed read, and the assignee dialog
  keeps the current assignee when the people list lacks it.

### Removed

- The unused `TokenSecretView` of `admin_ui`.
