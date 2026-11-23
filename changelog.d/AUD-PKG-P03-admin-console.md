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
