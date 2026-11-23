### Fixed

- The page "Live" queries the period up to now after a tab change and after TV mode, and TV
  mode moves the period with the clock at every rotation, so a screen in the office no longer
  shows the day it started on.
- A person who may read the statistics of a space but not of the instance starts the page
  "Live" in the first space instead of a refusal.
- The administration counts the waiting devices one pass at a time and only after a change on
  People or Devices, which keeps large instances within the read rate limit.
- A cancelled Touch ID, Face ID or PIN confirmation of a step-up no longer reads as "No
  connection"; an invalidated step-up key says that this device cannot confirm.
- People and group lists drop the answer of an older search or filter, "Load more" asks once
  while its page loads, and the group filter offers the groups of every page.
- "Extend" keeps the person or group and the target of the grant it replaces.
- The status page in the administration leaves its loading state on every error of a read.
- An import source shows a failed refresh with a retry above its last state.
- People, groups and waiting devices show that they load.
