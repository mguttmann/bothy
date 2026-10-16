// SPDX-License-Identifier: Apache-2.0

/// Applier of `run.state_changed` (04 11.3 item 1): the run summary by version feeds the sidebar
/// and "Running now". The `trigger_state` of the triggering message comes with its own
/// `message.updated` (brief M4-14 Design note 2d); `run.updated` is ephemeral and goes to the
/// run store hook.
library;

import '../dispatcher.dart';
import '../wire.dart';

/// The appliers of this group by event type.
final Map<String, Applier> runSummaryAppliers = {
  'run.state_changed': (c) =>
      c.db.runsDao.upsertIfNewer(runFromJson(obj(c.payload, 'run'))),
};
